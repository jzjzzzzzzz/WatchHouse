package delivery

import (
	"context"
	"errors"
	"testing"

	"watchhouse/internal/extprobe"
	"watchhouse/internal/spool"
	"watchhouse/internal/transport"
)

type probeQueueStub struct {
	items    []spool.ProbeItem
	receipts []spool.ProbeReceipt
}

func (queue *probeQueueStub) PeekProbeObservations(context.Context, int, int64) ([]spool.ProbeItem, error) {
	return queue.items, nil
}
func (queue *probeQueueStub) AckProbeObservations(_ context.Context, receipts []spool.ProbeReceipt) (int, error) {
	queue.receipts = receipts
	return len(receipts), nil
}
func (queue *probeQueueStub) Stats(context.Context) (spool.Stats, error) { return spool.Stats{}, nil }

type probeSenderStub struct {
	receipt transport.ProbeObservationReceipt
	err     error
}

func (sender probeSenderStub) PublishProbeObservation(context.Context, string, extprobe.Result) (transport.ProbeObservationReceipt, error) {
	return sender.receipt, sender.err
}

func TestOnceProbesDeletesOnlyAfterExactReceipt(t *testing.T) {
	item := spool.ProbeItem{Sequence: 7, ObservationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ProbeID: "outside-1"}
	queue := &probeQueueStub{items: []spool.ProbeItem{item}}
	result, err := OnceProbes(context.Background(), queue, probeSenderStub{receipt: transport.ProbeObservationReceipt{SchemaVersion: 1, ObservationID: item.ObservationID}}, 1024)
	if err != nil || result.Acknowledged != 1 || len(queue.receipts) != 1 || queue.receipts[0].Sequence != 7 {
		t.Fatalf("result %+v receipts %+v error %v", result, queue.receipts, err)
	}
}

func TestOnceProbesRetainsOnSendAndReceiptFailure(t *testing.T) {
	item := spool.ProbeItem{Sequence: 7, ObservationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ProbeID: "outside-1"}
	for _, sender := range []probeSenderStub{{err: errors.New("network")}, {receipt: transport.ProbeObservationReceipt{SchemaVersion: 1, ObservationID: "wrong"}}} {
		queue := &probeQueueStub{items: []spool.ProbeItem{item}}
		if _, err := OnceProbes(context.Background(), queue, sender, 1024); err == nil || len(queue.receipts) != 0 {
			t.Fatalf("failure was acknowledged: %v %+v", err, queue.receipts)
		}
	}
}
