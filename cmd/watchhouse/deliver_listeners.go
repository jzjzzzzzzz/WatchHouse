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

func runDeliverListeners(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("deliver-listeners", flag.ContinueOnError)
	flags.SetOutput(errOut)
	state := flags.String("state", "", "existing private spool directory")
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
	if flags.NArg() != 0 || *state == "" || *endpoint == "" || *ca == "" || *certificate == "" || *key == "" || *serverName == "" || *batchBytes < 1 || *batchBytes > 8*1024*1024 {
		fmt.Fprintln(errOut, "deliver-listeners requires state, HTTPS endpoint, CA, certificate, key, server name, and valid bounds")
		return 2
	}
	if _, err := os.Stat(filepath.Join(*state, "queue.db")); err != nil {
		fmt.Fprintln(errOut, "deliver-listeners requires existing spool:", err)
		return 1
	}
	tlsConfiguration, authenticatedHost, err := transport.LoadClientTLS(*ca, *certificate, *key, *serverName)
	if err != nil {
		fmt.Fprintln(errOut, "deliver-listeners TLS identity:", err)
		return 1
	}
	client, err := transport.NewClient(*endpoint, tlsConfiguration)
	if err != nil {
		fmt.Fprintln(errOut, "deliver-listeners transport:", err)
		return 1
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := spool.Open(ctx, *state, spool.Options{MaxBytes: *maxBytes, MaxRecords: *maxRecords})
	if err != nil {
		fmt.Fprintln(errOut, "deliver-listeners spool:", err)
		return 1
	}
	defer store.Close()
	items, err := store.PeekListenerSnapshots(ctx, 1, *batchBytes)
	if err != nil {
		fmt.Fprintln(errOut, "deliver-listeners queue:", err)
		return 1
	}
	if len(items) == 1 && items[0].HostID != authenticatedHost {
		fmt.Fprintln(errOut, "deliver-listeners queued host does not match certificate; snapshot retained")
		return 1
	}
	result, err := delivery.OnceListeners(ctx, store, client, *batchBytes)
	if err != nil {
		fmt.Fprintln(errOut, "deliver-listeners:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(struct {
		Type              string                  `json:"type"`
		AuthenticatedHost string                  `json:"authenticated_host"`
		Result            delivery.ListenerResult `json:"result"`
	}{"listener_delivery_result", authenticatedHost, result}); err != nil {
		return 1
	}
	return 0
}
