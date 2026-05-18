package controlstore

import (
	"context"
	"fmt"

	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

func (store *Store) QueryFindings(ctx context.Context, host string, limit int, before int64) ([]transport.FindingRecord, error) {
	if !telemetry.ValidHost(host) || limit < 1 || limit > 200 || before < 0 {
		return nil, fmt.Errorf("invalid finding query")
	}
	query := `SELECT finding_sequence,finding_id,host_id,rule_id,observed_at,priority,summary,
		(rule_config->>'window_seconds')::bigint,(rule_config->>'threshold')::integer,evidence_event_ids,
		created_at,last_evaluated_at FROM control_findings WHERE host_id=$1 ORDER BY finding_sequence DESC LIMIT $2`
	arguments := []any{host, limit}
	if before > 0 {
		query = `SELECT finding_sequence,finding_id,host_id,rule_id,observed_at,priority,summary,
			(rule_config->>'window_seconds')::bigint,(rule_config->>'threshold')::integer,evidence_event_ids,
			created_at,last_evaluated_at FROM control_findings WHERE host_id=$1 AND finding_sequence<$2 ORDER BY finding_sequence DESC LIMIT $3`
		arguments = []any{host, before, limit}
	}
	rows, err := store.pool.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query authenticated findings: %w", err)
	}
	defer rows.Close()
	result := make([]transport.FindingRecord, 0)
	for rows.Next() {
		var record transport.FindingRecord
		if err := rows.Scan(&record.FindingSequence, &record.FindingID, &record.HostID, &record.RuleID,
			&record.ObservedAt, &record.Priority, &record.Summary, &record.WindowSeconds, &record.Threshold,
			&record.EvidenceEventIDs, &record.CreatedAt, &record.LastEvaluatedAt); err != nil {
			return nil, err
		}
		if record.FindingSequence < 1 || record.HostID != host || len(record.FindingID) != 64 ||
			record.RuleID == "" || record.Priority == "" || record.Summary == "" || record.WindowSeconds < 1 ||
			record.Threshold < 1 || len(record.EvidenceEventIDs) < 1 || record.ObservedAt.IsZero() ||
			record.CreatedAt.IsZero() || record.LastEvaluatedAt.IsZero() {
			return nil, fmt.Errorf("stored finding failed query integrity validation")
		}
		record.ObservedAt = record.ObservedAt.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		record.LastEvaluatedAt = record.LastEvaluatedAt.UTC()
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
