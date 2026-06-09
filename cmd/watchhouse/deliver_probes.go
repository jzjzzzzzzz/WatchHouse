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

	"watchhouse/internal/delivery"
	"watchhouse/internal/spool"
	"watchhouse/internal/transport"
)

func runDeliverProbes(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("deliver-probes", flag.ContinueOnError)
	flags.SetOutput(errOut)
	state := flags.String("state", "", "existing private spool directory")
	endpoint := flags.String("control-endpoint", "", "HTTPS control origin")
	ca := flags.String("control-ca", "", "control CA bundle")
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
	if flags.NArg() != 0 || *state == "" || *endpoint == "" || *ca == "" || *certificate == "" || *key == "" || *serverName == "" || *batchBytes < 1 || *batchBytes > 8*1024*1024 {
		fmt.Fprintln(errOut, "deliver-probes requires state, control endpoint/CA, certificate, key, server name, and valid bounds")
		return 2
	}
	if _, err := os.Stat(filepath.Join(*state, "queue.db")); err != nil {
		fmt.Fprintln(errOut, "deliver-probes requires existing spool:", err)
		return 1
	}
	tlsConfiguration, authenticatedProbe, err := transport.LoadProbeTLS(*ca, *certificate, *key, *serverName)
	if err != nil {
		fmt.Fprintln(errOut, "deliver-probes TLS identity:", err)
		return 1
	}
	client, err := transport.NewClient(*endpoint, tlsConfiguration)
	if err != nil {
		fmt.Fprintln(errOut, "deliver-probes transport:", err)
		return 1
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := spool.Open(ctx, *state, spool.Options{MaxBytes: *maxBytes, MaxRecords: *maxRecords})
	if err != nil {
		fmt.Fprintln(errOut, "deliver-probes spool:", err)
		return 1
	}
	defer store.Close()
	items, err := store.PeekProbeObservations(ctx, 1, *batchBytes)
	if err != nil {
		fmt.Fprintln(errOut, "deliver-probes queue:", err)
		return 1
	}
	if len(items) == 1 && items[0].ProbeID != authenticatedProbe {
		fmt.Fprintln(errOut, "deliver-probes queued probe does not match certificate; observation retained")
		return 1
	}
	result, err := delivery.OnceProbes(ctx, store, client, *batchBytes)
	if err != nil {
		fmt.Fprintln(errOut, "deliver-probes:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(struct {
		Type               string               `json:"type"`
		AuthenticatedProbe string               `json:"authenticated_probe"`
		Result             delivery.ProbeResult `json:"result"`
	}{"probe_delivery_result", authenticatedProbe, result}); err != nil {
		return 1
	}
	return 0
}
