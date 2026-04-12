package spool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func appendEvents(t *testing.T, s *Store, count int) {
	t.Helper()
	previous := ""
	for i := 1; i <= count; i++ {
		e := testEvent(i)
		if _, err := s.Append(context.Background(), checkpoint(previous, e), &e); err != nil {
			t.Fatal(err)
		}
		previous = e.SourceCursor
	}
}

func TestPeekOrderBudgetAndNoDeletion(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	appendEvents(t, s, 3)
	items, err := s.Peek(ctx, 2, 1024*1024)
	if err != nil || len(items) != 2 || items[0].Sequence >= items[1].Sequence {
		t.Fatalf("peek %+v %v", items, err)
	}
	body, _ := json.Marshal(items[0].Event)
	one, err := s.Peek(ctx, 3, int64(len(body)))
	if err != nil || len(one) != 1 {
		t.Fatalf("budget %v", err)
	}
	if _, err := s.Peek(ctx, 1, 1); !errors.Is(err, ErrBatchTooSmall) {
		t.Fatalf("small budget %v", err)
	}
	stats, _ := s.Stats(ctx)
	if stats.PendingRecords != 3 {
		t.Fatal("peek consumed queue")
	}
}

func TestExactAckAndIdempotentReplay(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	appendEvents(t, s, 3)
	items, _ := s.Peek(ctx, 3, 1024*1024)
	r := Receipt{items[1].Sequence, items[1].EventID}
	n, err := s.Ack(ctx, []Receipt{r, r})
	if err != nil || n != 1 {
		t.Fatalf("ack %d %v", n, err)
	}
	n, err = s.Ack(ctx, []Receipt{r})
	if err != nil || n != 0 {
		t.Fatal("repeat ack changed queue")
	}
	remaining, _ := s.Peek(ctx, 3, 1024*1024)
	if len(remaining) != 2 || remaining[0].Sequence != items[0].Sequence || remaining[1].Sequence != items[2].Sequence {
		t.Fatal("ack deleted unconfirmed events")
	}
	stats, _ := s.Stats(ctx)
	b0, _ := json.Marshal(items[0].Event)
	b2, _ := json.Marshal(items[2].Event)
	if stats.PendingBytes != int64(len(b0)+len(b2)) || stats.PendingRecords != 2 {
		t.Fatal("ack accounting incorrect")
	}
}

func TestAckConflictRollsBackWholeBatch(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	appendEvents(t, s, 2)
	items, _ := s.Peek(ctx, 2, 1024*1024)
	bad := strings.Repeat("a", 64)
	if _, err := s.Ack(ctx, []Receipt{{items[0].Sequence, items[0].EventID}, {items[1].Sequence, bad}}); !errors.Is(err, ErrAckConflict) {
		t.Fatalf("want conflict %v", err)
	}
	stats, _ := s.Stats(ctx)
	if stats.PendingRecords != 2 {
		t.Fatal("partial conflicting ack survived")
	}
	if _, err := s.Ack(ctx, []Receipt{{items[0].Sequence, items[0].EventID}, {items[0].Sequence, bad}}); !errors.Is(err, ErrAckConflict) {
		t.Fatal("contradictory duplicate receipts accepted")
	}
}

func TestSequenceNeverReusesAcknowledgedSlot(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	appendEvents(t, s, 1)
	old, _ := s.Peek(ctx, 1, 1024*1024)
	s.Ack(ctx, []Receipt{{old[0].Sequence, old[0].EventID}})
	e := testEvent(2)
	s.Append(ctx, checkpoint(testEvent(1).SourceCursor, e), &e)
	next, _ := s.Peek(ctx, 1, 1024*1024)
	if next[0].Sequence <= old[0].Sequence {
		t.Fatal("sequence ABA risk")
	}
	if n, err := s.Ack(ctx, []Receipt{{old[0].Sequence, old[0].EventID}}); err != nil || n != 0 {
		t.Fatal("old ack touched new queue entry")
	}
}

func TestStoredContentIntegrity(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	appendEvents(t, s, 1)
	if _, err := s.db.Exec("UPDATE events SET content_digest='corrupted'"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Peek(ctx, 1, 1024*1024); !errors.Is(err, ErrCorrupt) {
		t.Fatal("corrupt content accepted")
	}
}

func TestDeliveryArgumentValidation(t *testing.T) {
	s, _ := openTest(t, DefaultOptions())
	ctx := context.Background()
	for _, args := range []struct {
		n int
		b int64
	}{{0, 1024}, {501, 1024}, {1, 0}, {1, 8*1024*1024 + 1}} {
		if _, err := s.Peek(ctx, args.n, args.b); err == nil {
			t.Fatal("invalid peek accepted")
		}
	}
	for _, receipts := range [][]Receipt{{{0, strings.Repeat("a", 64)}}, {{1, "invalid"}}, make([]Receipt, 501)} {
		if _, err := s.Ack(ctx, receipts); err == nil {
			t.Fatal("invalid ack accepted")
		}
	}
	if n, err := s.Ack(ctx, nil); err != nil || n != 0 {
		t.Fatal("empty ack failed")
	}
}
