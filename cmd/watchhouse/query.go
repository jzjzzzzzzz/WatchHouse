package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

func runQueryEvents(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("query-events", flag.ContinueOnError)
	flags.SetOutput(errOut)
	host := flags.String("host", "", "authenticated host to query")
	endpoint := flags.String("endpoint", "", "HTTPS control origin")
	ca := flags.String("ca", "", "private CA bundle")
	certificate := flags.String("cert", "", "human certificate")
	key := flags.String("key", "", "human private key")
	serverName := flags.String("server-name", "", "verified control certificate name")
	limit := flags.Int("limit", 100, "records, 1..200")
	before := flags.Int64("before", 0, "exclusive server ingest sequence; zero is newest page")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || !telemetry.ValidHost(*host) || *endpoint == "" || *ca == "" || *certificate == "" || *key == "" || *serverName == "" || *limit < 1 || *limit > 200 || *before < 0 {
		fmt.Fprintln(errOut, "query-events requires host, HTTPS endpoint, CA, human certificate/key, server name, and valid page bounds")
		return 2
	}
	tlsConfiguration, user, err := transport.LoadHumanTLS(*ca, *certificate, *key, *serverName)
	if err != nil {
		fmt.Fprintln(errOut, "query-events TLS identity:", err)
		return 1
	}
	client, err := transport.NewClient(*endpoint, tlsConfiguration)
	if err != nil {
		fmt.Fprintln(errOut, "query-events transport:", err)
		return 1
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	page, err := client.QueryEvents(ctx, *host, *limit, *before)
	if err != nil {
		fmt.Fprintln(errOut, "query-events:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(struct {
		Type              string              `json:"type"`
		AuthenticatedUser string              `json:"authenticated_user"`
		Page              transport.EventPage `json:"page"`
	}{"event_query", user, page}); err != nil {
		return 1
	}
	return 0
}
