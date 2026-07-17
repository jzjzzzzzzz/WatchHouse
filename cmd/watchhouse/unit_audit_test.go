package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/unitaudit"
)

func TestRunUnitAuditExitSemantics(t *testing.T) {
	for _, test := range []struct {
		name   string
		report unitaudit.Report
		err    error
		want   int
	}{
		{"pass", unitaudit.Report{SchemaVersion: 1, Units: []unitaudit.UnitReport{}, Passed: 30}, nil, 0},
		{"hardening failure", unitaudit.Report{SchemaVersion: 1, Units: []unitaudit.UnitReport{}, Failed: 1}, nil, 3},
		{"missing unit", unitaudit.Report{SchemaVersion: 1, Units: []unitaudit.UnitReport{}, Errors: 1}, nil, 3},
		{"systemctl failure", unitaudit.Report{}, errors.New("bus unavailable"), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runUnitAudit(nil, &out, &errOut, func(context.Context, func() time.Time) (unitaudit.Report, error) { return test.report, test.err })
			if code != test.want {
				t.Fatalf("code=%d want=%d out=%q err=%q", code, test.want, out.String(), errOut.String())
			}
			if test.err == nil && !strings.Contains(out.String(), `"schema_version":1`) {
				t.Fatalf("missing report: %q", out.String())
			}
		})
	}
}

func TestRunUnitAuditRejectsArbitraryUnit(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runUnitAudit([]string{"ssh.service"}, &out, &errOut, func(context.Context, func() time.Time) (unitaudit.Report, error) {
		called = true
		return unitaudit.Report{}, nil
	})
	if code != 2 || called || !strings.Contains(errOut.String(), "no unit names") {
		t.Fatalf("code=%d called=%v err=%q", code, called, errOut.String())
	}
}
