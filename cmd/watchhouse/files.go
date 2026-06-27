package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/fileaudit"
)

type fileAuditCollector func(func() time.Time) (fileaudit.Report, error)

func runFileAudit(args []string, out, errOut io.Writer, collect fileAuditCollector) int {
	flags := flag.NewFlagSet("audit-files", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "audit-files accepts no paths")
		return 2
	}
	report, err := collect(time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "audit-files:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "audit-files: encode:", err)
		return 1
	}
	if report.Failed > 0 || report.Errors > 0 {
		return 3
	}
	return 0
}
