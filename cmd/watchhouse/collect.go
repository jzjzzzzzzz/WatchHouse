package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/ingest"
	"watchhouse/internal/journal"
	"watchhouse/internal/spool"
	"watchhouse/internal/telemetry"
)

type nativePoll func(context.Context, string, int) (journal.Capture, error)

func runCollect(args []string, out, errOut io.Writer, poll nativePoll) int {
	flags := flag.NewFlagSet("collect", flag.ContinueOnError)
	flags.SetOutput(errOut)
	host := flags.String("host", "", "local source label; network identity added later")
	dir := flags.String("state", "", "private persistent state directory")
	limit := flags.Int("limit", 200, "first new records after verified cursor, 1..1000")
	defaults := spool.DefaultOptions()
	maxBytes := flags.Int64("max-bytes", defaults.MaxBytes, "logical payload capacity")
	maxRecords := flags.Int("max-records", defaults.MaxRecords, "pending event capacity")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || !telemetry.ValidHost(*host) || *dir == "" || *limit < 1 || *limit > 1000 {
		fmt.Fprintln(errOut, "valid --host, --state and --limit 1..1000 required")
		return 2
	}
	ctx := context.Background()
	store, err := spool.Open(ctx, *dir, spool.Options{MaxBytes: *maxBytes, MaxRecords: *maxRecords})
	if err != nil {
		fmt.Fprintln(errOut, "spool:", err)
		return 1
	}
	defer store.Close()
	cursor, err := store.Cursor(ctx, *host, spool.SSHSource)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	capture, err := poll(ctx, cursor, *limit)
	if err != nil {
		fmt.Fprintln(errOut, "native collect:", err)
		return 1
	}
	stats, err := ingest.RunVerifiedAfter(ctx, bytes.NewReader(capture.Data), store, *host, cursor, time.Now)
	if encodeErr := json.NewEncoder(out).Encode(struct {
		Type  string       `json:"type"`
		Stats ingest.Stats `json:"stats"`
	}{"collect_summary", stats}); encodeErr != nil {
		return 1
	}
	if err != nil {
		fmt.Fprintln(errOut, "persist collect:", err)
		return 1
	}
	return 0
}
