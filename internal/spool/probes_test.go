package spool

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/extprobe"
)

func queuedProbe() extprobe.Result {
	return extprobe.Result{Type: "external_https_probe", URL: "https://203.0.113.10/health", ObservedAt: time.Unix(1770000000, 0).UTC(),
		ResolvedAddresses: []string{"203.0.113.10"}, ConnectedAddress: "203.0.113.10:443", DNSDuration: time.Millisecond,
		TCPDuration: time.Millisecond, TLSDuration: time.Millisecond, TotalDuration: 4 * time.Millisecond, TLSVersion: "TLS 1.3",
		CipherSuite: "TLS_AES_128_GCM_SHA256", PeerCertificateSHA256: strings.Repeat("a", 64), HTTPStatus: 200,
		ExpectedStatus: 200, ContentType: "text/plain", BodyBytes: 2, BodySHA256: strings.Repeat("b", 64), Expected: true}
}

func TestProbeOutboxExactAckAndReopen(t *testing.T) {
	ctx := context.Background()
	store, dir := openTest(t, DefaultOptions())
	result := queuedProbe()
	appended, err := store.AppendProbeObservation(ctx, "outside-1", result)
	if err != nil || !appended.Inserted {
		t.Fatalf("append %+v error %v", appended, err)
	}
	appended, err = store.AppendProbeObservation(ctx, "outside-1", result)
	if err != nil || !appended.Duplicate {
		t.Fatalf("duplicate %+v error %v", appended, err)
	}
	items, err := store.PeekProbeObservations(ctx, 10, 1024*1024)
	if err != nil || len(items) != 1 || items[0].ProbeID != "outside-1" {
		t.Fatalf("peek %+v error %v", items, err)
	}
	if _, err := store.AckProbeObservations(ctx, []ProbeReceipt{{Sequence: items[0].Sequence, ObservationID: strings.Repeat("c", 64)}}); !errors.Is(err, ErrAckConflict) {
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
	items, err = reopened.PeekProbeObservations(ctx, 10, 1024*1024)
	if err != nil || len(items) != 1 {
		t.Fatalf("reopened peek %+v error %v", items, err)
	}
	deleted, err := reopened.AckProbeObservations(ctx, []ProbeReceipt{{Sequence: items[0].Sequence, ObservationID: items[0].ObservationID}})
	if err != nil || deleted != 1 {
		t.Fatalf("ack %d error %v", deleted, err)
	}
	stats, err := reopened.Stats(ctx)
	if err != nil || stats.ProbePendingRecords != 0 || stats.ProbePendingBytes != 0 {
		t.Fatalf("stats %+v error %v", stats, err)
	}
}

func TestProbeOutboxCapacityRetainsOldest(t *testing.T) {
	ctx := context.Background()
	store, _ := openTest(t, Options{MaxBytes: 2048, MaxRecords: 1})
	result := queuedProbe()
	if _, err := store.AppendProbeObservation(ctx, "outside-1", result); err != nil {
		t.Fatal(err)
	}
	result.ObservedAt = result.ObservedAt.Add(time.Second)
	if _, err := store.AppendProbeObservation(ctx, "outside-1", result); !errors.Is(err, ErrFull) {
		t.Fatalf("capacity error %v", err)
	}
	items, err := store.PeekProbeObservations(ctx, 10, 2048)
	if err != nil || len(items) != 1 {
		t.Fatalf("retained items %+v error %v", items, err)
	}
}
