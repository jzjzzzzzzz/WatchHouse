package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/extprobe"
)

func runProbeHTTPS(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("probe-https", flag.ContinueOnError)
	flags.SetOutput(errOut)
	target := flags.String("url", "", "exact HTTPS health URL without query")
	expectedStatus := flags.Int("expect-status", 200, "expected HTTP status")
	maxBody := flags.Int64("max-body", 64*1024, "response body hash bound, up to 1 MiB")
	timeout := flags.Duration("timeout", 15*time.Second, "overall timeout, 1s..1m")
	ca := flags.String("ca", "", "optional absolute private CA bundle; default system roots")
	allowPrivate := flags.Bool("allow-private", false, "explicitly permit loopback/private/link-local targets for a lab")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *target == "" {
		fmt.Fprintln(errOut, "probe-https requires --url and valid bounded options")
		return 2
	}
	roots, err := extprobe.LoadRoots(*ca)
	if err != nil {
		fmt.Fprintln(errOut, "probe-https trust roots:", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	result, err := extprobe.Run(ctx, extprobe.Config{URL: *target, ExpectedStatus: *expectedStatus, MaxBodyBytes: *maxBody,
		AllowPrivate: *allowPrivate, Roots: roots, Timeout: *timeout})
	if err != nil {
		fmt.Fprintln(errOut, "probe-https:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		return 1
	}
	if !result.Expected {
		fmt.Fprintf(errOut, "probe-https: expected HTTP %d, received %d\n", *expectedStatus, result.HTTPStatus)
		return 1
	}
	return 0
}
