package delivery

import (
	"context"
	"errors"
	"testing"

	"watchhouse/internal/hostview"
	"watchhouse/internal/spool"
	"watchhouse/internal/transport"
)

type listenerQueueStub struct {
	items    []spool.ListenerItem
	receipts []spool.ListenerReceipt
	stats    spool.Stats
}

func (queue *listenerQueueStub) PeekListenerSnapshots(context.Context, int, int64) ([]spool.ListenerItem, error) {
	return queue.items, nil
}
func (queue *listenerQueueStub) AckListenerSnapshots(_ context.Context, receipts []spool.ListenerReceipt) (int, error) {
	queue.receipts = receipts
	return len(receipts), nil
}
func (queue *listenerQueueStub) Stats(context.Context) (spool.Stats, error) { return queue.stats, nil }

type listenerSenderStub struct {
	receipt transport.ListenerSnapshotReceipt
	err     error
}

func (sender listenerSenderStub) PublishListenerSnapshot(context.Context, string, hostview.HostSnapshot) (transport.ListenerSnapshotReceipt, error) {
	return sender.receipt, sender.err
}

func TestOnceListenersDeletesOnlyAfterExactReceipt(t *testing.T) {
	item := spool.ListenerItem{Sequence: 7, SnapshotID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", HostID: "host-1"}
	queue := &listenerQueueStub{items: []spool.ListenerItem{item}}
	result, err := OnceListeners(context.Background(), queue, listenerSenderStub{receipt: transport.ListenerSnapshotReceipt{SchemaVersion: 1, SnapshotID: item.SnapshotID}}, 1024)
	if err != nil || result.Acknowledged != 1 || len(queue.receipts) != 1 || queue.receipts[0].Sequence != 7 {
		t.Fatalf("result %+v receipts %+v error %v", result, queue.receipts, err)
	}
}

func TestOnceListenersRetainsOnSendAndReceiptFailure(t *testing.T) {
	item := spool.ListenerItem{Sequence: 7, SnapshotID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", HostID: "host-1"}
	for _, sender := range []listenerSenderStub{{err: errors.New("network")}, {receipt: transport.ListenerSnapshotReceipt{SchemaVersion: 1, SnapshotID: "wrong"}}} {
		queue := &listenerQueueStub{items: []spool.ListenerItem{item}}
		if _, err := OnceListeners(context.Background(), queue, sender, 1024); err == nil || len(queue.receipts) != 0 {
			t.Fatalf("failure was acknowledged: %v %+v", err, queue.receipts)
		}
	}
}
