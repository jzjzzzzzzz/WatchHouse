package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/hostview"
	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

type listenerSnapshotter func() (hostview.HostSnapshot, error)

func runReportListeners(args []string, out, errOut io.Writer, snapshot listenerSnapshotter) int {
	flags := flag.NewFlagSet("report-listeners", flag.ContinueOnError)
	flags.SetOutput(errOut)
	host := flags.String("host", "", "host identity bound to the agent certificate")
	endpoint := flags.String("endpoint", "", "HTTPS control origin")
	ca := flags.String("ca", "", "private CA bundle")
	certificate := flags.String("cert", "", "agent certificate")
	key := flags.String("key", "", "agent private key")
	serverName := flags.String("server-name", "", "verified control certificate name")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || !telemetry.ValidHost(*host) || *endpoint == "" || *ca == "" || *certificate == "" || *key == "" || *serverName == "" {
		fmt.Fprintln(errOut, "report-listeners requires host, HTTPS endpoint, CA, agent certificate/key, and server name")
		return 2
	}
	tlsConfiguration, authenticatedHost, err := transport.LoadClientTLS(*ca, *certificate, *key, *serverName)
	if err != nil || authenticatedHost != *host {
		fmt.Fprintln(errOut, "report-listeners TLS identity does not match --host")
		return 1
	}
	observed, err := snapshot()
	if err != nil {
		fmt.Fprintln(errOut, "report-listeners snapshot:", err)
		return 1
	}
	client, err := transport.NewClient(*endpoint, tlsConfiguration)
	if err != nil {
		fmt.Fprintln(errOut, "report-listeners transport:", err)
		return 1
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	receipt, err := client.PublishListenerSnapshot(ctx, *host, observed)
	if err != nil {
		fmt.Fprintln(errOut, "report-listeners:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(struct {
		Type              string                            `json:"type"`
		AuthenticatedHost string                            `json:"authenticated_host"`
		Listeners         int                               `json:"listeners"`
		Receipt           transport.ListenerSnapshotReceipt `json:"receipt"`
	}{"listener_snapshot_report", authenticatedHost, len(observed.Listeners), receipt}); err != nil {
		return 1
	}
	return 0
}
