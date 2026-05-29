package spool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"watchhouse/internal/hostview"
	"watchhouse/internal/strictjson"
	"watchhouse/internal/telemetry"
)

type ListenerItem struct {
	Sequence   int64                 `json:"sequence"`
	SnapshotID string                `json:"snapshot_id"`
	HostID     string                `json:"host_id"`
	Snapshot   hostview.HostSnapshot `json:"snapshot"`
}

type ListenerReceipt struct {
	Sequence   int64  `json:"sequence"`
	SnapshotID string `json:"snapshot_id"`
}

func (s *Store) AppendListenerSnapshot(ctx context.Context, host string, snapshot hostview.HostSnapshot) (AppendResult, error) {
	var result AppendResult
	if !telemetry.ValidHost(host) {
		return result, fmt.Errorf("invalid listener snapshot host")
	}
	if err := snapshot.Validate(); err != nil {
		return result, err
	}
	id := hostview.SnapshotIdentity(host, snapshot)
	payload, err := json.Marshal(snapshot)
	if err != nil || len(payload) > 8*1024*1024 {
		return result, fmt.Errorf("listener snapshot exceeds queue bound")
	}
	digestBytes := sha256.Sum256(payload)
	digest := hex.EncodeToString(digestBytes[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT content_digest FROM listener_snapshots WHERE snapshot_id=?`, id).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, err
	}
	if existing != "" {
		if existing != digest {
			return result, ErrConflict
		}
		result.Duplicate = true
		return result, nil
	}
	var count int
	var size int64
	if err := tx.QueryRowContext(ctx, `SELECT listener_pending_records,listener_pending_bytes FROM queue_state WHERE id=1`).Scan(&count, &size); err != nil {
		return result, err
	}
	if count >= s.options.MaxRecords || int64(len(payload)) > s.options.MaxBytes-size {
		if _, err := tx.ExecContext(ctx, `UPDATE queue_state SET blocked_attempts=blocked_attempts+1 WHERE id=1`); err != nil {
			return result, err
		}
		if err := tx.Commit(); err != nil {
			return result, err
		}
		return result, ErrFull
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO listener_snapshots(snapshot_id,host_id,payload,payload_bytes,content_digest) VALUES(?,?,?,?,?)`, id, host, payload, len(payload), digest); err != nil {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE queue_state SET listener_pending_bytes=listener_pending_bytes+?,listener_pending_records=listener_pending_records+1 WHERE id=1`, len(payload)); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	result.Inserted = true
	return result, nil
}

func (s *Store) PeekListenerSnapshots(ctx context.Context, limit int, maxBytes int64) ([]ListenerItem, error) {
	if limit < 1 || limit > 100 || maxBytes < 1 || maxBytes > 8*1024*1024 {
		return nil, fmt.Errorf("invalid listener batch bound")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sequence,snapshot_id,host_id,payload,content_digest FROM listener_snapshots ORDER BY sequence LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ListenerItem, 0)
	var used int64
	for rows.Next() {
		var item ListenerItem
		var payload []byte
		var digest string
		if err := rows.Scan(&item.Sequence, &item.SnapshotID, &item.HostID, &payload, &digest); err != nil {
			return nil, err
		}
		if int64(len(payload)) > maxBytes-used {
			if len(items) == 0 {
				return nil, ErrBatchTooSmall
			}
			break
		}
		digestBytes := sha256.Sum256(payload)
		if hex.EncodeToString(digestBytes[:]) != digest {
			return nil, ErrCorrupt
		}
		snapshot, err := strictjson.Decode[hostview.HostSnapshot](bytes.NewReader(payload), 8*1024*1024)
		if err != nil || snapshot.Validate() != nil || !telemetry.ValidHost(item.HostID) || hostview.SnapshotIdentity(item.HostID, snapshot) != item.SnapshotID {
			return nil, ErrCorrupt
		}
		item.Snapshot = snapshot
		items = append(items, item)
		used += int64(len(payload))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) AckListenerSnapshots(ctx context.Context, receipts []ListenerReceipt) (int, error) {
	if len(receipts) > 100 {
		return 0, fmt.Errorf("too many listener receipts")
	}
	seen := make(map[int64]string)
	for _, receipt := range receipts {
		if receipt.Sequence < 1 || !identityPattern.MatchString(receipt.SnapshotID) {
			return 0, fmt.Errorf("invalid listener receipt")
		}
		if id, exists := seen[receipt.Sequence]; exists && id != receipt.SnapshotID {
			return 0, ErrAckConflict
		}
		seen[receipt.Sequence] = receipt.SnapshotID
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
		err := tx.QueryRowContext(ctx, `SELECT snapshot_id,payload_bytes FROM listener_snapshots WHERE sequence=?`, sequence).Scan(&stored, &size)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if stored != id {
			return 0, ErrAckConflict
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM listener_snapshots WHERE sequence=? AND snapshot_id=?`, sequence, id); err != nil {
			return 0, err
		}
		deleted++
		bytesRemoved += size
	}
	if _, err := tx.ExecContext(ctx, `UPDATE queue_state SET listener_pending_bytes=listener_pending_bytes-?,listener_pending_records=listener_pending_records-? WHERE id=1`, bytesRemoved, deleted); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deleted, nil
}
