package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/sshaudit"
)

func TestRunSSHAuditExitSemantics(t *testing.T) {
	for _, test := range []struct {
		name   string
		report sshaudit.Report
		err    error
		want   int
	}{
		{"pass", sshaudit.Report{SchemaVersion: 1, Results: []sshaudit.Result{}, Passed: 10}, nil, 0},
		{"policy failure", sshaudit.Report{SchemaVersion: 1, Results: []sshaudit.Result{}, Failed: 1}, nil, 3},
		{"missing evidence", sshaudit.Report{SchemaVersion: 1, Results: []sshaudit.Result{}, Errors: 1}, nil, 3},
		{"collector failure", sshaudit.Report{}, errors.New("sshd rejected config"), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runSSHAudit(nil, &out, &errOut, func(context.Context, func() time.Time) (sshaudit.Report, error) { return test.report, test.err })
			if code != test.want {
				t.Fatalf("code=%d want=%d out=%q err=%q", code, test.want, out.String(), errOut.String())
			}
			if test.err == nil && !strings.Contains(out.String(), `"schema_version":1`) {
				t.Fatalf("missing report: %q", out.String())
			}
		})
	}
}

func TestRunSSHAuditRejectsContextInjection(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runSSHAudit([]string{"user=attacker"}, &out, &errOut, func(context.Context, func() time.Time) (sshaudit.Report, error) {
		called = true
		return sshaudit.Report{}, nil
	})
	if code != 2 || called || !strings.Contains(errOut.String(), "no paths") {
		t.Fatalf("code=%d called=%v err=%q", code, called, errOut.String())
	}
}
