package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/pkginventory"
)

func TestRunPackagesEmitsBoundedReport(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runPackages(nil, &out, &errOut, func(_ context.Context, now func() time.Time) (pkginventory.Report, error) {
		if now().IsZero() {
			t.Fatal("zero clock")
		}
		return pkginventory.Report{SchemaVersion: 1, Manager: "dpkg", PackageCount: 1, Packages: []pkginventory.Package{{Name: "base-files", Version: "1", Architecture: "arm64"}}}, nil
	})
	if code != 0 || !strings.Contains(out.String(), `"manager":"dpkg"`) || errOut.Len() != 0 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestRunPackagesRejectsArgumentsAndReportsFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	if code := runPackages([]string{"--root", "/tmp"}, &out, &errOut, func(context.Context, func() time.Time) (pkginventory.Report, error) {
		called = true
		return pkginventory.Report{}, nil
	}); code != 2 || called {
		t.Fatalf("code=%d called=%v", code, called)
	}
	out.Reset()
	errOut.Reset()
	if code := runPackages(nil, &out, &errOut, func(context.Context, func() time.Time) (pkginventory.Report, error) {
		return pkginventory.Report{}, errors.New("dpkg locked")
	}); code != 1 || !strings.Contains(errOut.String(), "dpkg locked") {
		t.Fatalf("code=%d err=%q", code, errOut.String())
	}
}
