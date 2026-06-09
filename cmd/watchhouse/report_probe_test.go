package main

import (
	"bytes"
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"watchhouse/internal/spool"
)

func TestReportProbeQueuesBeforeControlTransportFailure(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	directory := t.TempDir()
	ca := filepath.Join(directory, "target-ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: target.Certificate().Raw}), 0o644); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(directory, "state")
	var out, errOut bytes.Buffer
	code := runReportProbe([]string{"--probe-id", "outside-1", "--state", state, "--url", target.URL,
		"--expect-status", "204", "--target-ca", ca, "--allow-private", "--control-endpoint", "https://127.0.0.1:1",
		"--control-ca", "/missing/control-ca", "--cert", "/missing/cert", "--key", "/missing/key", "--server-name", "control.test"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("exit %d output %q error %q", code, out.String(), errOut.String())
	}
	store, err := spool.Open(context.Background(), state, spool.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.PeekProbeObservations(context.Background(), 1, 1024*1024)
	if err != nil || len(items) != 1 || items[0].ProbeID != "outside-1" || !items[0].Result.Expected {
		t.Fatalf("retained probe observation %+v error %v", items, err)
	}
}
