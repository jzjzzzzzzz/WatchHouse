package spool

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"watchhouse/internal/telemetry"
)

var (
	ErrBatchTooSmall = errors.New("batch byte budget cannot fit the next event")
	ErrAckConflict   = errors.New("receipt identity does not match queued sequence")
	ErrCorrupt       = errors.New("stored event failed integrity validation")
)

var identityPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Item struct {
	Sequence int64           `json:"sequence"`
	EventID  string          `json:"event_id"`
	Event    telemetry.Event `json:"event"`
}

type Receipt struct {
	Sequence int64  `json:"sequence"`
	EventID  string `json:"event_id"`
}

// Peek never removes events. Only receipts from a verified remote acceptance
// may be passed to Ack by the transport layer; the store cannot authenticate it.
func (s *Store) Peek(ctx context.Context, limit int, maxBytes int64) ([]Item, error) {
	if limit < 1 || limit > 500 || maxBytes < 1 || maxBytes > 8*1024*1024 {
		return nil, fmt.Errorf("invalid batch record or byte limit")
	}
	rows, err := s.db.QueryContext(ctx, "SELECT sequence,event_id,payload,content_digest FROM events ORDER BY sequence LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	var used int64
	for rows.Next() {
		var item Item
		var payload []byte
		var digest string
		if err := rows.Scan(&item.Sequence, &item.EventID, &payload, &digest); err != nil {
			return nil, err
		}
		if int64(len(payload)) > maxBytes-used {
			if len(items) == 0 {
				return nil, ErrBatchTooSmall
			}
			break
		}
		item.Event, err = validateStored(payload, item.EventID, digest)
		if err != nil {
			return nil, err
		}
		used += int64(len(payload))
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func validateStored(payload []byte, id, digest string) (telemetry.Event, error) {
	var event telemetry.Event
	if len(payload) > telemetry.MaxRecordBytes || json.Unmarshal(payload, &event) != nil || event.Validate() != nil || event.EventID != id {
		return telemetry.Event{}, ErrCorrupt
	}
	canonical := event
	canonical.ReceivedAt = time.Time{}
	body, err := json.Marshal(canonical)
	if err != nil || telemetry.Identity(string(body)) != digest {
		return telemetry.Event{}, ErrCorrupt
	}
	return event, nil
}

// Ack deletes exact (sequence,event_id) pairs in one transaction. It never
// deletes "everything up to sequence", which would lose unacknowledged data.
func (s *Store) Ack(ctx context.Context, receipts []Receipt) (int, error) {
	if len(receipts) > 500 {
		return 0, fmt.Errorf("too many receipts")
	}
	seen := make(map[int64]string)
	for _, receipt := range receipts {
		if receipt.Sequence < 1 || !identityPattern.MatchString(receipt.EventID) {
			return 0, fmt.Errorf("invalid receipt")
		}
		if id, exists := seen[receipt.Sequence]; exists && id != receipt.EventID {
			return 0, ErrAckConflict
		}
		seen[receipt.Sequence] = receipt.EventID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	deleted := 0
	var bytesRemoved int64
	for sequence, id := range seen {
		var stored string
		var size int64
		err := tx.QueryRowContext(ctx, "SELECT event_id,payload_bytes FROM events WHERE sequence=?", sequence).Scan(&stored, &size)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if stored != id {
			return 0, ErrAckConflict
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM events WHERE sequence=? AND event_id=?", sequence, id); err != nil {
			return 0, err
		}
		deleted++
		bytesRemoved += size
	}
	if _, err := tx.ExecContext(ctx, "UPDATE queue_state SET pending_bytes=pending_bytes-?,pending_records=pending_records-? WHERE id=1", bytesRemoved, deleted); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deleted, nil
}
