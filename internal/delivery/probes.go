package delivery

import (
	"context"
	"fmt"

	"watchhouse/internal/extprobe"
	"watchhouse/internal/spool"
	"watchhouse/internal/transport"
)

type ProbeQueue interface {
	PeekProbeObservations(context.Context, int, int64) ([]spool.ProbeItem, error)
	AckProbeObservations(context.Context, []spool.ProbeReceipt) (int, error)
	Stats(context.Context) (spool.Stats, error)
}

type ProbeSender interface {
	PublishProbeObservation(context.Context, string, extprobe.Result) (transport.ProbeObservationReceipt, error)
}

type ProbeResult struct {
	Attempted       int   `json:"attempted"`
	Acknowledged    int   `json:"acknowledged"`
	PendingRecords  int   `json:"pending_records"`
	PendingBytes    int64 `json:"pending_bytes"`
	RemoteContacted bool  `json:"remote_contacted"`
}

func OnceProbes(ctx context.Context, queue ProbeQueue, sender ProbeSender, maxBytes int64) (ProbeResult, error) {
	var result ProbeResult
	if queue == nil || sender == nil || maxBytes < 1 || maxBytes > 8*1024*1024 {
		return result, fmt.Errorf("invalid probe delivery configuration")
	}
	items, err := queue.PeekProbeObservations(ctx, 1, maxBytes)
	if err != nil {
		return result, fmt.Errorf("peek probe outbox: %w", err)
	}
	result.Attempted = len(items)
	if len(items) == 1 {
		result.RemoteContacted = true
		receipt, err := sender.PublishProbeObservation(ctx, items[0].ProbeID, items[0].Result)
		if err != nil {
			return result, fmt.Errorf("publish probe observation: %w", err)
		}
		if receipt.SchemaVersion != transport.SchemaVersion || receipt.ObservationID != items[0].ObservationID {
			return result, fmt.Errorf("probe sender returned a non-exact receipt")
		}
		deleted, err := queue.AckProbeObservations(ctx, []spool.ProbeReceipt{{Sequence: items[0].Sequence, ObservationID: receipt.ObservationID}})
		if err != nil {
			return result, fmt.Errorf("ack probe observation locally: %w", err)
		}
		if deleted != 1 {
			return result, fmt.Errorf("probe acknowledgement cardinality changed")
		}
		result.Acknowledged = 1
	}
	stats, err := queue.Stats(ctx)
	if err != nil {
		return result, fmt.Errorf("read probe outbox state: %w", err)
	}
	result.PendingRecords = stats.ProbePendingRecords
	result.PendingBytes = stats.ProbePendingBytes
	return result, nil
}
