package certaudit

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"
)

func certificatePEM(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	uri, _ := url.Parse("spiffe://watchhouse/host/lab")
	template := &x509.Certificate{SerialNumber: big.NewInt(42), Subject: pkix.Name{CommonName: "lab"}, NotBefore: notBefore, NotAfter: notAfter, DNSNames: []string{"lab.internal"}, URIs: []*url.URL{uri}, KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestAuditValidCertificate(t *testing.T) {
	now := time.Date(2026, 10, 5, 20, 0, 0, 0, time.UTC)
	got, err := Audit(certificatePEM(t, now.Add(-time.Hour), now.Add(60*24*time.Hour)), now, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Pass || got.Subject != "CN=lab" || got.Serial != "2a" || got.RemainingSeconds != int64((60*24*time.Hour).Seconds()) {
		t.Fatalf("%#v", got)
	}
	if len(got.DNSNames) != 1 || len(got.URIs) != 1 || len(got.SHA256) != 64 {
		t.Fatalf("%#v", got)
	}
}

func TestAuditValidityFailures(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name          string
		before, after time.Time
		detail        string
	}{
		{"future", now.Add(time.Hour), now.Add(48 * time.Hour), "not yet valid"},
		{"expired", now.Add(-48 * time.Hour), now.Add(-time.Hour), "expired"},
		{"renewal", now.Add(-time.Hour), now.Add(24 * time.Hour), "renewal window"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Audit(certificatePEM(t, test.before, test.after), now, 30*24*time.Hour)
			if err != nil || got.Status != Fail || !strings.Contains(got.Detail, test.detail) {
				t.Fatalf("got=%#v err=%v", got, err)
			}
		})
	}
}

func TestAuditRejectsAmbiguousPEM(t *testing.T) {
	now := time.Now()
	valid := certificatePEM(t, now.Add(-time.Hour), now.Add(time.Hour))
	for _, raw := range [][]byte{nil, append(valid, valid...), []byte("not pem"), make([]byte, MaxPEMBytes+1)} {
		if _, err := Audit(raw, now, 0); err == nil {
			t.Fatalf("accepted %d bytes", len(raw))
		}
	}
}

func FuzzAuditNeverPanics(f *testing.F) {
	f.Add([]byte("not pem"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxPEMBytes+1 {
			t.Skip()
		}
		_, _ = Audit(raw, time.Unix(0, 0), 0)
	})
}
