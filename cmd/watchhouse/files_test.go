package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/fileaudit"
)

func TestRunFileAuditProducesReportAndPolicyExit(t *testing.T) {
	for _, test := range []struct {
		name   string
		report fileaudit.Report
		want   int
	}{
		{"clean", fileaudit.Report{SchemaVersion: 1, Results: []fileaudit.Result{}, Passed: 6}, 0},
		{"failure", fileaudit.Report{SchemaVersion: 1, Results: []fileaudit.Result{}, Failed: 1}, 3},
		{"observation error", fileaudit.Report{SchemaVersion: 1, Results: []fileaudit.Result{}, Errors: 1}, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runFileAudit(nil, &out, &errOut, func(func() time.Time) (fileaudit.Report, error) { return test.report, nil })
			if code != test.want || !strings.Contains(out.String(), `"schema_version":1`) || errOut.Len() != 0 {
				t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
			}
		})
	}
}

func TestRunFileAuditSeparatesCollectionFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runFileAudit(nil, &out, &errOut, func(func() time.Time) (fileaudit.Report, error) {
		return fileaudit.Report{}, errors.New("stat unavailable")
	})
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "stat unavailable") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestRunFileAuditRejectsPathInjection(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runFileAudit([]string{"/tmp/passwd"}, &out, &errOut, func(func() time.Time) (fileaudit.Report, error) { called = true; return fileaudit.Report{}, nil })
	if code != 2 || called || !strings.Contains(errOut.String(), "no paths") {
		t.Fatalf("code=%d called=%v err=%q", code, called, errOut.String())
	}
}
