package spool

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"watchhouse/internal/state"
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

func TestVersionOneSpoolMigratesWithoutEventLoss(t *testing.T) {
	ctx := context.Background()
	dir, err := state.Prepare(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	path, _ := dir.File("queue.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE events (sequence INTEGER PRIMARY KEY AUTOINCREMENT,event_id TEXT NOT NULL UNIQUE,host_id TEXT NOT NULL,payload BLOB NOT NULL,payload_bytes INTEGER NOT NULL,content_digest TEXT NOT NULL)`,
		`CREATE TABLE checkpoints (host_id TEXT NOT NULL,source TEXT NOT NULL,cursor TEXT NOT NULL,PRIMARY KEY(host_id,source))`,
		`CREATE TABLE queue_state (id INTEGER PRIMARY KEY CHECK(id=1),pending_bytes INTEGER NOT NULL, pending_records INTEGER NOT NULL,blocked_attempts INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO queue_state VALUES(1,0,0,0)`,
		`PRAGMA user_version=1`, fmt.Sprintf("PRAGMA application_id=%d", ApplicationID),
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	store, err := Open(ctx, dir.Path, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != SchemaVersion {
		t.Fatalf("migrated version %d error %v", version, err)
	}
	if _, err := store.AppendListenerSnapshot(ctx, "host-1", queuedSnapshot()); err != nil {
		t.Fatalf("migrated listener append: %v", err)
	}
}

func TestOpenAndSchemaSettings(t *testing.T) {
	s, dir := openTest(t, DefaultOptions())
	stats, err := s.Stats(context.Background())
	if err != nil || stats.PendingRecords != 0 || stats.PendingBytes != 0 {
		t.Fatalf("stats %+v %v", stats, err)
	}
	for query, want := range map[string]int{"PRAGMA user_version": SchemaVersion, "PRAGMA synchronous": 2, "PRAGMA foreign_keys": 1} {
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
