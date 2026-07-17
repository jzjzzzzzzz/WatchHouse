package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/unitaudit"
)

type unitAuditCollector func(context.Context, func() time.Time) (unitaudit.Report, error)

func runUnitAudit(args []string, out, errOut io.Writer, collect unitAuditCollector) int {
	flags := flag.NewFlagSet("audit-units", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "audit-units accepts no unit names")
		return 2
	}
	report, err := collect(context.Background(), time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "audit-units:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "audit-units: encode:", err)
		return 1
	}
	if report.Failed > 0 || report.Errors > 0 {
		return 3
	}
	return 0
}
