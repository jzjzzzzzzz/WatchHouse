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

	"watchhouse/internal/extprobe"
	"watchhouse/internal/strictjson"
	"watchhouse/internal/telemetry"
)

type ProbeItem struct {
	Sequence      int64           `json:"sequence"`
	ObservationID string          `json:"observation_id"`
	ProbeID       string          `json:"probe_id"`
	Result        extprobe.Result `json:"result"`
}

type ProbeReceipt struct {
	Sequence      int64  `json:"sequence"`
	ObservationID string `json:"observation_id"`
}

func (s *Store) AppendProbeObservation(ctx context.Context, probe string, result extprobe.Result) (AppendResult, error) {
	var appended AppendResult
	id, err := extprobe.ObservationIdentity(probe, result)
	if err != nil {
		return appended, err
	}
	payload, err := json.Marshal(result)
	if err != nil || len(payload) > 8*1024*1024 {
		return appended, fmt.Errorf("probe observation exceeds queue bound")
	}
	digestBytes := sha256.Sum256(payload)
	digest := hex.EncodeToString(digestBytes[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return appended, err
	}
	defer tx.Rollback()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT content_digest FROM probe_observations WHERE observation_id=?`, id).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return appended, err
	}
	if existing != "" {
		if existing != digest {
			return appended, ErrConflict
		}
		appended.Duplicate = true
		return appended, nil
	}
	var count int
	var size int64
	if err := tx.QueryRowContext(ctx, `SELECT probe_pending_records,probe_pending_bytes FROM queue_state WHERE id=1`).Scan(&count, &size); err != nil {
		return appended, err
	}
	if count >= s.options.MaxRecords || int64(len(payload)) > s.options.MaxBytes-size {
		if _, err := tx.ExecContext(ctx, `UPDATE queue_state SET blocked_attempts=blocked_attempts+1 WHERE id=1`); err != nil {
			return appended, err
		}
		if err := tx.Commit(); err != nil {
			return appended, err
		}
		return appended, ErrFull
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO probe_observations(observation_id,probe_id,payload,payload_bytes,content_digest) VALUES(?,?,?,?,?)`, id, probe, payload, len(payload), digest); err != nil {
		return appended, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE queue_state SET probe_pending_bytes=probe_pending_bytes+?,probe_pending_records=probe_pending_records+1 WHERE id=1`, len(payload)); err != nil {
		return appended, err
	}
	if err := tx.Commit(); err != nil {
		return appended, err
	}
	appended.Inserted = true
	return appended, nil
}

func (s *Store) PeekProbeObservations(ctx context.Context, limit int, maxBytes int64) ([]ProbeItem, error) {
	if limit < 1 || limit > 100 || maxBytes < 1 || maxBytes > 8*1024*1024 {
		return nil, fmt.Errorf("invalid probe batch bound")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sequence,observation_id,probe_id,payload,content_digest FROM probe_observations ORDER BY sequence LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ProbeItem, 0)
	var used int64
	for rows.Next() {
		var item ProbeItem
		var payload []byte
		var digest string
		if err := rows.Scan(&item.Sequence, &item.ObservationID, &item.ProbeID, &payload, &digest); err != nil {
			return nil, err
		}
		if int64(len(payload)) > maxBytes-used {
			if len(items) == 0 {
				return nil, ErrBatchTooSmall
			}
			break
		}
		result, err := validateStoredProbe(payload, item.ObservationID, item.ProbeID, digest)
		if err != nil {
			return nil, err
		}
		item.Result = result
		items = append(items, item)
		used += int64(len(payload))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func validateStoredProbe(payload []byte, id, probe, digest string) (extprobe.Result, error) {
	digestBytes := sha256.Sum256(payload)
	if hex.EncodeToString(digestBytes[:]) != digest || !telemetry.ValidHost(probe) {
		return extprobe.Result{}, ErrCorrupt
	}
	result, err := strictjson.Decode[extprobe.Result](bytes.NewReader(payload), 8*1024*1024)
	expected, identityErr := extprobe.ObservationIdentity(probe, result)
	if err != nil || identityErr != nil || result.Validate() != nil || expected != id {
		return extprobe.Result{}, ErrCorrupt
	}
	return result, nil
}

func (s *Store) AckProbeObservations(ctx context.Context, receipts []ProbeReceipt) (int, error) {
	if len(receipts) > 100 {
		return 0, fmt.Errorf("too many probe receipts")
	}
	seen := make(map[int64]string)
	for _, receipt := range receipts {
		if receipt.Sequence < 1 || !identityPattern.MatchString(receipt.ObservationID) {
			return 0, fmt.Errorf("invalid probe receipt")
		}
		if id, exists := seen[receipt.Sequence]; exists && id != receipt.ObservationID {
			return 0, ErrAckConflict
		}
		seen[receipt.Sequence] = receipt.ObservationID
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
		err := tx.QueryRowContext(ctx, `SELECT observation_id,payload_bytes FROM probe_observations WHERE sequence=?`, sequence).Scan(&stored, &size)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if stored != id {
			return 0, ErrAckConflict
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM probe_observations WHERE sequence=? AND observation_id=?`, sequence, id); err != nil {
			return 0, err
		}
		deleted++
		bytesRemoved += size
	}
	if _, err := tx.ExecContext(ctx, `UPDATE queue_state SET probe_pending_bytes=probe_pending_bytes-?,probe_pending_records=probe_pending_records-? WHERE id=1`, bytesRemoved, deleted); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deleted, nil
}
