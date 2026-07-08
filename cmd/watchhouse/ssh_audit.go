package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/sshaudit"
)

type sshAuditCollector func(context.Context, func() time.Time) (sshaudit.Report, error)

func runSSHAudit(args []string, out, errOut io.Writer, collect sshAuditCollector) int {
	flags := flag.NewFlagSet("audit-ssh", flag.ContinueOnError)
	flags.SetOutput(errOut)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOut, "audit-ssh accepts no paths, users, or addresses")
		return 2
	}
	report, err := collect(context.Background(), time.Now)
	if err != nil {
		fmt.Fprintln(errOut, "audit-ssh:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "audit-ssh: encode:", err)
		return 1
	}
	if report.Failed > 0 || report.Errors > 0 {
		return 3
	}
	return 0
}
