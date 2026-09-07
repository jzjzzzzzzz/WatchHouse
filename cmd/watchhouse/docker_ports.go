package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/dockerports"
)

type dockerPortCollector func(context.Context, func() time.Time) (dockerports.Report, error)

func runDockerPorts(args []string, out, errOut io.Writer, collect dockerPortCollector) int {
	flags := flag.NewFlagSet("docker-ports", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "docker-ports accepts no sockets, commands, or container IDs")
		return 2
	}
	report, err := collect(context.Background(), time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "docker-ports:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "docker-ports: encode:", err)
		return 1
	}
	return 0
}
