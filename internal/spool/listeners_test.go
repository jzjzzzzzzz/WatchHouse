package spool

import (
	"context"
	"errors"
	"testing"
	"time"

	"watchhouse/internal/hostview"
)

func queuedSnapshot() hostview.HostSnapshot {
	return hostview.HostSnapshot{Type: "tcp_listener_snapshot", ObservedAt: time.Unix(1770000000, 0).UTC(),
		BootID: "12345678-1234-1234-1234-123456789abc", NetworkNamespace: "net:[4026531840]",
		Listeners: []hostview.Listener{{Socket: hostview.Socket{Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 443, KernelUID: 1000, Inode: 42}, Ownership: "unknown_unmapped"}}}
}

func TestListenerOutboxExactAckAndReopen(t *testing.T) {
	ctx := context.Background()
	store, dir := openTest(t, DefaultOptions())
	snapshot := queuedSnapshot()
	result, err := store.AppendListenerSnapshot(ctx, "host-1", snapshot)
	if err != nil || !result.Inserted {
		t.Fatalf("append %+v error %v", result, err)
	}
	result, err = store.AppendListenerSnapshot(ctx, "host-1", snapshot)
	if err != nil || !result.Duplicate {
		t.Fatalf("duplicate %+v error %v", result, err)
	}
	items, err := store.PeekListenerSnapshots(ctx, 10, 1024*1024)
	if err != nil || len(items) != 1 || items[0].HostID != "host-1" {
		t.Fatalf("peek %+v error %v", items, err)
	}
	if _, err := store.AckListenerSnapshots(ctx, []ListenerReceipt{{Sequence: items[0].Sequence, SnapshotID: hostview.SnapshotIdentity("other", snapshot)}}); !errors.Is(err, ErrAckConflict) {
		t.Fatalf("wrong receipt: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, dir, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	items, err = reopened.PeekListenerSnapshots(ctx, 10, 1024*1024)
	if err != nil || len(items) != 1 {
		t.Fatalf("reopened peek %+v error %v", items, err)
	}
	deleted, err := reopened.AckListenerSnapshots(ctx, []ListenerReceipt{{Sequence: items[0].Sequence, SnapshotID: items[0].SnapshotID}})
	if err != nil || deleted != 1 {
		t.Fatalf("ack %d error %v", deleted, err)
	}
	stats, err := reopened.Stats(ctx)
	if err != nil || stats.ListenerPendingRecords != 0 || stats.ListenerPendingBytes != 0 {
		t.Fatalf("stats %+v error %v", stats, err)
	}
}

func TestListenerOutboxCapacityDoesNotDropSnapshot(t *testing.T) {
	ctx := context.Background()
	store, _ := openTest(t, Options{MaxBytes: 1024, MaxRecords: 1})
	snapshot := queuedSnapshot()
	if _, err := store.AppendListenerSnapshot(ctx, "host-1", snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.ObservedAt = snapshot.ObservedAt.Add(time.Second)
	if _, err := store.AppendListenerSnapshot(ctx, "host-1", snapshot); !errors.Is(err, ErrFull) {
		t.Fatalf("capacity error %v", err)
	}
	items, err := store.PeekListenerSnapshots(ctx, 10, 1024)
	if err != nil || len(items) != 1 {
		t.Fatalf("retained items %+v error %v", items, err)
	}
}
