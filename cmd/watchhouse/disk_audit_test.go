package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/diskaudit"
)

func TestRunDiskAuditExitSemantics(t *testing.T) {
	for _, test := range []struct {
		name   string
		report diskaudit.Report
		err    error
		want   int
	}{
		{"capacity healthy", diskaudit.Report{SchemaVersion: 1, Results: []diskaudit.Result{}, Passed: 2}, nil, 0},
		{"capacity low", diskaudit.Report{SchemaVersion: 1, Results: []diskaudit.Result{}, Failed: 1}, nil, 3},
		{"path missing", diskaudit.Report{SchemaVersion: 1, Results: []diskaudit.Result{}, Errors: 1}, nil, 3},
		{"collector failure", diskaudit.Report{}, errors.New("statfs failed"), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runDiskAudit(nil, &out, &errOut, func(func() time.Time) (diskaudit.Report, error) { return test.report, test.err })
			if code != test.want {
				t.Fatalf("code=%d want=%d out=%q err=%q", code, test.want, out.String(), errOut.String())
			}
			if test.err == nil && !strings.Contains(out.String(), `"schema_version":1`) {
				t.Fatalf("missing report: %q", out.String())
			}
		})
	}
}

func TestRunDiskAuditRejectsThresholdOverride(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runDiskAudit([]string{"--minimum", "0"}, &out, &errOut, func(func() time.Time) (diskaudit.Report, error) { called = true; return diskaudit.Report{}, nil })
	if code != 2 || called || !strings.Contains(errOut.String(), "flag provided but not defined") {
		t.Fatalf("code=%d called=%v err=%q", code, called, errOut.String())
	}
}
