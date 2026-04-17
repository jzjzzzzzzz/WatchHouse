package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"watchhouse/internal/ingest"
	"watchhouse/internal/spool"
	"watchhouse/internal/telemetry"
)

func runSpool(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: watchhouse spool init|status|peek|ingest|check --state DIR")
		return 2
	}
	action := args[0]
	if action != "init" && action != "status" && action != "peek" && action != "ingest" && action != "check" {
		fmt.Fprintln(errOut, "unknown spool action")
		return 2
	}
	flags := flag.NewFlagSet("spool "+action, flag.ContinueOnError)
	flags.SetOutput(errOut)
	dir := flags.String("state", "", "private state directory")
	defaults := spool.DefaultOptions()
	maxBytes := flags.Int64("max-bytes", defaults.MaxBytes, "pending payload byte limit, not total disk limit")
	maxRecords := flags.Int("max-records", defaults.MaxRecords, "pending event record limit")
	host, input, limit, budget := "", "-", 100, int64(1024*1024)
	if action == "ingest" {
		flags.StringVar(&host, "host", "", "offline source label")
		flags.StringVar(&input, "input", "-", "complete journal JSONL file; - reads stdin")
	}
	if action == "peek" {
		flags.IntVar(&limit, "limit", 100, "up to 500 pending events")
		flags.Int64Var(&budget, "batch-bytes", budget, "up to 8 MiB pending payload")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *dir == "" {
		fmt.Fprintln(errOut, "--state is required; no positional arguments accepted")
		return 2
	}
	if action == "ingest" && !telemetry.ValidHost(host) {
		fmt.Fprintln(errOut, "valid --host required")
		return 2
	}
	if action == "peek" && (limit < 1 || limit > 500 || budget < 1 || budget > 8*1024*1024) {
		fmt.Fprintln(errOut, "invalid peek budget")
		return 2
	}
	if action == "status" || action == "peek" || action == "check" {
		if _, err := os.Stat(filepath.Join(*dir, "queue.db")); err != nil {
			fmt.Fprintln(errOut, "existing spool required:", err)
			return 1
		}
	}
	if action == "ingest" && input != "-" {
		file, err := os.Open(input)
		if err != nil {
			fmt.Fprintln(errOut, "input:", err)
			return 1
		}
		defer file.Close()
		in = file
	}
	ctx := context.Background()
	store, err := spool.Open(ctx, *dir, spool.Options{MaxBytes: *maxBytes, MaxRecords: *maxRecords})
	if err != nil {
		fmt.Fprintln(errOut, "spool:", err)
		return 1
	}
	defer store.Close()
	encoder := json.NewEncoder(out)
	switch action {
	case "init", "status":
		stats, err := store.Stats(ctx)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if err := encoder.Encode(struct {
			Type  string      `json:"type"`
			Stats spool.Stats `json:"stats"`
		}{"spool_status", stats}); err != nil {
			return 1
		}
	case "peek":
		items, err := store.Peek(ctx, limit, budget)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		for _, item := range items {
			if err := encoder.Encode(item); err != nil {
				return 1
			}
		}
	case "ingest":
		stats, err := ingest.Run(ctx, in, store, host, ingest.WholeJournalFile, time.Now)
		if encodeErr := encoder.Encode(struct {
			Type  string       `json:"type"`
			Stats ingest.Stats `json:"stats"`
		}{"ingest_summary", stats}); encodeErr != nil {
			return 1
		}
		if err != nil {
			fmt.Fprintln(errOut, "ingest:", err)
			return 1
		}
	case "check":
		result, err := store.Audit(ctx)
		if encodeErr := encoder.Encode(struct {
			Type   string            `json:"type"`
			Result spool.AuditResult `json:"result"`
		}{"spool_audit", result}); encodeErr != nil {
			return 1
		}
		if err != nil {
			fmt.Fprintln(errOut, "audit:", err)
			return 1
		}
	}
	return 0
}
