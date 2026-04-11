package spool

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T, options Options) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "state")
	s, err := Open(context.Background(), dir, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

func TestOpenAndSchemaSettings(t *testing.T) {
	s, dir := openTest(t, DefaultOptions())
	stats, err := s.Stats(context.Background())
	if err != nil || stats.PendingRecords != 0 || stats.PendingBytes != 0 {
		t.Fatalf("stats %+v %v", stats, err)
	}
	for query, want := range map[string]int{"PRAGMA user_version": 1, "PRAGMA synchronous": 2, "PRAGMA foreign_keys": 1} {
		var value int
		if err := s.db.QueryRow(query).Scan(&value); err != nil || value != want {
			t.Fatalf("%s = %d err %v", query, value, err)
		}
	}
	var journal string
	s.db.QueryRow("PRAGMA journal_mode").Scan(&journal)
	if journal != "wal" {
		t.Fatal("WAL not enabled")
	}
	info, _ := os.Stat(filepath.Join(dir, "queue.db"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("database not private")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(context.Background(), dir, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if _, err := s2.Stats(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRejectFutureSchemaWithoutReset(t *testing.T) {
	s, dir := openTest(t, DefaultOptions())
	if _, err := s.db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := Open(context.Background(), dir, DefaultOptions()); err == nil {
		t.Fatal("future schema silently reset")
	}
}

func TestInvalidCapacityAndCanceledOpen(t *testing.T) {
	for _, options := range []Options{{}, {1023, 1}, {1024, 0}, {1024, 1_000_001}, {1024*1024*1024 + 1, 1}} {
		if _, err := Open(context.Background(), filepath.Join(t.TempDir(), "state"), options); err == nil {
			t.Errorf("invalid options %+v", options)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Open(ctx, filepath.Join(t.TempDir(), "state"), DefaultOptions()); err == nil {
		t.Fatal("canceled open succeeded")
	}
}
