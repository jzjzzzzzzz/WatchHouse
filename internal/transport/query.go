package transport

import (
	"context"
	"time"

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
