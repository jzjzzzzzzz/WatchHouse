// Package delivery coordinates one bounded spool upload. It deliberately keeps
// remote commit and local deletion as separate idempotent steps.
package delivery

import (
	"context"
	"fmt"

	"watchhouse/internal/spool"
)

type Queue interface {
	Peek(context.Context, int, int64) ([]spool.Item, error)
	Ack(context.Context, []spool.Receipt) (int, error)
	Stats(context.Context) (spool.Stats, error)
}

type Sender interface {
	Deliver(context.Context, []spool.Item) ([]spool.Receipt, error)
}

type Result struct {
	Attempted       int   `json:"attempted"`
	Acknowledged    int   `json:"acknowledged"`
	PendingRecords  int   `json:"pending_records"`
	PendingBytes    int64 `json:"pending_bytes"`
	RemoteContacted bool  `json:"remote_contacted"`
}

func Once(ctx context.Context, queue Queue, sender Sender, limit int, maxBytes int64) (Result, error) {
	var result Result
	if queue == nil || sender == nil || limit < 1 || limit > 500 || maxBytes < 1 || maxBytes > 8*1024*1024 {
		return result, fmt.Errorf("invalid bounded delivery configuration")
	}
	items, err := queue.Peek(ctx, limit, maxBytes)
	if err != nil {
		return result, fmt.Errorf("peek delivery batch: %w", err)
	}
	result.Attempted = len(items)
	if len(items) > 0 {
		result.RemoteContacted = true
		receipts, err := sender.Deliver(ctx, items)
		if err != nil {
			return result, fmt.Errorf("deliver batch: %w", err)
		}
		if !exactReceipts(items, receipts) {
			return result, fmt.Errorf("sender returned a non-exact receipt set")
		}
		deleted, err := queue.Ack(ctx, receipts)
		if err != nil {
			return result, fmt.Errorf("ack committed batch locally: %w", err)
		}
		if deleted != len(items) {
			return result, fmt.Errorf("local acknowledgement cardinality changed")
		}
		result.Acknowledged = deleted
	}
	stats, err := queue.Stats(ctx)
	if err != nil {
		return result, fmt.Errorf("read post-delivery queue state: %w", err)
	}
	result.PendingRecords, result.PendingBytes = stats.PendingRecords, stats.PendingBytes
	return result, nil
}

func exactReceipts(items []spool.Item, receipts []spool.Receipt) bool {
	if len(items) != len(receipts) {
		return false
	}
	expected := make(map[int64]string, len(items))
	for _, item := range items {
		expected[item.Sequence] = item.EventID
	}
	seen := make(map[int64]struct{}, len(receipts))
	for _, receipt := range receipts {
		if expected[receipt.Sequence] != receipt.EventID {
			return false
		}
		if _, exists := seen[receipt.Sequence]; exists {
			return false
		}
		seen[receipt.Sequence] = struct{}{}
	}
	return true
}
