package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/diskaudit"
)

type diskAuditCollector func(func() time.Time) (diskaudit.Report, error)

func runDiskAudit(args []string, out, errOut io.Writer, collect diskAuditCollector) int {
	flags := flag.NewFlagSet("audit-disk", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "audit-disk accepts no paths or thresholds")
		return 2
	}
	report, err := collect(time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "audit-disk:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "audit-disk: encode:", err)
		return 1
	}
	if report.Failed > 0 || report.Errors > 0 {
		return 3
	}
	return 0
}
