package spool

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"watchhouse/internal/telemetry"
)

var (
	ErrFull            = errors.New("spool capacity reached; source checkpoint not advanced")
	ErrStaleCheckpoint = errors.New("source checkpoint changed; reread before retrying")
	ErrConflict        = errors.New("queued event identity has conflicting content")
)

const SSHSource = "journald.ssh"

type Checkpoint struct {
	HostID         string
	Source         string
	ExpectedCursor string
	NextCursor     string
}

type AppendResult struct {
	Inserted  bool
	Duplicate bool
}

func validCursor(cursor string, optional bool) bool {
	if cursor == "" {
		return optional
	}
	if len(cursor) > 4096 {
		return false
	}
	for _, r := range cursor {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

func (s *Store) Cursor(ctx context.Context, host, source string) (string, error) {
	if !telemetry.ValidHost(host) || source != SSHSource {
		return "", fmt.Errorf("invalid checkpoint scope")
	}
	var cursor string
	err := s.db.QueryRowContext(ctx, "SELECT cursor FROM checkpoints WHERE host_id=? AND source=?", host, source).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return cursor, err
}

// Append atomically stores the normalized event and source checkpoint. A nil
// event advances an explicitly processed, unmatched source record only.
func (s *Store) Append(ctx context.Context, cp Checkpoint, event *telemetry.Event) (AppendResult, error) {
	var result AppendResult
	if !telemetry.ValidHost(cp.HostID) || cp.Source != SSHSource || !validCursor(cp.ExpectedCursor, true) || !validCursor(cp.NextCursor, false) {
		return result, fmt.Errorf("invalid checkpoint")
	}
	var payload []byte
	var digest string
	if event != nil {
		if err := event.Validate(); err != nil {
			return result, err
		}
		if event.HostID != cp.HostID || event.SourceCursor != cp.NextCursor {
			return result, fmt.Errorf("event does not belong to checkpoint")
		}
		var err error
		payload, err = json.Marshal(event)
		if err != nil {
			return result, err
		}
		if len(payload) > telemetry.MaxRecordBytes {
			return result, fmt.Errorf("normalized event exceeds limit")
		}
		canonical := *event
		canonical.ReceivedAt = time.Time{}
		body, err := json.Marshal(canonical)
		if err != nil {
			return result, err
		}
		digest = telemetry.Identity(string(body))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var current string
	err = tx.QueryRowContext(ctx, "SELECT cursor FROM checkpoints WHERE host_id=? AND source=?", cp.HostID, cp.Source).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	var existingDigest string
	if event != nil {
		err = tx.QueryRowContext(ctx, "SELECT content_digest FROM events WHERE event_id=?", event.EventID).Scan(&existingDigest)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return result, err
		}
		if existingDigest != "" && existingDigest != digest {
			return result, ErrConflict
		}
	}
	if current == cp.NextCursor {
		result.Duplicate = true
		return result, nil
	}
	if current != cp.ExpectedCursor {
		return result, ErrStaleCheckpoint
	}
	if event != nil && existingDigest == "" {
		var count int
		var size int64
		if err := tx.QueryRowContext(ctx, "SELECT pending_bytes,pending_records FROM queue_state WHERE id=1").Scan(&size, &count); err != nil {
			return result, err
		}
		if count >= s.options.MaxRecords || int64(len(payload)) > s.options.MaxBytes-size {
			if _, err := tx.ExecContext(ctx, "UPDATE queue_state SET blocked_attempts=blocked_attempts+1 WHERE id=1"); err != nil {
				return result, err
			}
			if err := tx.Commit(); err != nil {
				return result, err
			}
			return result, ErrFull
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO events(event_id,host_id,payload,payload_bytes,content_digest) VALUES(?,?,?,?,?)", event.EventID, event.HostID, payload, len(payload), digest); err != nil {
			return result, err
		}
		if _, err := tx.ExecContext(ctx, "UPDATE queue_state SET pending_bytes=pending_bytes+?,pending_records=pending_records+1 WHERE id=1", len(payload)); err != nil {
			return result, err
		}
		result.Inserted = true
	} else if event != nil {
		result.Duplicate = true
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO checkpoints(host_id,source,cursor) VALUES(?,?,?)
		ON CONFLICT(host_id,source) DO UPDATE SET cursor=excluded.cursor`, cp.HostID, cp.Source, cp.NextCursor); err != nil {
		return AppendResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return AppendResult{}, err
	}
	return result, nil
}
