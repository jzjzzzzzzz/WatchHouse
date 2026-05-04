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

func runDeliver(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("deliver", flag.ContinueOnError)
	flags.SetOutput(errOut)
	state := flags.String("state", "", "existing private spool directory")
	endpoint := flags.String("endpoint", "", "HTTPS control origin")
	ca := flags.String("ca", "", "private CA bundle")
	certificate := flags.String("cert", "", "agent certificate")
	key := flags.String("key", "", "agent private key")
	serverName := flags.String("server-name", "", "verified control certificate name")
	limit := flags.Int("limit", 100, "records per delivery, 1..500")
	batchBytes := flags.Int64("batch-bytes", 1024*1024, "encoded event payload budget, up to 8 MiB")
	defaults := spool.DefaultOptions()
	maxBytes := flags.Int64("max-bytes", defaults.MaxBytes, "spool logical payload limit")
	maxRecords := flags.Int("max-records", defaults.MaxRecords, "spool record limit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *state == "" || *endpoint == "" || *ca == "" || *certificate == "" || *key == "" || *serverName == "" ||
		*limit < 1 || *limit > 500 || *batchBytes < 1 || *batchBytes > 8*1024*1024 {
		fmt.Fprintln(errOut, "deliver requires state, HTTPS endpoint, CA, certificate, key, server name, and valid bounds")
		return 2
	}
	if _, err := os.Stat(filepath.Join(*state, "queue.db")); err != nil {
		fmt.Fprintln(errOut, "deliver requires existing spool:", err)
		return 1
	}
	tlsConfiguration, authenticatedHost, err := transport.LoadClientTLS(*ca, *certificate, *key, *serverName)
	if err != nil {
		fmt.Fprintln(errOut, "deliver TLS identity:", err)
		return 1
	}
	client, err := transport.NewClient(*endpoint, tlsConfiguration)
	if err != nil {
		fmt.Fprintln(errOut, "deliver transport:", err)
		return 1
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := spool.Open(ctx, *state, spool.Options{MaxBytes: *maxBytes, MaxRecords: *maxRecords})
	if err != nil {
		fmt.Fprintln(errOut, "deliver spool:", err)
		return 1
	}
	defer store.Close()
	result, err := delivery.Once(ctx, store, client, *limit, *batchBytes)
	if err != nil {
		fmt.Fprintln(errOut, "deliver:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(struct {
		Type              string          `json:"type"`
		AuthenticatedHost string          `json:"authenticated_host"`
		Result            delivery.Result `json:"result"`
	}{"delivery_result", authenticatedHost, result}); err != nil {
		return 1
	}
	return 0
}
