package transport

import (
	"context"
	"time"

	"watchhouse/internal/authz"
	"watchhouse/internal/telemetry"
)

type EventRecord struct {
	IngestSequence int64           `json:"ingest_sequence"`
	HostID         string          `json:"host_id"`
	EventID        string          `json:"event_id"`
	IngestedAt     time.Time       `json:"ingested_at"`
	Event          telemetry.Event `json:"event"`
}

type EventPage struct {
	SchemaVersion int           `json:"schema_version"`
	Records       []EventRecord `json:"records"`
}

type EventQueryStore interface {
	QueryEvents(context.Context, string, int, int64) ([]EventRecord, error)
}

type FindingRecord struct {
	FindingSequence  int64     `json:"finding_sequence"`
	FindingID        string    `json:"finding_id"`
	HostID           string    `json:"host_id"`
	RuleID           string    `json:"rule_id"`
	ObservedAt       time.Time `json:"observed_at"`
	Priority         string    `json:"priority"`
	Summary          string    `json:"summary"`
	WindowSeconds    int64     `json:"window_seconds"`
	Threshold        int       `json:"threshold"`
	EvidenceEventIDs []string  `json:"evidence_event_ids"`
	CreatedAt        time.Time `json:"created_at"`
	LastEvaluatedAt  time.Time `json:"last_evaluated_at"`
}

type FindingPage struct {
	SchemaVersion int             `json:"schema_version"`
	Records       []FindingRecord `json:"records"`
}

type FindingQueryStore interface {
	QueryFindings(context.Context, string, int, int64) ([]FindingRecord, error)
}

type QueryAuditor interface {
	RecordQueryDecision(context.Context, string, authz.Role, string, string, string) error
}
