package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"watchhouse/internal/detection"
	"watchhouse/internal/hostview"
	"watchhouse/internal/journal"
	"watchhouse/internal/replay"
	"watchhouse/internal/runtimeinfo"
	"watchhouse/internal/telemetry"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(errOut, "Usage: watchhouse replay --host HOST [--input FILE|-] [--threshold 5] [--window 5m] [--max-events 8192]")
		fmt.Fprintln(errOut, "       watchhouse snapshot --host HOST [--limit 200]  (Linux; read-only bounded journal capture)")
		fmt.Fprintln(errOut, "       watchhouse spool init|status|peek|ingest|check --state DIR [options]")
		fmt.Fprintln(errOut, "       watchhouse collect --host HOST --state DIR [--limit 200] (Linux verified forward capture)")
		fmt.Fprintln(errOut, "       watchhouse self (inspect own runtime privileges)")
		fmt.Fprintln(errOut, "       watchhouse listeners (Linux; bounded current-network-namespace snapshot)")
		fmt.Fprintln(errOut, "       watchhouse listener-check --policy FILE (Linux; evaluate declared endpoints)")
		fmt.Fprintln(errOut, "       watchhouse deliver --state DIR --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse query-events --host HOST --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse query-findings --host HOST --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse report-listeners --host HOST --state DIR --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse deliver-listeners --state DIR --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse probe-https --url HTTPS_URL [--expect-status 200] [--ca FILE]")
		fmt.Fprintln(errOut, "       watchhouse report-probe --probe-id ID --state DIR --url HTTPS_URL --control-endpoint HTTPS_ORIGIN --control-ca FILE --cert FILE --key FILE --server-name NAME")
		return 0
	}
	if args[0] == "spool" {
		return runSpool(args[1:], in, out, errOut)
	}
	if args[0] == "collect" {
		return runCollect(args[1:], out, errOut, journal.Poll)
	}
	if args[0] == "self" {
		if len(args) != 1 {
			fmt.Fprintln(errOut, "self accepts no arbitrary paths or process IDs")
			return 2
		}
		result, err := runtimeinfo.Self()
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if err := json.NewEncoder(out).Encode(result); err != nil {
			return 1
		}
		return 0
	}
	if args[0] == "listeners" {
		return runListeners(args[1:], out, errOut, hostview.Snapshot)
	}
	if args[0] == "listener-check" {
		return runListenerCheck(args[1:], out, errOut, hostview.Snapshot)
	}
	if args[0] == "deliver" {
		return runDeliver(args[1:], out, errOut)
	}
	if args[0] == "query-events" {
		return runQueryEvents(args[1:], out, errOut)
	}
	if args[0] == "query-findings" {
		return runQueryFindings(args[1:], out, errOut)
	}
	if args[0] == "report-listeners" {
		return runReportListeners(args[1:], out, errOut, hostview.Snapshot)
	}
	if args[0] == "deliver-listeners" {
		return runDeliverListeners(args[1:], out, errOut)
	}
	if args[0] == "probe-https" {
		return runProbeHTTPS(args[1:], out, errOut)
	}
	if args[0] == "report-probe" {
		return runReportProbe(args[1:], out, errOut)
	}
	if args[0] != "replay" && args[0] != "snapshot" {
		fmt.Fprintln(errOut, "unknown command; use --help")
		return 2
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(errOut)
	host := flags.String("host", "", "offline host label; not an authenticated identity")
	input := "-"
	limit := 200
	if args[0] == "replay" {
		flags.StringVar(&input, "input", "-", "journal JSON Lines file; - reads stdin")
	} else {
		flags.IntVar(&limit, "limit", 200, "latest matching records, 1..1000; not a persistent subscription")
	}
	defaults := detection.DefaultConfig()
	threshold := flags.Int("threshold", defaults.Threshold, "failure count before success")
	window := flags.Duration("window", defaults.Window, "ordered event-time observation window")
	maxEvents := flags.Int("max-events", defaults.MaxEvents, "maximum in-memory dedup events")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "unexpected positional arguments")
		return 2
	}
	if !telemetry.ValidHost(*host) {
		fmt.Fprintln(errOut, "valid --host is required")
		return 2
	}
	if _, err := detection.NewSSH(detection.Config{Window: *window, Threshold: *threshold, MaxEvents: *maxEvents}); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if args[0] == "snapshot" {
		capture, err := journal.Read(context.Background(), limit)
		if err != nil {
			fmt.Fprintln(errOut, "snapshot:", err)
			return 1
		}
		in = bytes.NewReader(capture.Data)
	} else if input != "-" {
		file, err := os.Open(input)
		if err != nil {
			fmt.Fprintln(errOut, "input:", err)
			return 1
		}
		defer file.Close()
		in = file
	}
	stats, err := replay.Run(in, out, *host, detection.Config{Window: *window, Threshold: *threshold, MaxEvents: *maxEvents}, time.Now)
	if encodeErr := json.NewEncoder(errOut).Encode(struct {
		Type  string       `json:"type"`
		Stats replay.Stats `json:"stats"`
	}{"summary", stats}); encodeErr != nil {
		return 1
	}
	if err != nil {
		fmt.Fprintln(errOut, "replay:", err)
		return 1
	}
	return 0
}
