package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/hostview"
	"watchhouse/internal/networkcompare"
)

func runExposure(args []string, out, errOut io.Writer, snapshot func() (hostview.HostSnapshot, error), containers dockerPortCollector) int {
	flags := flag.NewFlagSet("exposure", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "exposure accepts no namespaces, sockets, or container IDs")
		return 2
	}
	listeners, err := snapshot()
	if err != nil {
		fmt.Fprintln(errOut, "exposure: listeners:", err)
		return 1
	}
	bindings, err := containers(context.Background(), time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "exposure: Docker ports:", err)
		return 1
	}
	report := networkcompare.Compare(listeners, bindings, time.Now())
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "exposure: encode:", err)
		return 1
	}
	return 0
}
