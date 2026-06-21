package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/posture"
)

func TestRunPostureExitCodesAreAutomationSafe(t *testing.T) {
	tests := []struct {
		name   string
		report posture.Report
		err    error
		code   int
	}{
		{"pass", posture.Report{SchemaVersion: 1, Results: []posture.Result{}, Passed: 2}, nil, 0},
		{"failed control", posture.Report{SchemaVersion: 1, Results: []posture.Result{}, Failed: 1}, nil, 3},
		{"collection error result", posture.Report{SchemaVersion: 1, Results: []posture.Result{}, Errors: 1}, nil, 3},
		{"collector failure", posture.Report{}, errors.New("unavailable"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runPosture(nil, &out, &errOut, func(func() time.Time) (posture.Report, error) { return tt.report, tt.err })
			if code != tt.code {
				t.Fatalf("code=%d want=%d out=%q err=%q", code, tt.code, out.String(), errOut.String())
			}
			if tt.err == nil && !strings.Contains(out.String(), `"schema_version":1`) {
				t.Fatalf("missing report: %q", out.String())
			}
		})
	}
}

func TestRunPostureRejectsCallerOverrides(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runPosture([]string{"--root", "/tmp"}, &out, &errOut, func(func() time.Time) (posture.Report, error) { called = true; return posture.Report{}, nil })
	if code != 2 || called {
		t.Fatalf("code=%d called=%v err=%q", code, called, errOut.String())
	}
}
