package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"watchhouse/internal/journal"
	"watchhouse/internal/spool"
)

func TestCollectResumesPersistedNativeCursor(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	fixture, err := os.ReadFile("../../tests/fixtures/ssh-sequence.journal.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--host", "lab-1", "--state", dir}
	var out, errOut bytes.Buffer
	code := runCollect(args, &out, &errOut, func(_ context.Context, cursor string, limit int) (journal.Capture, error) {
		if cursor != "" || limit != 200 {
			t.Fatal("initial cursor or limit wrong")
		}
		return journal.Capture{Data: fixture}, nil
	})
	if code != 0 || !strings.Contains(out.String(), `"inserted":7`) {
		t.Fatalf("collect %d %s %s", code, &out, &errOut)
	}
	out.Reset()
	errOut.Reset()
	code = runCollect(args, &out, &errOut, func(_ context.Context, cursor string, _ int) (journal.Capture, error) {
		if cursor != "s=fixture;i=8" {
			t.Fatalf("resume did not use persisted checkpoint: %q", cursor)
		}
		return journal.Capture{}, nil
	})
	if code != 0 || !strings.Contains(out.String(), `"records":0`) {
		t.Fatal("empty native continuation failed")
	}
}

func TestNativeGapPreservesDurableState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	var out, errOut bytes.Buffer
	code := runCollect([]string{"--host", "lab-1", "--state", dir}, &out, &errOut, func(context.Context, string, int) (journal.Capture, error) {
		return journal.Capture{}, journal.ErrCursorGap
	})
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "cursor") {
		t.Fatal("native gap claimed success")
	}
	s, err := spool.Open(context.Background(), dir, spool.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cursor, _ := s.Cursor(context.Background(), "lab-1", spool.SSHSource)
	if cursor != "" {
		t.Fatal("gap advanced checkpoint")
	}
}

func TestCollectArgumentsDoNotExecuteReader(t *testing.T) {
	for _, args := range [][]string{nil, {"--host", "bad host", "--state", "ignored"}, {"--host", "lab-1", "--state", "ignored", "--limit", "1001"}, {"--host", "lab-1", "--state", "ignored", "extra"}} {
		var out, errOut bytes.Buffer
		code := runCollect(args, &out, &errOut, func(context.Context, string, int) (journal.Capture, error) {
			t.Fatal("invalid args executed reader")
			return journal.Capture{}, errors.New("not reached")
		})
		if code != 2 {
			t.Errorf("invalid collect args returned %d", code)
		}
	}
}
