package controlstore

import (
	"bytes"
	"context"
	"fmt"

	"watchhouse/internal/strictjson"
	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

func (store *Store) QueryEvents(ctx context.Context, host string, limit int, before int64) ([]transport.EventRecord, error) {
	if !telemetry.ValidHost(host) || limit < 1 || limit > 200 || before < 0 {
		return nil, fmt.Errorf("invalid event query")
	}
	query := `SELECT ingest_sequence,host_id,event_id,ingested_at,payload FROM control_events WHERE host_id=$1 ORDER BY ingest_sequence DESC LIMIT $2`
	arguments := []any{host, limit}
	if before > 0 {
		query = `SELECT ingest_sequence,host_id,event_id,ingested_at,payload FROM control_events WHERE host_id=$1 AND ingest_sequence<$2 ORDER BY ingest_sequence DESC LIMIT $3`
		arguments = []any{host, before, limit}
	}
	rows, err := store.pool.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query authenticated events: %w", err)
	}
	defer rows.Close()
	result := make([]transport.EventRecord, 0)
	for rows.Next() {
		var record transport.EventRecord
		var payload []byte
		if err := rows.Scan(&record.IngestSequence, &record.HostID, &record.EventID, &record.IngestedAt, &payload); err != nil {
			return nil, err
		}
		event, err := strictjson.Decode[telemetry.Event](bytes.NewReader(payload), telemetry.MaxRecordBytes)
		if err != nil || event.Validate() != nil || event.HostID != record.HostID || event.EventID != record.EventID || record.IngestSequence < 1 || record.IngestedAt.IsZero() {
			return nil, fmt.Errorf("stored event failed query integrity validation")
		}
		record.Event, record.IngestedAt = event, record.IngestedAt.UTC()
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
