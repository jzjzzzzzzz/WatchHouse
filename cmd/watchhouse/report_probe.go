package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/delivery"
	"watchhouse/internal/extprobe"
	"watchhouse/internal/spool"
	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

func runReportProbe(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("report-probe", flag.ContinueOnError)
	flags.SetOutput(errOut)
	probeID := flags.String("probe-id", "", "identity bound to the probe certificate")
	state := flags.String("state", "", "private durable spool directory")
	target := flags.String("url", "", "exact HTTPS health URL")
	expectedStatus := flags.Int("expect-status", 200, "expected HTTP status")
	maxBody := flags.Int64("max-body", 64*1024, "response body hash bound")
	timeout := flags.Duration("timeout", 15*time.Second, "probe and publish timeout")
	targetCA := flags.String("target-ca", "", "optional target CA bundle")
	allowPrivate := flags.Bool("allow-private", false, "explicitly permit private targets for a lab")
	controlEndpoint := flags.String("control-endpoint", "", "HTTPS control origin")
	controlCA := flags.String("control-ca", "", "control CA bundle")
	certificate := flags.String("cert", "", "probe certificate")
	key := flags.String("key", "", "probe private key")
	serverName := flags.String("server-name", "", "verified control certificate name")
	batchBytes := flags.Int64("batch-bytes", 1024*1024, "probe payload budget, up to 8 MiB")
	defaults := spool.DefaultOptions()
	maxBytes := flags.Int64("max-bytes", defaults.MaxBytes, "probe outbox logical payload limit")
	maxRecords := flags.Int("max-records", defaults.MaxRecords, "probe outbox record limit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || !telemetry.ValidHost(*probeID) || *state == "" || *target == "" || *controlEndpoint == "" || *controlCA == "" || *certificate == "" || *key == "" || *serverName == "" || *batchBytes < 1 || *batchBytes > 8*1024*1024 {
		fmt.Fprintln(errOut, "report-probe requires probe identity, state, target, control endpoint/CA, probe certificate/key, server name, and valid bounds")
		return 2
	}
	roots, err := extprobe.LoadRoots(*targetCA)
	if err != nil {
		fmt.Fprintln(errOut, "report-probe target roots:", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	result, err := extprobe.Run(ctx, extprobe.Config{URL: *target, ExpectedStatus: *expectedStatus, MaxBodyBytes: *maxBody,
		AllowPrivate: *allowPrivate, Roots: roots, Timeout: *timeout})
	if err != nil {
		fmt.Fprintln(errOut, "report-probe observation:", err)
		return 1
	}
	store, err := spool.Open(ctx, *state, spool.Options{MaxBytes: *maxBytes, MaxRecords: *maxRecords})
	if err != nil {
		fmt.Fprintln(errOut, "report-probe spool:", err)
		return 1
	}
	defer store.Close()
	queued, err := store.AppendProbeObservation(ctx, *probeID, result)
	if err != nil {
		fmt.Fprintln(errOut, "report-probe queue:", err)
		return 1
	}
	tlsConfiguration, authenticatedProbe, err := transport.LoadProbeTLS(*controlCA, *certificate, *key, *serverName)
	if err != nil || authenticatedProbe != *probeID {
		fmt.Fprintln(errOut, "report-probe TLS identity does not match --probe-id; observation remains queued")
		return 1
	}
	client, err := transport.NewClient(*controlEndpoint, tlsConfiguration)
	if err != nil {
		fmt.Fprintln(errOut, "report-probe control transport:", err)
		return 1
	}
	defer client.Close()
	delivered, err := delivery.OnceProbes(ctx, store, client, *batchBytes)
	if err != nil {
		fmt.Fprintln(errOut, "report-probe publish; observation remains queued:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(struct {
		Type               string               `json:"type"`
		AuthenticatedProbe string               `json:"authenticated_probe"`
		ObservationID      string               `json:"observation_id"`
		Observation        extprobe.Result      `json:"observation"`
		Queued             spool.AppendResult   `json:"queued"`
		Result             delivery.ProbeResult `json:"result"`
	}{"probe_report", authenticatedProbe, mustProbeID(*probeID, result), result, queued, delivered}); err != nil {
		return 1
	}
	if !result.Expected {
		fmt.Fprintf(errOut, "report-probe: expected HTTP %d, received %d; observation persisted\n", result.ExpectedStatus, result.HTTPStatus)
		return 1
	}
	return 0
}

func mustProbeID(probe string, result extprobe.Result) string {
	id, _ := extprobe.ObservationIdentity(probe, result)
	return id
}
