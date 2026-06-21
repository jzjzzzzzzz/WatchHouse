package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/posture"
)

type postureCollector func(func() time.Time) (posture.Report, error)

func runPosture(args []string, out, errOut io.Writer, collect postureCollector) int {
	flags := flag.NewFlagSet("posture", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "posture accepts no paths or control overrides")
		return 2
	}
	report, err := collect(time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "posture:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "posture: encode:", err)
		return 1
	}
	if report.Failed > 0 || report.Errors > 0 {
		return 3
	}
	return 0
}
