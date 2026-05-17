package controlstore

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"watchhouse/internal/detection"
	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

func integrationItem(sequence int64, cursor string) transport.Item {
	event := telemetry.Event{SchemaVersion: 1, HostID: "host-1", BootID: "0123456789abcdef0123456789abcdef", Source: "journald", SourceCursor: cursor, ObservedAt: time.Unix(1770000000+sequence, 0).UTC(), ReceivedAt: time.Unix(1770000100, 0).UTC(), Kind: "ssh.authentication", Authentication: telemetry.Authentication{Outcome: "failed", Method: "publickey", User: "alice", SourceIP: "192.0.2.1", SourcePort: 2222}}
	event.EventID = telemetry.Identity(event.HostID, event.BootID, event.SourceCursor)
	return transport.Item{Sequence: sequence, EventID: event.EventID, Event: event}
}

func TestPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("WATCHHOUSE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL integration DSN not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("idempotent migration: %v", err)
	}
	store, err := New(pool)
	if err != nil {
		t.Fatal(err)
	}
	first := integrationItem(1, "s=integration;i=1")
	if err := store.CommitBatch(ctx, "host-1", []transport.Item{first}); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBatch(ctx, "host-1", []transport.Item{first}); err != nil {
		t.Fatalf("idempotent repeat: %v", err)
	}
	count, err := store.EventCount(ctx, "host-1")
	if err != nil || count != 1 {
		t.Fatalf("count %d error %v", count, err)
	}

	newItem := integrationItem(2, "s=integration;i=2")
	conflict := first
	conflict.Event.Authentication.User = "mallory"
	err = store.CommitBatch(ctx, "host-1", []transport.Item{newItem, conflict})
	if !errors.Is(err, transport.ErrEventConflict) {
		t.Fatalf("conflict error %v", err)
	}
	count, err = store.EventCount(ctx, "host-1")
	if err != nil || count != 1 {
		t.Fatalf("conflicting batch was not atomic: count %d error %v", count, err)
	}

	concurrent := integrationItem(3, "s=integration;i=3")
	var wait sync.WaitGroup
	errorsSeen := make(chan error, 8)
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsSeen <- store.CommitBatch(context.Background(), "host-1", []transport.Item{concurrent})
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Errorf("concurrent repeat: %v", err)
		}
	}
	count, err = store.EventCount(ctx, "host-1")
	if err != nil || count != 2 {
		t.Fatalf("concurrent idempotency count %d error %v", count, err)
	}
	page, err := store.QueryEvents(ctx, "host-1", 1, 0)
	if err != nil || len(page) != 1 {
		t.Fatalf("first query page %+v error %v", page, err)
	}
	older, err := store.QueryEvents(ctx, "host-1", 2, page[0].IngestSequence)
	if err != nil || len(older) != 1 || older[0].EventID == page[0].EventID || older[0].IngestSequence >= page[0].IngestSequence {
		t.Fatalf("older query page %+v after %+v error %v", older, page, err)
	}
	if _, err := store.QueryEvents(ctx, "../host", 1, 0); err == nil {
		t.Fatal("invalid query host accepted")
	}
	var sequence []transport.Item
	for index := 0; index < 6; index++ {
		item := integrationItem(int64(index+20), "s=detection;i="+strconv.Itoa(index))
		item.Event.HostID = "detect-host"
		item.Event.ObservedAt = time.Unix(1772000000+int64(index), 0).UTC()
		if index == 5 {
			item.Event.Authentication.Outcome = "accepted"
		}
		item.Event.EventID = telemetry.Identity(item.Event.HostID, item.Event.BootID, item.Event.SourceCursor)
		item.EventID = item.Event.EventID
		sequence = append(sequence, item)
	}
	if err := store.CommitBatch(ctx, "detect-host", sequence); err != nil {
		t.Fatal(err)
	}
	detected, err := store.RunSSHDetection(ctx, "detect-host", detection.DefaultConfig(), 100)
	if err != nil || detected.FindingsObserved != 1 || detected.FindingsInserted != 1 {
		t.Fatalf("detection %+v error %v", detected, err)
	}
	repeated, err := store.RunSSHDetection(ctx, "detect-host", detection.DefaultConfig(), 100)
	if err != nil || repeated.FindingsObserved != 1 || repeated.FindingsExisting != 1 {
		t.Fatalf("repeat detection %+v error %v", repeated, err)
	}
	var evidenceCount int
	if err := pool.QueryRow(ctx, `SELECT jsonb_array_length(evidence_event_ids) FROM control_findings WHERE host_id='detect-host'`).Scan(&evidenceCount); err != nil || evidenceCount != 6 {
		t.Fatalf("finding evidence count %d error %v", evidenceCount, err)
	}
}
