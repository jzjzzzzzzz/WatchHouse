// Package controlstore persists authenticated control-plane events in
// PostgreSQL. Host identity is supplied by the mTLS boundary, never payload.
package controlstore

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

//go:embed migrations/*.sql
var migrations embed.FS

const schemaVersion = 5

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) (*Store, error) {
	if pool == nil {
		return nil, fmt.Errorf("PostgreSQL pool is required")
	}
	return &Store{pool: pool}, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("PostgreSQL pool is required")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin control migration: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(817263540901)`); err != nil {
		return fmt.Errorf("lock control migration: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS control_schema_migrations (version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT statement_timestamp())`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	var current int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version),0) FROM control_schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read control schema version: %w", err)
	}
	if current > schemaVersion {
		return fmt.Errorf("control database schema %d is newer than supported %d", current, schemaVersion)
	}
	for version := current + 1; version <= schemaVersion; version++ {
		name := fmt.Sprintf("migrations/%03d.sql", version)
		body, err := migrations.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read embedded migration %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply control schema %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO control_schema_migrations(version) VALUES($1)`, version); err != nil {
			return fmt.Errorf("record control schema %d: %w", version, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit control migration: %w", err)
	}
	return nil
}

func (store *Store) CommitBatch(ctx context.Context, authenticatedHost string, items []transport.Item) error {
	request := transport.BatchRequest{SchemaVersion: transport.SchemaVersion, Items: items}
	if err := request.Validate(authenticatedHost); err != nil {
		return err
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin event batch: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, item := range items {
		body, digest, err := eventContent(item.Event)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO control_events(host_id,event_id,schema_version,observed_at,agent_received_at,kind,payload,content_digest)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(host_id,event_id) DO NOTHING`,
			authenticatedHost, item.EventID, item.Event.SchemaVersion, item.Event.ObservedAt,
			item.Event.ReceivedAt, item.Event.Kind, body, digest)
		if err != nil {
			return fmt.Errorf("insert authenticated event: %w", err)
		}
		if tag.RowsAffected() == 1 {
			continue
		}
		var storedDigest string
		if err := tx.QueryRow(ctx, `SELECT content_digest FROM control_events WHERE host_id=$1 AND event_id=$2`, authenticatedHost, item.EventID).Scan(&storedDigest); err != nil {
			return fmt.Errorf("read existing authenticated event: %w", err)
		}
		if storedDigest != digest {
			return transport.ErrEventConflict
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit event batch: %w", err)
	}
	return nil
}

func (store *Store) EventCount(ctx context.Context, host string) (int64, error) {
	if !telemetry.ValidHost(host) {
		return 0, fmt.Errorf("invalid host ID")
	}
	var count int64
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_events WHERE host_id=$1`, host).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func eventContent(event telemetry.Event) ([]byte, string, error) {
	if err := event.Validate(); err != nil {
		return nil, "", err
	}
	canonical := event
	canonical.ReceivedAt = canonical.ReceivedAt.UTC()
	body, err := json.Marshal(canonical)
	if err != nil {
		return nil, "", err
	}
	identity := canonical
	// ReceivedAt reflects collection/replay time and is excluded consistently
	// with the agent spool's conflict digest.
	identity.ReceivedAt = time.Time{}
	identityBody, err := json.Marshal(identity)
	if err != nil {
		return nil, "", err
	}
	return body, telemetry.Identity(string(identityBody)), nil
}
