package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/pkginventory"
)

type packageCollector func(context.Context, func() time.Time) (pkginventory.Report, error)

func runPackages(args []string, out, errOut io.Writer, collect packageCollector) int {
	flags := flag.NewFlagSet("packages", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "packages accepts no commands, roots, or package filters")
		return 2
	}
	report, err := collect(context.Background(), time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "packages:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "packages: encode:", err)
		return 1
	}
	return 0
}
