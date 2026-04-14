package spool

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"watchhouse/internal/state"
)

func TestForeignDatabaseNotModified(t *testing.T) {
	dir, err := state.Prepare(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := dir.File("queue.db")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE unrelated(secret TEXT)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	before, _ := os.ReadFile(path)
	if _, err := Open(context.Background(), dir.Path, DefaultOptions()); err == nil {
		t.Fatal("foreign database adopted")
	}
	after, _ := os.ReadFile(path)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("foreign database modified before rejection")
	}
}

func TestBrandAndAccountingSurviveReopen(t *testing.T) {
	s, dir := openTest(t, DefaultOptions())
	appendEvents(t, s, 2)
	var brand int
	s.db.QueryRow("PRAGMA application_id").Scan(&brand)
	if brand != ApplicationID {
		t.Fatal("missing Watchhouse application ID")
	}
	s.db.Exec("UPDATE queue_state SET pending_records=99 WHERE id=1")
	s.Close()
	if _, err := Open(context.Background(), dir, DefaultOptions()); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("corrupt counters adopted: %v", err)
	}
}

func TestMissingSchemaNotSilentlyRecreated(t *testing.T) {
	s, dir := openTest(t, DefaultOptions())
	s.db.Exec("DROP TABLE checkpoints")
	s.Close()
	if _, err := Open(context.Background(), dir, DefaultOptions()); err == nil {
		t.Fatal("missing table silently recreated")
	}
}
