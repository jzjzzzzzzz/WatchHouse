package spool

import (
	"context"
	"database/sql"

	"watchhouse/internal/telemetry"
)

type AuditResult struct {
	Records      int   `json:"records"`
	PayloadBytes int64 `json:"payload_bytes"`
	Checkpoints  int   `json:"checkpoints"`
	Valid        bool  `json:"valid"`
}

// Audit performs a read-only snapshot inspection, not a repair or proof that
// root could not alter both telemetry and its unkeyed content digest.
func (s *Store) Audit(ctx context.Context) (AuditResult, error) {
	var result AuditResult
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var quick string
	if err := tx.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&quick); err != nil {
		return result, err
	}
	if quick != "ok" {
		return result, ErrCorrupt
	}
	var expectedRecords int
	var expectedBytes int64
	if err := tx.QueryRowContext(ctx, "SELECT pending_records,pending_bytes FROM queue_state WHERE id=1").Scan(&expectedRecords, &expectedBytes); err != nil {
		return result, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT event_id,host_id,payload,payload_bytes,content_digest FROM events ORDER BY sequence")
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id, host, digest string
		var payload []byte
		var storedBytes int64
		if err := rows.Scan(&id, &host, &payload, &storedBytes, &digest); err != nil {
			rows.Close()
			return result, err
		}
		event, err := validateStored(payload, id, digest)
		if err != nil || event.HostID != host || storedBytes != int64(len(payload)) {
			rows.Close()
			return result, ErrCorrupt
		}
		result.Records++
		result.PayloadBytes += storedBytes
	}
	rowErr := rows.Err()
	rows.Close()
	if rowErr != nil {
		return result, rowErr
	}
	if result.Records != expectedRecords || result.PayloadBytes != expectedBytes {
		return result, ErrCorrupt
	}
	positions, err := tx.QueryContext(ctx, "SELECT host_id,source,cursor FROM checkpoints")
	if err != nil {
		return result, err
	}
	for positions.Next() {
		var host, source, cursor string
		if err := positions.Scan(&host, &source, &cursor); err != nil {
			positions.Close()
			return result, err
		}
		if !telemetry.ValidHost(host) || source != SSHSource || !validCursor(cursor, false) {
			positions.Close()
			return result, ErrCorrupt
		}
		result.Checkpoints++
	}
	rowErr = positions.Err()
	positions.Close()
	if rowErr != nil {
		return result, rowErr
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	result.Valid = true
	return result, nil
}
