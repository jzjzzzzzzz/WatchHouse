// Package spool provides private, transactional local telemetry storage.
package spool

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
	"watchhouse/internal/state"
)

const SchemaVersion = 3
const ApplicationID = 0x57485345 // WHSE; distinguish this spool from arbitrary SQLite files.

type Options struct {
	MaxBytes   int64
	MaxRecords int
}

func DefaultOptions() Options { return Options{MaxBytes: 100 * 1024 * 1024, MaxRecords: 100_000} }

type Store struct {
	db      *sql.DB
	mu      sync.Mutex
	options Options
}

func Open(ctx context.Context, dir string, options Options) (*Store, error) {
	if options.MaxBytes < 1024 || options.MaxBytes > 1024*1024*1024 || options.MaxRecords < 1 || options.MaxRecords > 1_000_000 {
		return nil, fmt.Errorf("invalid spool byte or record capacity")
	}
	directory, err := state.Prepare(dir)
	if err != nil {
		return nil, err
	}
	path, err := directory.File("queue.db")
	if err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection preserves PRAGMA scope and serializes transactions inside
	// this process. SQLite still arbitrates other processes via busy_timeout.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db, options: options}
	if err := s.initialize(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) initialize(ctx context.Context) error {
	var version, application int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if err := s.db.QueryRowContext(ctx, "PRAGMA application_id").Scan(&application); err != nil {
		return err
	}
	if version < 0 || version > SchemaVersion {
		return fmt.Errorf("unsupported spool schema %d", version)
	}
	if version > 0 && application != ApplicationID {
		return fmt.Errorf("database is not a branded Watchhouse spool")
	}
	if version == 0 {
		var tables int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
			return err
		}
		if application != 0 || tables != 0 {
			return fmt.Errorf("refusing to initialize a nonempty or foreign database")
		}
	}
	for _, pragma := range []string{"PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA foreign_keys=ON"} {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("configure spool: %w", err)
		}
	}
	if version == 1 {
		if err := s.migrateV2(ctx); err != nil {
			return err
		}
		version = 2
	}
	if version == 2 {
		if err := s.migrateV3(ctx); err != nil {
			return err
		}
		version = 3
	}
	if version == SchemaVersion {
		return s.verifyAccounting(ctx)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE IF NOT EXISTS events (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id TEXT NOT NULL UNIQUE,
			host_id TEXT NOT NULL,
			payload BLOB NOT NULL,
			payload_bytes INTEGER NOT NULL CHECK(payload_bytes > 0 AND payload_bytes = length(payload)),
			content_digest TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS checkpoints (
			host_id TEXT NOT NULL, source TEXT NOT NULL, cursor TEXT NOT NULL,
			PRIMARY KEY(host_id, source)
		)`,
		`CREATE TABLE IF NOT EXISTS queue_state (
			id INTEGER PRIMARY KEY CHECK(id=1),
			pending_bytes INTEGER NOT NULL CHECK(pending_bytes>=0),
			pending_records INTEGER NOT NULL CHECK(pending_records>=0),
			blocked_attempts INTEGER NOT NULL DEFAULT 0 CHECK(blocked_attempts>=0),
			listener_pending_bytes INTEGER NOT NULL DEFAULT 0 CHECK(listener_pending_bytes>=0),
			listener_pending_records INTEGER NOT NULL DEFAULT 0 CHECK(listener_pending_records>=0),
			probe_pending_bytes INTEGER NOT NULL DEFAULT 0 CHECK(probe_pending_bytes>=0),
			probe_pending_records INTEGER NOT NULL DEFAULT 0 CHECK(probe_pending_records>=0)
		)`,
		`CREATE TABLE IF NOT EXISTS listener_snapshots (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id TEXT NOT NULL UNIQUE,
			host_id TEXT NOT NULL,
			payload BLOB NOT NULL,
			payload_bytes INTEGER NOT NULL CHECK(payload_bytes > 0 AND payload_bytes = length(payload)),
			content_digest TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS probe_observations (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			observation_id TEXT NOT NULL UNIQUE,
			probe_id TEXT NOT NULL,
			payload BLOB NOT NULL,
			payload_bytes INTEGER NOT NULL CHECK(payload_bytes > 0 AND payload_bytes = length(payload)),
			content_digest TEXT NOT NULL
		)`,
		`INSERT OR IGNORE INTO queue_state(id, pending_bytes, pending_records) VALUES(1,0,0)`,
		`PRAGMA user_version=3`,
		fmt.Sprintf("PRAGMA application_id=%d", ApplicationID),
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize spool: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) migrateV3(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`ALTER TABLE queue_state ADD COLUMN probe_pending_bytes INTEGER NOT NULL DEFAULT 0 CHECK(probe_pending_bytes>=0)`,
		`ALTER TABLE queue_state ADD COLUMN probe_pending_records INTEGER NOT NULL DEFAULT 0 CHECK(probe_pending_records>=0)`,
		`CREATE TABLE probe_observations (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			observation_id TEXT NOT NULL UNIQUE,
			probe_id TEXT NOT NULL,
			payload BLOB NOT NULL,
			payload_bytes INTEGER NOT NULL CHECK(payload_bytes > 0 AND payload_bytes = length(payload)),
			content_digest TEXT NOT NULL
		)`,
		`PRAGMA user_version=3`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate spool schema v3: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) migrateV2(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`ALTER TABLE queue_state ADD COLUMN listener_pending_bytes INTEGER NOT NULL DEFAULT 0 CHECK(listener_pending_bytes>=0)`,
		`ALTER TABLE queue_state ADD COLUMN listener_pending_records INTEGER NOT NULL DEFAULT 0 CHECK(listener_pending_records>=0)`,
		`CREATE TABLE listener_snapshots (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id TEXT NOT NULL UNIQUE,
			host_id TEXT NOT NULL,
			payload BLOB NOT NULL,
			payload_bytes INTEGER NOT NULL CHECK(payload_bytes > 0 AND payload_bytes = length(payload)),
			content_digest TEXT NOT NULL
		)`,
		`PRAGMA user_version=2`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate spool schema v2: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) verifyAccounting(ctx context.Context) error {
	var records, actualRecords, listenerRecords, actualListenerRecords, probeRecords, actualProbeRecords int
	var bytes, actualBytes, listenerBytes, actualListenerBytes, probeBytes, actualProbeBytes, blocked int64
	err := s.db.QueryRowContext(ctx, `SELECT pending_records,pending_bytes,blocked_attempts,listener_pending_records,listener_pending_bytes,probe_pending_records,probe_pending_bytes,
		(SELECT COUNT(*) FROM events),(SELECT COALESCE(SUM(payload_bytes),0) FROM events),
		(SELECT COUNT(*) FROM listener_snapshots),(SELECT COALESCE(SUM(payload_bytes),0) FROM listener_snapshots),
		(SELECT COUNT(*) FROM probe_observations),(SELECT COALESCE(SUM(payload_bytes),0) FROM probe_observations)
		FROM queue_state WHERE id=1`).Scan(&records, &bytes, &blocked, &listenerRecords, &listenerBytes, &probeRecords, &probeBytes,
		&actualRecords, &actualBytes, &actualListenerRecords, &actualListenerBytes, &actualProbeRecords, &actualProbeBytes)
	if err != nil {
		return fmt.Errorf("spool schema/accounting unavailable: %w", err)
	}
	if records != actualRecords || bytes != actualBytes || listenerRecords != actualListenerRecords || listenerBytes != actualListenerBytes ||
		probeRecords != actualProbeRecords || probeBytes != actualProbeBytes || records < 0 || bytes < 0 || listenerRecords < 0 || listenerBytes < 0 ||
		probeRecords < 0 || probeBytes < 0 || blocked < 0 {
		return ErrCorrupt
	}
	rows, err := s.db.QueryContext(ctx, "SELECT host_id,source,cursor FROM checkpoints LIMIT 0")
	if err != nil {
		return fmt.Errorf("checkpoint schema unavailable: %w", err)
	}
	return rows.Close()
}

type Stats struct {
	PendingBytes           int64 `json:"pending_bytes"`
	PendingRecords         int   `json:"pending_records"`
	BlockedAttempts        int64 `json:"blocked_attempts"`
	MaxBytes               int64 `json:"max_bytes"`
	MaxRecords             int   `json:"max_records"`
	ListenerPendingBytes   int64 `json:"listener_pending_bytes"`
	ListenerPendingRecords int   `json:"listener_pending_records"`
	ProbePendingBytes      int64 `json:"probe_pending_bytes"`
	ProbePendingRecords    int   `json:"probe_pending_records"`
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	stats := Stats{MaxBytes: s.options.MaxBytes, MaxRecords: s.options.MaxRecords}
	err := s.db.QueryRowContext(ctx, "SELECT pending_bytes,pending_records,blocked_attempts,listener_pending_bytes,listener_pending_records,probe_pending_bytes,probe_pending_records FROM queue_state WHERE id=1").Scan(&stats.PendingBytes, &stats.PendingRecords, &stats.BlockedAttempts, &stats.ListenerPendingBytes, &stats.ListenerPendingRecords, &stats.ProbePendingBytes, &stats.ProbePendingRecords)
	return stats, err
}
