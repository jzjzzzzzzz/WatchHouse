package spool

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"watchhouse/internal/telemetry"
)

func testEvent(n int) telemetry.Event {
	e := telemetry.Event{SchemaVersion: 1, HostID: "lab-1", BootID: "0123456789abcdef0123456789abcdef", Source: "journald", SourceCursor: fmt.Sprintf("s=test;i=%d", n), Kind: "ssh.authentication", ObservedAt: time.Unix(1775642400+int64(n), 0).UTC(), ReceivedAt: time.Unix(1775642500, 0).UTC(), Authentication: telemetry.Authentication{Outcome: "failed", Method: "password", User: "alice", SourceIP: "192.0.2.10", SourcePort: 51000}}
	e.EventID = telemetry.Identity(e.HostID, e.BootID, e.SourceCursor)
	return e
}

func checkpoint(previous string, e telemetry.Event) Checkpoint {
	return Checkpoint{HostID: e.HostID, Source: SSHSource, ExpectedCursor: previous, NextCursor: e.SourceCursor}
}

func TestAppendAtomicCursorAndDuplicate(t *testing.T) {
	s, dir := openTest(t, DefaultOptions())
	ctx := context.Background()
	e := testEvent(1)
	r, err := s.Append(ctx, checkpoint("", e), &e)
	if err != nil || !r.Inserted {
		t.Fatalf("insert %+v %v", r, err)
	}
	cursor, err := s.Cursor(ctx, e.HostID, SSHSource)
	if err != nil || cursor != e.SourceCursor {
		t.Fatal("checkpoint not persisted")
	}
	e.ReceivedAt = e.ReceivedAt.Add(time.Hour)
	r, err = s.Append(ctx, checkpoint("", e), &e)
	if err != nil || !r.Duplicate {
		t.Fatalf("replay %+v %v", r, err)
	}
	stats, _ := s.Stats(ctx)
	if stats.PendingRecords != 1 || stats.PendingBytes <= 0 {
		t.Fatal("duplicate stored twice")
	}
	s.Close()
	s2, err := Open(ctx, dir, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	cursor, err = s2.Cursor(ctx, e.HostID, SSHSource)
	if err != nil || cursor != e.SourceCursor {
		t.Fatal("checkpoint lost across reopen")
	}
	stats, _ = s2.Stats(ctx)
	if stats.PendingRecords != 1 {
		t.Fatal("event lost across reopen")
	}
}

func TestCapacityDoesNotAdvanceCheckpoint(t *testing.T) {
	s, _ := openTest(t, Options{MaxBytes: 1024 * 1024, MaxRecords: 1})
	ctx := context.Background()
	e1, e2 := testEvent(1), testEvent(2)
	if _, err := s.Append(ctx, checkpoint("", e1), &e1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(ctx, checkpoint(e1.SourceCursor, e2), &e2); !errors.Is(err, ErrFull) {
		t.Fatalf("want full got %v", err)
	}
	cursor, _ := s.Cursor(ctx, e1.HostID, SSHSource)
	if cursor != e1.SourceCursor {
		t.Fatal("full queue advanced source")
	}
	stats, _ := s.Stats(ctx)
	if stats.BlockedAttempts != 1 || stats.PendingRecords != 1 {
		t.Fatal("full queue not observable")
	}
	// Unmatched records consume no payload capacity but still have a cursor.
	cp := Checkpoint{e1.HostID, SSHSource, e1.SourceCursor, "s=unmatched;i=3"}
	if _, err := s.Append(ctx, cp, nil); err != nil {
		t.Fatal(err)
	}
	cursor, _ = s.Cursor(ctx, e1.HostID, SSHSource)
	if cursor != cp.NextCursor {
		t.Fatal("unmatched source cursor not stored")
	}
}

func TestStaleAndConflictingEvents(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	e1, e2 := testEvent(1), testEvent(2)
	s.Append(ctx, checkpoint("", e1), &e1)
	if _, err := s.Append(ctx, checkpoint("", e2), &e2); !errors.Is(err, ErrStaleCheckpoint) {
		t.Fatalf("stale accepted: %v", err)
	}
	conflict := e1
	conflict.Authentication.User = "bob"
	if _, err := s.Append(ctx, checkpoint("", conflict), &conflict); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict accepted: %v", err)
	}
	if _, err := s.Append(ctx, checkpoint(e1.SourceCursor, e2), &e2); err != nil {
		t.Fatal("error consumed valid checkpoint")
	}
}

func TestTransactionRollbackOnCheckpointFailure(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	e := testEvent(1)
	_, err := s.db.Exec(`CREATE TRIGGER refuse_checkpoint BEFORE INSERT ON checkpoints BEGIN SELECT RAISE(ABORT,'injected checkpoint failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(ctx, checkpoint("", e), &e); err == nil {
		t.Fatal("injected failure ignored")
	}
	stats, _ := s.Stats(ctx)
	cursor, _ := s.Cursor(ctx, e.HostID, SSHSource)
	if stats.PendingRecords != 0 || stats.PendingBytes != 0 || cursor != "" {
		t.Fatal("partial transaction survived")
	}
}

func TestAppendValidationAndCancellation(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	e := testEvent(1)
	for _, cp := range []Checkpoint{{"bad host", SSHSource, "", e.SourceCursor}, {e.HostID, "other", "", e.SourceCursor}, {e.HostID, SSHSource, "", ""}, {e.HostID, SSHSource, "", "bad\ncursor"}, {e.HostID, SSHSource, "", "wrong-cursor"}} {
		if _, err := s.Append(ctx, cp, &e); err == nil {
			t.Fatal("invalid checkpoint accepted")
		}
	}
	bad := e
	bad.EventID = "invalid"
	if _, err := s.Append(ctx, checkpoint("", bad), &bad); err == nil {
		t.Fatal("invalid event accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Append(canceled, checkpoint("", e), &e); err == nil {
		t.Fatal("canceled append succeeded")
	}
	if _, err := s.Cursor(ctx, e.HostID, "other"); err == nil {
		t.Fatal("unknown stream accepted")
	}
}

func TestPendingDuplicateCannotRewindCheckpoint(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	e1, e2 := testEvent(1), testEvent(2)
	if _, err := s.Append(ctx, checkpoint("", e1), &e1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(ctx, checkpoint(e1.SourceCursor, e2), &e2); err != nil {
		t.Fatal(err)
	}
	result, err := s.Append(ctx, checkpoint(e2.SourceCursor, e1), &e1)
	if err != nil || !result.Duplicate {
		t.Fatalf("replay %+v %v", result, err)
	}
	cursor, _ := s.Cursor(ctx, e1.HostID, SSHSource)
	if cursor != e2.SourceCursor {
		t.Fatal("old duplicate rewound checkpoint")
	}
}
