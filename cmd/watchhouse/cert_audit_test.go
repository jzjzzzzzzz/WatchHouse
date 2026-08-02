package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/certaudit"
)

func TestRunCertAuditExitSemantics(t *testing.T) {
	for _, test := range []struct {
		name   string
		report certaudit.Report
		err    error
		want   int
	}{
		{"valid", certaudit.Report{SchemaVersion: 1, Status: certaudit.Pass, DNSNames: []string{}, URIs: []string{}}, nil, 0},
		{"renewal due", certaudit.Report{SchemaVersion: 1, Status: certaudit.Fail, DNSNames: []string{}, URIs: []string{}}, nil, 3},
		{"invalid PEM", certaudit.Report{}, errors.New("invalid PEM"), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := runCertAudit([]string{"--cert", "/etc/watchhouse/client.pem"}, &out, &errOut, func(path string, _ time.Time, minimum time.Duration) (certaudit.Report, error) {
				if path != "/etc/watchhouse/client.pem" || minimum != 30*24*time.Hour {
					t.Fatalf("path=%q minimum=%s", path, minimum)
				}
				return test.report, test.err
			})
			if code != test.want {
				t.Fatalf("code=%d want=%d out=%q err=%q", code, test.want, out.String(), errOut.String())
			}
			if test.err == nil && !strings.Contains(out.String(), `"schema_version":1`) {
				t.Fatalf("missing report: %q", out.String())
			}
		})
	}
}

func TestRunCertAuditValidatesArgumentsBeforeReading(t *testing.T) {
	for _, args := range [][]string{nil, {"--cert", "/x", "--minimum-remaining", "-1s"}, {"--cert", "/x", "extra"}} {
		var out, errOut bytes.Buffer
		called := false
		code := runCertAudit(args, &out, &errOut, func(string, time.Time, time.Duration) (certaudit.Report, error) {
			called = true
			return certaudit.Report{}, nil
		})
		if code != 2 || called {
			t.Fatalf("args=%v code=%d called=%v err=%q", args, code, called, errOut.String())
		}
	}
}
