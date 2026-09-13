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

	"watchhouse/internal/certaudit"
	"watchhouse/internal/detection"
	"watchhouse/internal/diskaudit"
	"watchhouse/internal/dockerports"
	"watchhouse/internal/fileaudit"
	"watchhouse/internal/firewall"
	"watchhouse/internal/hostview"
	"watchhouse/internal/journal"
	"watchhouse/internal/pkginventory"
	"watchhouse/internal/posture"
	"watchhouse/internal/replay"
	"watchhouse/internal/runtimeinfo"
	"watchhouse/internal/sshaudit"
	"watchhouse/internal/telemetry"
	"watchhouse/internal/unitaudit"
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
		fmt.Fprintln(errOut, "       watchhouse firewall (Linux; lossy bounded nftables ruleset summary)")
		fmt.Fprintln(errOut, "       watchhouse posture (Linux; evaluate fixed runtime sysctl profile)")
		fmt.Fprintln(errOut, "       watchhouse audit-files (Linux; inspect fixed privileged-file modes)")
		fmt.Fprintln(errOut, "       watchhouse audit-ssh (Linux; evaluate sshd effective configuration)")
		fmt.Fprintln(errOut, "       watchhouse audit-units (Linux; inspect Watchhouse systemd sandboxes)")
		fmt.Fprintln(errOut, "       watchhouse audit-disk (Linux; check root and state capacity)")
		fmt.Fprintln(errOut, "       watchhouse audit-cert --cert ABSOLUTE_PEM [--minimum-remaining 720h]")
		fmt.Fprintln(errOut, "       watchhouse evidence-bundle create --input DIR --output FILE")
		fmt.Fprintln(errOut, "       watchhouse evidence-bundle verify --archive FILE")
		fmt.Fprintln(errOut, "       watchhouse packages (Linux/dpkg; bounded installed-package inventory)")
		fmt.Fprintln(errOut, "       watchhouse docker-ports (Linux; inspect running-container host bindings)")
		fmt.Fprintln(errOut, "       watchhouse exposure (Linux; correlate listeners and Docker bindings)")
		fmt.Fprintln(errOut, "       watchhouse listener-check --policy FILE (Linux; evaluate declared endpoints)")
		fmt.Fprintln(errOut, "       watchhouse deliver --state DIR --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse query-events --host HOST --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse query-findings --host HOST --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse report-listeners --host HOST --state DIR --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse deliver-listeners --state DIR --endpoint HTTPS_ORIGIN --ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse probe-https --url HTTPS_URL [--expect-status 200] [--ca FILE]")
		fmt.Fprintln(errOut, "       watchhouse report-probe --probe-id ID --state DIR --url HTTPS_URL --control-endpoint HTTPS_ORIGIN --control-ca FILE --cert FILE --key FILE --server-name NAME")
		fmt.Fprintln(errOut, "       watchhouse deliver-probes --state DIR --control-endpoint HTTPS_ORIGIN --control-ca FILE --cert FILE --key FILE --server-name NAME")
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
	if args[0] == "firewall" {
		return runFirewall(args[1:], out, errOut, firewall.Collect)
	}
	if args[0] == "posture" {
		return runPosture(args[1:], out, errOut, posture.CollectSysctls)
	}
	if args[0] == "audit-files" {
		return runFileAudit(args[1:], out, errOut, fileaudit.Collect)
	}
	if args[0] == "audit-ssh" {
		return runSSHAudit(args[1:], out, errOut, sshaudit.Collect)
	}
	if args[0] == "audit-units" {
		return runUnitAudit(args[1:], out, errOut, unitaudit.Collect)
	}
	if args[0] == "audit-disk" {
		return runDiskAudit(args[1:], out, errOut, diskaudit.Collect)
	}
	if args[0] == "audit-cert" {
		return runCertAudit(args[1:], out, errOut, certaudit.AuditFile)
	}
	if args[0] == "evidence-bundle" {
		return runEvidenceBundle(args[1:], out, errOut)
	}
	if args[0] == "packages" {
		return runPackages(args[1:], out, errOut, pkginventory.Collect)
	}
	if args[0] == "docker-ports" {
		return runDockerPorts(args[1:], out, errOut, dockerports.Collect)
	}
	if args[0] == "exposure" {
		return runExposure(args[1:], out, errOut, hostview.Snapshot, dockerports.Collect)
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
	if args[0] == "deliver-probes" {
		return runDeliverProbes(args[1:], out, errOut)
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
