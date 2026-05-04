package delivery

import (
	"context"
	"errors"
	"testing"

	"watchhouse/internal/spool"
)

type fakeQueue struct {
	items           []spool.Item
	peekErr, ackErr error
	acked           bool
}

func (queue *fakeQueue) Peek(context.Context, int, int64) ([]spool.Item, error) {
	return queue.items, queue.peekErr
}
func (queue *fakeQueue) Ack(_ context.Context, receipts []spool.Receipt) (int, error) {
	queue.acked = true
	if queue.ackErr != nil {
		return 0, queue.ackErr
	}
	queue.items = nil
	return len(receipts), nil
}
func (queue *fakeQueue) Stats(context.Context) (spool.Stats, error) {
	return spool.Stats{PendingRecords: len(queue.items)}, nil
}

type fakeSender struct {
	receipts []spool.Receipt
	err      error
	calls    int
}

func (sender *fakeSender) Deliver(context.Context, []spool.Item) ([]spool.Receipt, error) {
	sender.calls++
	return sender.receipts, sender.err
}

func TestEmptyQueueDoesNotContactRemote(t *testing.T) {
	queue, sender := &fakeQueue{}, &fakeSender{}
	result, err := Once(context.Background(), queue, sender, 100, 1024)
	if err != nil || result.RemoteContacted || sender.calls != 0 || queue.acked {
		t.Fatalf("result %+v err %v", result, err)
	}
}

func TestRemoteSuccessThenExactLocalAck(t *testing.T) {
	item := spool.Item{Sequence: 1, EventID: "event"}
	queue := &fakeQueue{items: []spool.Item{item}}
	sender := &fakeSender{receipts: []spool.Receipt{{Sequence: 1, EventID: "event"}}}
	result, err := Once(context.Background(), queue, sender, 100, 1024)
	if err != nil || !queue.acked || result.Acknowledged != 1 {
		t.Fatalf("result %+v err %v", result, err)
	}
}

func TestDeliveryFailureNeverAcknowledges(t *testing.T) {
	queue := &fakeQueue{items: []spool.Item{{Sequence: 1, EventID: "event"}}}
	sender := &fakeSender{err: errors.New("TLS failed")}
	if _, err := Once(context.Background(), queue, sender, 100, 1024); err == nil || queue.acked {
		t.Fatal("failed delivery acknowledged")
	}
	queue.ackErr = errors.New("disk failed")
	sender.err = nil
	sender.receipts = []spool.Receipt{{Sequence: 1, EventID: "event"}}
	if _, err := Once(context.Background(), queue, sender, 100, 1024); err == nil || !queue.acked {
		t.Fatal("local ack failure not surfaced")
	}
}

func TestPartialReceiptNeverReachesQueueAck(t *testing.T) {
	queue := &fakeQueue{items: []spool.Item{{Sequence: 1, EventID: "one"}, {Sequence: 2, EventID: "two"}}}
	sender := &fakeSender{receipts: []spool.Receipt{{Sequence: 1, EventID: "one"}}}
	if _, err := Once(context.Background(), queue, sender, 100, 1024); err == nil || queue.acked {
		t.Fatal("partial receipt reached destructive acknowledgement")
	}
}

func TestDeliveryConfigurationBoundedBeforeQueueAccess(t *testing.T) {
	queue := &fakeQueue{peekErr: errors.New("should not be called")}
	if _, err := Once(context.Background(), queue, &fakeSender{}, 501, 1); err == nil {
		t.Fatal("invalid limit accepted")
	}
}
