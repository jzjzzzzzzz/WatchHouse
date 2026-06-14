package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/firewall"
)

func TestRunFirewallWritesSnapshot(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runFirewall(nil, &out, &errOut, func(_ context.Context, now func() time.Time) (firewall.Snapshot, error) {
		called = true
		if now().IsZero() {
			t.Fatal("zero clock")
		}
		return firewall.Snapshot{SchemaVersion: 1, Tables: []firewall.Table{}, BaseChains: []firewall.BaseChain{}}, nil
	})
	if code != 0 || !called || !strings.Contains(out.String(), `"schema_version":1`) || errOut.Len() != 0 {
		t.Fatalf("code=%d called=%v out=%q err=%q", code, called, out.String(), errOut.String())
	}
}

func TestRunFirewallRejectsArgumentsWithoutCollection(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runFirewall([]string{"/tmp/rules"}, &out, &errOut, func(context.Context, func() time.Time) (firewall.Snapshot, error) {
		called = true
		return firewall.Snapshot{}, nil
	})
	if code != 2 || called || !strings.Contains(errOut.String(), "no paths") {
		t.Fatalf("code=%d called=%v err=%q", code, called, errOut.String())
	}
}

func TestRunFirewallReportsCollectorFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runFirewall(nil, &out, &errOut, func(context.Context, func() time.Time) (firewall.Snapshot, error) {
		return firewall.Snapshot{}, errors.New("permission denied")
	})
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "permission denied") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}
