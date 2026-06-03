package extprobe

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T, handler http.Handler) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	return server, roots
}

func TestProbePinsResolvedAddressAndRecordsTLSEvidence(t *testing.T) {
	server, roots := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/plain")
		response.WriteHeader(http.StatusNoContent)
	}))
	result, err := Run(context.Background(), Config{URL: server.URL + "/health", ExpectedStatus: http.StatusNoContent,
		MaxBodyBytes: 1024, AllowPrivate: true, Roots: roots, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Expected || result.HTTPStatus != http.StatusNoContent || result.ConnectedAddress == "" || len(result.ResolvedAddresses) != 1 ||
		result.TLSVersion == "" || len(result.PeerCertificateSHA256) != 64 || result.PrivateTargetsAllowed != true {
		t.Fatalf("probe result %+v", result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}
	result.ConnectedAddress = "192.0.2.1:443"
	if err := result.Validate(); err == nil {
		t.Fatal("connection outside DNS set accepted")
	}
}

func TestProbeRejectsPrivateTargetsAndRedirects(t *testing.T) {
	server, roots := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "https://example.invalid/", http.StatusFound)
	}))
	base := Config{URL: server.URL, ExpectedStatus: http.StatusOK, MaxBodyBytes: 1024, Roots: roots, Timeout: 5 * time.Second}
	if _, err := Run(context.Background(), base); err == nil || !strings.Contains(err.Error(), "no permitted") {
		t.Fatalf("private target error %v", err)
	}
	base.AllowPrivate = true
	if _, err := Run(context.Background(), base); err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("redirect error %v", err)
	}
}

func TestProbeBoundsResponseAndTargetSyntax(t *testing.T) {
	server, roots := testServer(t, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("too large"))
	}))
	config := Config{URL: server.URL, ExpectedStatus: http.StatusOK, MaxBodyBytes: 3, AllowPrivate: true, Roots: roots, Timeout: 5 * time.Second}
	if _, err := Run(context.Background(), config); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("body bound error %v", err)
	}
	config.URL = "https://user@example.com/?secret=x"
	if _, err := Run(context.Background(), config); err == nil {
		t.Fatal("credentialed query URL accepted")
	}
}
