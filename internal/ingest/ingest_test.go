package ingest

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/spool"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../../tests/fixtures/ssh-sequence.journal.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func clock() time.Time { return time.Unix(1775642500, 0) }
func store(t *testing.T, options spool.Options) (*spool.Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "state")
	s, err := spool.Open(context.Background(), dir, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

func TestWholeFilePersistenceAndResume(t *testing.T) {
	s, dir := store(t, spool.DefaultOptions())
	ctx := context.Background()
	stats, err := Run(ctx, bytes.NewReader(fixture(t)), s, "lab-1", WholeJournalFile, clock)
	if err != nil || stats != (Stats{Records: 8, Matched: 7, Unmatched: 1, Inserted: 7, Complete: true}) {
		t.Fatalf("stats %+v %v", stats, err)
	}
	s.Close()
	s2, err := spool.Open(ctx, dir, spool.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	stats, err = Run(ctx, bytes.NewReader(fixture(t)), s2, "lab-1", WholeJournalFile, clock)
	if err != nil || stats.Skipped != 8 || stats.Inserted != 0 || !stats.Complete {
		t.Fatalf("resume %+v %v", stats, err)
	}
	q, _ := s2.Stats(ctx)
	if q.PendingRecords != 7 {
		t.Fatal("restart duplicated queue")
	}
}

func TestMissingCursorFailsWithoutNewRecords(t *testing.T) {
	s, _ := store(t, spool.DefaultOptions())
	ctx := context.Background()
	lines := bytes.Split(bytes.TrimSpace(fixture(t)), []byte("\n"))
	Run(ctx, bytes.NewReader(append(lines[0], '\n')), s, "lab-1", WholeJournalFile, clock)
	stats, err := Run(ctx, bytes.NewReader(bytes.Join(lines[1:], []byte("\n"))), s, "lab-1", WholeJournalFile, clock)
	if !errors.Is(err, ErrSourceGap) || stats.Complete || stats.Inserted != 0 {
		t.Fatalf("gap %+v %v", stats, err)
	}
	q, _ := s.Stats(ctx)
	if q.PendingRecords != 1 {
		t.Fatal("gap advanced source")
	}
}

func TestCapacityStopAndResumeAfterAck(t *testing.T) {
	s, _ := store(t, spool.Options{MaxBytes: 1024 * 1024, MaxRecords: 2})
	ctx := context.Background()
	stats, err := Run(ctx, bytes.NewReader(fixture(t)), s, "lab-1", WholeJournalFile, clock)
	if !errors.Is(err, spool.ErrFull) || stats.Inserted != 2 || stats.Complete {
		t.Fatalf("full %+v %v", stats, err)
	}
	cursor, _ := s.Cursor(ctx, "lab-1", spool.SSHSource)
	if cursor != "s=fixture;i=2" {
		t.Fatal("full advanced past unpersisted record")
	}
	items, _ := s.Peek(ctx, 2, 1024*1024)
	receipts := make([]spool.Receipt, 0)
	for _, item := range items {
		receipts = append(receipts, spool.Receipt{Sequence: item.Sequence, EventID: item.EventID})
	}
	s.Ack(ctx, receipts)
	stats, err = Run(ctx, bytes.NewReader(fixture(t)), s, "lab-1", WholeJournalFile, clock)
	if !errors.Is(err, spool.ErrFull) || stats.Skipped != 2 || stats.Inserted != 2 {
		t.Fatalf("resume after ack %+v %v", stats, err)
	}
}

func TestMalformedRecordPreservesPriorProgress(t *testing.T) {
	s, _ := store(t, spool.DefaultOptions())
	ctx := context.Background()
	first := bytes.Split(fixture(t), []byte("\n"))[0]
	data := append(append([]byte{}, first...), []byte("\n{malformed}\n")...)
	stats, err := Run(ctx, bytes.NewReader(data), s, "lab-1", WholeJournalFile, clock)
	if err == nil || stats.Inserted != 1 || stats.Complete {
		t.Fatalf("partial %+v %v", stats, err)
	}
	cursor, _ := s.Cursor(ctx, "lab-1", spool.SSHSource)
	if cursor != "s=fixture;i=1" {
		t.Fatal("malformed record advanced source")
	}
}

func TestVerifiedContinuationAndValidation(t *testing.T) {
	s, _ := store(t, spool.DefaultOptions())
	ctx := context.Background()
	lines := bytes.Split(bytes.TrimSpace(fixture(t)), []byte("\n"))
	Run(ctx, bytes.NewReader(lines[0]), s, "lab-1", WholeJournalFile, clock)
	stats, err := Run(ctx, bytes.NewReader(bytes.Join(lines[1:], []byte("\n"))), s, "lab-1", VerifiedAfterCheckpoint, clock)
	if err != nil || stats.Inserted != 6 || stats.Skipped != 0 {
		t.Fatalf("verified continuation %+v %v", stats, err)
	}
	for _, mode := range []InputMode{-1, 99} {
		if _, err := Run(ctx, strings.NewReader(""), s, "lab-1", mode, clock); err == nil {
			t.Fatal("unknown mode accepted")
		}
	}
	if _, err := Run(ctx, strings.NewReader(""), nil, "lab-1", WholeJournalFile, clock); err == nil {
		t.Fatal("nil store accepted")
	}
	if _, err := Run(ctx, strings.NewReader(""), s, "lab-1", WholeJournalFile, nil); err == nil {
		t.Fatal("nil clock accepted")
	}
}

func TestVerifiedBatchAnchorMustNotChange(t *testing.T) {
	s, _ := store(t, spool.DefaultOptions())
	ctx := context.Background()
	lines := bytes.Split(bytes.TrimSpace(fixture(t)), []byte("\n"))
	Run(ctx, bytes.NewReader(lines[0]), s, "lab-1", WholeJournalFile, clock)
	stats, err := RunVerifiedAfter(ctx, bytes.NewReader(bytes.Join(lines[1:], []byte("\n"))), s, "lab-1", "stale-poll-anchor", clock)
	if !errors.Is(err, spool.ErrStaleCheckpoint) || stats.Inserted != 0 || stats.Complete {
		t.Fatal("stale native batch persisted")
	}
	stats, err = RunVerifiedAfter(ctx, bytes.NewReader(bytes.Join(lines[1:], []byte("\n"))), s, "lab-1", "s=fixture;i=1", clock)
	if err != nil || stats.Inserted != 6 {
		t.Fatalf("valid anchor rejected: %+v %v", stats, err)
	}
}
