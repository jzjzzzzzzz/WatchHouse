package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/certaudit"
)

type certAuditor func(string, time.Time, time.Duration) (certaudit.Report, error)

func runCertAudit(args []string, out, errOut io.Writer, audit certAuditor) int {
	flags := flag.NewFlagSet("audit-cert", flag.ContinueOnError)
	flags.SetOutput(errOut)
	path := flags.String("cert", "", "absolute certificate PEM path")
	minimum := flags.Duration("minimum-remaining", 30*24*time.Hour, "renewal warning window")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *path == "" || *minimum < 0 || *minimum > 365*24*time.Hour {
		fmt.Fprintln(errOut, "valid --cert and bounded --minimum-remaining are required")
		return 2
	}
	report, err := audit(*path, time.Now(), *minimum)
	if err != nil {
		fmt.Fprintln(errOut, "audit-cert:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(report); err != nil {
		fmt.Fprintln(errOut, "audit-cert: encode:", err)
		return 1
	}
	if report.Status != certaudit.Pass {
		return 3
	}
	return 0
}
