package delivery

import (
	"context"
	"fmt"

	"watchhouse/internal/hostview"
	"watchhouse/internal/spool"
	"watchhouse/internal/transport"
)

type ListenerQueue interface {
	PeekListenerSnapshots(context.Context, int, int64) ([]spool.ListenerItem, error)
	AckListenerSnapshots(context.Context, []spool.ListenerReceipt) (int, error)
	Stats(context.Context) (spool.Stats, error)
}

type ListenerSender interface {
	PublishListenerSnapshot(context.Context, string, hostview.HostSnapshot) (transport.ListenerSnapshotReceipt, error)
}

type ListenerResult struct {
	Attempted       int   `json:"attempted"`
	Acknowledged    int   `json:"acknowledged"`
	PendingRecords  int   `json:"pending_records"`
	PendingBytes    int64 `json:"pending_bytes"`
	RemoteContacted bool  `json:"remote_contacted"`
}

func OnceListeners(ctx context.Context, queue ListenerQueue, sender ListenerSender, maxBytes int64) (ListenerResult, error) {
	var result ListenerResult
	if queue == nil || sender == nil || maxBytes < 1 || maxBytes > 8*1024*1024 {
		return result, fmt.Errorf("invalid listener delivery configuration")
	}
	items, err := queue.PeekListenerSnapshots(ctx, 1, maxBytes)
	if err != nil {
		return result, fmt.Errorf("peek listener outbox: %w", err)
	}
	result.Attempted = len(items)
	if len(items) == 1 {
		result.RemoteContacted = true
		receipt, err := sender.PublishListenerSnapshot(ctx, items[0].HostID, items[0].Snapshot)
		if err != nil {
			return result, fmt.Errorf("publish listener snapshot: %w", err)
		}
		if receipt.SchemaVersion != transport.SchemaVersion || receipt.SnapshotID != items[0].SnapshotID {
			return result, fmt.Errorf("listener sender returned a non-exact receipt")
		}
		deleted, err := queue.AckListenerSnapshots(ctx, []spool.ListenerReceipt{{Sequence: items[0].Sequence, SnapshotID: receipt.SnapshotID}})
		if err != nil {
			return result, fmt.Errorf("ack listener snapshot locally: %w", err)
		}
		if deleted != 1 {
			return result, fmt.Errorf("listener acknowledgement cardinality changed")
		}
		result.Acknowledged = 1
	}
	stats, err := queue.Stats(ctx)
	if err != nil {
		return result, fmt.Errorf("read listener outbox state: %w", err)
	}
	result.PendingRecords = stats.ListenerPendingRecords
	result.PendingBytes = stats.ListenerPendingBytes
	return result, nil
}
