package certaudit

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

const MaxPEMBytes = 1 << 20

type Status string

const (
	Pass Status = "pass"
	Fail Status = "fail"
)

type Report struct {
	SchemaVersion    int       `json:"schema_version"`
	ObservedAt       time.Time `json:"observed_at"`
	Status           Status    `json:"status"`
	Subject          string    `json:"subject"`
	Issuer           string    `json:"issuer"`
	Serial           string    `json:"serial"`
	NotBefore        time.Time `json:"not_before"`
	NotAfter         time.Time `json:"not_after"`
	RemainingSeconds int64     `json:"remaining_seconds"`
	SHA256           string    `json:"sha256"`
	DNSNames         []string  `json:"dns_names"`
	URIs             []string  `json:"uris"`
	Expected         string    `json:"expected"`
	Detail           string    `json:"detail,omitempty"`
}

func Audit(raw []byte, now time.Time, minimumRemaining time.Duration) (Report, error) {
	if len(raw) == 0 || len(raw) > MaxPEMBytes {
		return Report{}, fmt.Errorf("certificate PEM size must be 1..%d bytes", MaxPEMBytes)
	}
	if minimumRemaining < 0 || minimumRemaining > 365*24*time.Hour {
		return Report{}, fmt.Errorf("invalid minimum remaining duration")
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return Report{}, fmt.Errorf("expected exactly one CERTIFICATE PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return Report{}, fmt.Errorf("parse certificate: %w", err)
	}
	sum := sha256.Sum256(cert.Raw)
	report := Report{SchemaVersion: 1, ObservedAt: now.UTC(), Subject: cert.Subject.String(), Issuer: cert.Issuer.String(), Serial: cert.SerialNumber.Text(16), NotBefore: cert.NotBefore.UTC(), NotAfter: cert.NotAfter.UTC(), RemainingSeconds: int64(cert.NotAfter.Sub(now).Seconds()), SHA256: hex.EncodeToString(sum[:]), DNSNames: append([]string{}, cert.DNSNames...), URIs: []string{}, Expected: fmt.Sprintf("currently valid with at least %s remaining", minimumRemaining)}
	for _, uri := range cert.URIs {
		report.URIs = append(report.URIs, uri.String())
	}
	switch {
	case now.Before(cert.NotBefore):
		report.Status, report.Detail = Fail, "certificate is not yet valid"
	case !now.Before(cert.NotAfter):
		report.Status, report.Detail = Fail, "certificate is expired"
	case cert.NotAfter.Sub(now) < minimumRemaining:
		report.Status, report.Detail = Fail, "certificate is inside renewal window"
	default:
		report.Status = Pass
	}
	return report, nil
}
