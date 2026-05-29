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
	"watchhouse/internal/hostview"
	"watchhouse/internal/spool"
	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

type listenerSnapshotter func() (hostview.HostSnapshot, error)

func runReportListeners(args []string, out, errOut io.Writer, snapshot listenerSnapshotter) int {
	flags := flag.NewFlagSet("report-listeners", flag.ContinueOnError)
	flags.SetOutput(errOut)
	host := flags.String("host", "", "host identity bound to the agent certificate")
	state := flags.String("state", "", "private durable spool directory")
	endpoint := flags.String("endpoint", "", "HTTPS control origin")
	ca := flags.String("ca", "", "private CA bundle")
	certificate := flags.String("cert", "", "agent certificate")
	key := flags.String("key", "", "agent private key")
	serverName := flags.String("server-name", "", "verified control certificate name")
	batchBytes := flags.Int64("batch-bytes", 1024*1024, "listener payload budget, up to 8 MiB")
	defaults := spool.DefaultOptions()
	maxBytes := flags.Int64("max-bytes", defaults.MaxBytes, "listener outbox logical payload limit")
	maxRecords := flags.Int("max-records", defaults.MaxRecords, "listener outbox record limit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || !telemetry.ValidHost(*host) || *state == "" || *endpoint == "" || *ca == "" || *certificate == "" || *key == "" || *serverName == "" || *batchBytes < 1 || *batchBytes > 8*1024*1024 {
		fmt.Fprintln(errOut, "report-listeners requires host, state, HTTPS endpoint, CA, agent certificate/key, server name, and valid bounds")
		return 2
	}
	observed, err := snapshot()
	if err != nil {
		fmt.Fprintln(errOut, "report-listeners snapshot:", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := spool.Open(ctx, *state, spool.Options{MaxBytes: *maxBytes, MaxRecords: *maxRecords})
	if err != nil {
		fmt.Fprintln(errOut, "report-listeners spool:", err)
		return 1
	}
	defer store.Close()
	queued, err := store.AppendListenerSnapshot(ctx, *host, observed)
	if err != nil {
		fmt.Fprintln(errOut, "report-listeners queue:", err)
		return 1
	}
	tlsConfiguration, authenticatedHost, err := transport.LoadClientTLS(*ca, *certificate, *key, *serverName)
	if err != nil || authenticatedHost != *host {
		fmt.Fprintln(errOut, "report-listeners TLS identity does not match --host; snapshot remains queued")
		return 1
	}
	client, err := transport.NewClient(*endpoint, tlsConfiguration)
	if err != nil {
		fmt.Fprintln(errOut, "report-listeners transport:", err)
		return 1
	}
	defer client.Close()
	result, err := delivery.OnceListeners(ctx, store, client, *batchBytes)
	if err != nil {
		fmt.Fprintln(errOut, "report-listeners delivery; snapshot remains queued:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(struct {
		Type              string                  `json:"type"`
		AuthenticatedHost string                  `json:"authenticated_host"`
		SnapshotID        string                  `json:"snapshot_id"`
		Listeners         int                     `json:"listeners"`
		Queued            spool.AppendResult      `json:"queued"`
		Result            delivery.ListenerResult `json:"result"`
	}{"listener_snapshot_report", authenticatedHost, hostview.SnapshotIdentity(*host, observed), len(observed.Listeners), queued, result}); err != nil {
		return 1
	}
	return 0
}
