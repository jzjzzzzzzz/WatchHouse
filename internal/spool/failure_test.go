package spool

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestByteCapacityBackpressure(t *testing.T) {
	s, _ := openTest(t, Options{MaxBytes: 1024, MaxRecords: 100})
	ctx := context.Background()
	previous := ""
	inserted := 0
	for i := 1; i <= 10; i++ {
		e := testEvent(i)
		_, err := s.Append(ctx, checkpoint(previous, e), &e)
		if errors.Is(err, ErrFull) {
			stats, _ := s.Stats(ctx)
			cursor, _ := s.Cursor(ctx, e.HostID, SSHSource)
			if stats.PendingBytes > 1024 || stats.PendingRecords != inserted || cursor != previous {
				t.Fatal("byte capacity violated")
			}
			if inserted < 1 {
				t.Fatal("test did not store initial record")
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		inserted++
		previous = e.SourceCursor
	}
	t.Fatal("byte capacity never enforced")
}

func TestCheckpointCASUnderConcurrency(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for i := 1; i <= 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := testEvent(i)
			_, err := s.Append(ctx, checkpoint("", e), &e)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrStaleCheckpoint) {
			t.Fatalf("unexpected concurrent error %v", err)
		}
	}
	stats, _ := s.Stats(ctx)
	if success != 1 || stats.PendingRecords != 1 {
		t.Fatalf("CAS winners=%d records=%d", success, stats.PendingRecords)
	}
}

func TestAckFailureAfterFirstDeleteRollsBack(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	appendEvents(t, s, 2)
	items, _ := s.Peek(ctx, 2, 1024*1024)
	_, err := s.db.Exec(`CREATE TRIGGER refuse_final_delete BEFORE DELETE ON events
		WHEN (SELECT COUNT(*) FROM events)=1 BEGIN SELECT RAISE(ABORT,'injected after first deletion'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ack(ctx, []Receipt{{items[0].Sequence, items[0].EventID}, {items[1].Sequence, items[1].EventID}}); err == nil {
		t.Fatal("partial ack injection ignored")
	}
	stats, _ := s.Stats(ctx)
	remaining, _ := s.Peek(ctx, 2, 1024*1024)
	if stats.PendingRecords != 2 || len(remaining) != 2 {
		t.Fatal("failed ack lost records")
	}
}

func TestCommittedCapacityCanBeReusedAfterAck(t *testing.T) {
	s, _ := openTest(t, Options{MaxBytes: 1024 * 1024, MaxRecords: 1})
	ctx := context.Background()
	e1, e2 := testEvent(1), testEvent(2)
	s.Append(ctx, checkpoint("", e1), &e1)
	if _, err := s.Append(ctx, checkpoint(e1.SourceCursor, e2), &e2); !errors.Is(err, ErrFull) {
		t.Fatal("expected full queue")
	}
	items, _ := s.Peek(ctx, 1, 1024*1024)
	s.Ack(ctx, []Receipt{{items[0].Sequence, items[0].EventID}})
	if result, err := s.Append(ctx, checkpoint(e1.SourceCursor, e2), &e2); err != nil || !result.Inserted {
		t.Fatal("ack failed to release capacity")
	}
}
