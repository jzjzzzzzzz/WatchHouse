package controlstore

import (
	"bytes"
	"testing"
	"time"

	"watchhouse/internal/telemetry"
)

func storedEvent(received time.Time) telemetry.Event {
	event := telemetry.Event{SchemaVersion: 1, HostID: "host-1", BootID: "0123456789abcdef0123456789abcdef", Source: "journald", SourceCursor: "s=test;i=1", ObservedAt: time.Unix(1, 0).UTC(), ReceivedAt: received, Kind: "ssh.authentication", Authentication: telemetry.Authentication{Outcome: "failed", Method: "publickey", User: "alice", SourceIP: "192.0.2.1", SourcePort: 2222}}
	event.EventID = telemetry.Identity(event.HostID, event.BootID, event.SourceCursor)
	return event
}

func TestControlContentDigestIgnoresOnlyReceiveTime(t *testing.T) {
	firstBody, firstDigest, err := eventContent(storedEvent(time.Unix(2, 0).UTC()))
	if err != nil {
		t.Fatal(err)
	}
	secondBody, secondDigest, err := eventContent(storedEvent(time.Unix(3, 0).UTC()))
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest || bytes.Equal(firstBody, secondBody) {
		t.Fatalf("digest %q %q bodies equal=%v", firstDigest, secondDigest, bytes.Equal(firstBody, secondBody))
	}
	event := storedEvent(time.Unix(2, 0).UTC())
	event.Authentication.User = "mallory"
	_, changed, err := eventContent(event)
	if err != nil || changed == firstDigest {
		t.Fatalf("changed digest %q error %v", changed, err)
	}
}

func TestEmbeddedSchemaHasIdentityAndTimeConstraints(t *testing.T) {
	body, err := migrations.ReadFile("migrations/001.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range [][]byte{[]byte("PRIMARY KEY (host_id, event_id)"), []byte("content_digest"), []byte("observed_at timestamptz"), []byte("schema_version = 1")} {
		if !bytes.Contains(body, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if _, err := New(nil); err == nil {
		t.Fatal("nil PostgreSQL pool accepted")
	}
	second, err := migrations.ReadFile("migrations/002.sql")
	if err != nil || !bytes.Contains(second, []byte("ingest_sequence")) || !bytes.Contains(second, []byte("GENERATED ALWAYS AS IDENTITY")) {
		t.Fatalf("query-sequence migration missing: %v", err)
	}
}
