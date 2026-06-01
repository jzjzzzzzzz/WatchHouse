package main

import (
	"bytes"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestProbeHTTPSCommandReportsExpectedAndUnexpectedStatus(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain")
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"--url", server.URL + "/health", "--ca", ca, "--allow-private", "--expect-status", strconv.Itoa(http.StatusNoContent)}
	var out, errOut bytes.Buffer
	if code := runProbeHTTPS(args, &out, &errOut); code != 0 || !bytes.Contains(out.Bytes(), []byte(`"expected":true`)) {
		t.Fatalf("code %d output %q error %q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	args[len(args)-1] = strconv.Itoa(http.StatusOK)
	if code := runProbeHTTPS(args, &out, &errOut); code != 1 || !bytes.Contains(out.Bytes(), []byte(`"expected":false`)) {
		t.Fatalf("unexpected code %d output %q error %q", code, out.String(), errOut.String())
	}
}
