package controlstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"watchhouse/internal/detection"
	"watchhouse/internal/strictjson"
	"watchhouse/internal/telemetry"
)

type DetectionResult struct {
	EventsScanned    int `json:"events_scanned"`
	FindingsObserved int `json:"findings_observed"`
	FindingsInserted int `json:"findings_inserted"`
	FindingsExisting int `json:"findings_existing"`
}

func (store *Store) RunSSHDetection(ctx context.Context, host string, config detection.Config, maxScan int) (DetectionResult, error) {
	var result DetectionResult
	if !telemetry.ValidHost(host) || maxScan < 1 || maxScan > 100_000 {
		return result, fmt.Errorf("invalid detection scan scope")
	}
	detector, err := detection.NewSSH(config)
	if err != nil {
		return result, err
	}
	rows, err := store.pool.Query(ctx, `SELECT payload FROM control_events WHERE host_id=$1 ORDER BY observed_at ASC,ingest_sequence ASC LIMIT $2`, host, maxScan+1)
	if err != nil {
		return result, fmt.Errorf("query detection events: %w", err)
	}
	var events []telemetry.Event
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			rows.Close()
			return result, err
		}
		event, err := strictjson.Decode[telemetry.Event](bytes.NewReader(payload), telemetry.MaxRecordBytes)
		if err != nil || event.Validate() != nil || event.HostID != host {
			rows.Close()
			return result, fmt.Errorf("stored detection event failed integrity validation")
		}
		events = append(events, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	if len(events) > maxScan {
		return result, fmt.Errorf("detection scan exceeds %d events", maxScan)
	}
	result.EventsScanned = len(events)
	var findings []detection.Finding
	for _, event := range events {
		finding, err := detector.Observe(event)
		if err != nil {
			return result, fmt.Errorf("evaluate ordered detection stream: %w", err)
		}
		if finding != nil {
			findings = append(findings, *finding)
		}
	}
	result.FindingsObserved = len(findings)
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	configuration, _ := json.Marshal(struct {
		WindowSeconds int64 `json:"window_seconds"`
		Threshold     int   `json:"threshold"`
	}{int64(config.Window.Seconds()), config.Threshold})
	for _, finding := range findings {
		evidence, err := json.Marshal(finding.Evidence)
		if err != nil {
			return result, err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO control_findings(finding_id,host_id,rule_id,observed_at,priority,summary,rule_config,evidence_event_ids)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(finding_id) DO UPDATE SET last_evaluated_at=statement_timestamp()
			WHERE control_findings.host_id=EXCLUDED.host_id AND control_findings.rule_id=EXCLUDED.rule_id AND control_findings.observed_at=EXCLUDED.observed_at
			AND control_findings.priority=EXCLUDED.priority AND control_findings.summary=EXCLUDED.summary
			AND control_findings.rule_config=EXCLUDED.rule_config AND control_findings.evidence_event_ids=EXCLUDED.evidence_event_ids`,
			finding.FindingID, finding.HostID, finding.RuleID, finding.ObservedAt, finding.Priority, finding.Summary, configuration, evidence)
		if err != nil {
			return result, fmt.Errorf("persist detection finding: %w", err)
		}
		if tag.RowsAffected() != 1 {
			return result, fmt.Errorf("existing finding identity has conflicting content")
		}
		var created bool
		if err := tx.QueryRow(ctx, `SELECT created_at=last_evaluated_at FROM control_findings WHERE finding_id=$1`, finding.FindingID).Scan(&created); err != nil {
			return result, err
		}
		if created {
			result.FindingsInserted++
		} else {
			result.FindingsExisting++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}
