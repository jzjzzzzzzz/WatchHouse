// Package ingest advances source progress only after local durable processing.
package ingest

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/spool"
	"watchhouse/internal/telemetry"
)

var ErrSourceGap = errors.New("persisted cursor not found in source input; resume requires investigation")

type InputMode int

const (
	WholeJournalFile InputMode = iota
	VerifiedAfterCheckpoint
)

type Stats struct {
	Records    int  `json:"records"`
	Skipped    int  `json:"skipped"`
	Matched    int  `json:"matched"`
	Unmatched  int  `json:"unmatched"`
	Inserted   int  `json:"inserted"`
	Duplicates int  `json:"duplicates"`
	Complete   bool `json:"complete"`
}

// WholeJournalFile seeks the persisted cursor before ingesting any new rows.
// VerifiedAfterCheckpoint is only for a collector that verified native resume;
// a caller must not use it on an arbitrary tail or claim gaps were checked.
func Run(ctx context.Context, in io.Reader, store *spool.Store, host string, mode InputMode, clock func() time.Time) (Stats, error) {
	var stats Stats
	if store == nil || !telemetry.ValidHost(host) || clock == nil || (mode != WholeJournalFile && mode != VerifiedAfterCheckpoint) {
		return stats, fmt.Errorf("invalid ingest arguments")
	}
	cursor, err := store.Cursor(ctx, host, spool.SSHSource)
	if err != nil {
		return stats, err
	}
	return process(ctx, in, store, host, mode, cursor, clock)
}

// RunVerifiedAfter binds the input batch to the cursor used by native Poll.
// It prevents a concurrent collector from changing the anchor between capture
// and persistence; the per-record CAS protects subsequent changes as well.
func RunVerifiedAfter(ctx context.Context, in io.Reader, store *spool.Store, host, expectedCursor string, clock func() time.Time) (Stats, error) {
	var stats Stats
	if store == nil || !telemetry.ValidHost(host) || clock == nil {
		return stats, fmt.Errorf("invalid ingest arguments")
	}
	cursor, err := store.Cursor(ctx, host, spool.SSHSource)
	if err != nil {
		return stats, err
	}
	if cursor != expectedCursor {
		return stats, spool.ErrStaleCheckpoint
	}
	return process(ctx, in, store, host, VerifiedAfterCheckpoint, cursor, clock)
}

func process(ctx context.Context, in io.Reader, store *spool.Store, host string, mode InputMode, cursor string, clock func() time.Time) (Stats, error) {
	var stats Stats
	seeking := mode == WholeJournalFile && cursor != ""
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), telemetry.MaxRecordBytes+1)
	for scanner.Scan() {
		stats.Records++
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		position, err := telemetry.JournalPosition(scanner.Bytes())
		if err != nil {
			return stats, fmt.Errorf("record %d source metadata: %w", stats.Records, err)
		}
		if seeking {
			stats.Skipped++
			if position.Cursor == cursor {
				seeking = false
			}
			continue
		}
		event, matched, err := telemetry.ParseJournal(scanner.Bytes(), host, clock())
		if err != nil {
			return stats, fmt.Errorf("record %d normalize: %w", stats.Records, err)
		}
		var pending *telemetry.Event
		if matched {
			pending = &event
			stats.Matched++
		} else {
			stats.Unmatched++
		}
		result, err := store.Append(ctx, spool.Checkpoint{HostID: host, Source: spool.SSHSource, ExpectedCursor: cursor, NextCursor: position.Cursor}, pending)
		if err != nil {
			return stats, fmt.Errorf("record %d persist: %w", stats.Records, err)
		}
		if result.Inserted {
			stats.Inserted++
		}
		if result.Duplicate {
			stats.Duplicates++
		} else {
			cursor = position.Cursor
		}
	}
	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("source input: %w", err)
	}
	if seeking {
		return stats, ErrSourceGap
	}
	stats.Complete = true
	return stats, nil
}
