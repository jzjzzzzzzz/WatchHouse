package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/firewall"
)

type firewallCollector func(context.Context, func() time.Time) (firewall.Snapshot, error)

func runFirewall(args []string, out, errOut io.Writer, collect firewallCollector) int {
	flags := flag.NewFlagSet("firewall", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "firewall accepts no paths or command arguments")
		return 2
	}
	snapshot, err := collect(context.Background(), time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "firewall:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(snapshot); err != nil {
		fmt.Fprintln(errOut, "firewall: encode:", err)
		return 1
	}
	return 0
}
