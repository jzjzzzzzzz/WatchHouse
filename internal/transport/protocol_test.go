package transport

import (
	"crypto/x509"
	"fmt"
	"net/url"
	"testing"
	"time"

	"watchhouse/internal/telemetry"
)

func testItem(sequence int64, host string) Item {
	event := telemetry.Event{SchemaVersion: 1, HostID: host, BootID: "0123456789abcdef0123456789abcdef", Source: "journald", SourceCursor: fmt.Sprintf("s=x;i=%d", sequence), ObservedAt: time.Unix(1, 0).UTC(), ReceivedAt: time.Unix(2, 0).UTC(), Kind: "ssh.authentication", Authentication: telemetry.Authentication{Outcome: "failed", Method: "publickey", User: "alice", SourceIP: "192.0.2.1", SourcePort: 2222}}
	event.EventID = telemetry.Identity(event.HostID, event.BootID, event.SourceCursor)
	return Item{Sequence: sequence, EventID: event.EventID, Event: event}
}

func TestBatchAndExactReceipts(t *testing.T) {
	item := testItem(7, "host-1")
	request := BatchRequest{SchemaVersion: 1, Items: []Item{item}}
	if err := request.Validate("host-1"); err != nil {
		t.Fatal(err)
	}
	response := BatchResponse{SchemaVersion: 1, Receipts: []Receipt{{Sequence: 7, EventID: item.EventID}}}
	if err := response.ValidateExact(request.Items); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []BatchResponse{
		{SchemaVersion: 1},
		{SchemaVersion: 2, Receipts: response.Receipts},
		{SchemaVersion: 1, Receipts: []Receipt{{Sequence: 8, EventID: item.EventID}}},
		{SchemaVersion: 1, Receipts: []Receipt{{Sequence: 7, EventID: telemetry.Identity("other")}}},
	} {
		if err := bad.ValidateExact(request.Items); err == nil {
			t.Fatalf("bad receipt accepted: %+v", bad)
		}
	}
}

func TestBatchRejectsClaimedHostAndDuplicateIdentities(t *testing.T) {
	item := testItem(1, "host-1")
	for _, request := range []BatchRequest{
		{SchemaVersion: 1},
		{SchemaVersion: 1, Items: []Item{item, item}},
		{SchemaVersion: 1, Items: []Item{{Sequence: item.Sequence, EventID: "bad", Event: item.Event}}},
	} {
		if err := request.Validate("host-1"); err == nil {
			t.Fatalf("bad request accepted: %+v", request)
		}
	}
	if err := (BatchRequest{SchemaVersion: 1, Items: []Item{item}}).Validate("other-host"); err == nil {
		t.Fatal("claimed host accepted")
	}
}

func TestCertificateHostUsesOnlyOneExactURI(t *testing.T) {
	identity, err := HostURI("host-1")
	if err != nil {
		t.Fatal(err)
	}
	host, err := HostFromCertificate(&x509.Certificate{URIs: []*url.URL{identity}})
	if err != nil || host != "host-1" {
		t.Fatalf("host %q %v", host, err)
	}
	bad, _ := url.Parse("spiffe://watchhouse/host/host-1?role=admin")
	for _, certificate := range []*x509.Certificate{nil, {}, {URIs: []*url.URL{identity, identity}}, {URIs: []*url.URL{bad}}} {
		if _, err := HostFromCertificate(certificate); err == nil {
			t.Fatal("bad certificate identity accepted")
		}
	}
}
