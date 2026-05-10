// Package transport defines the authenticated, bounded agent event protocol.
package transport

import (
	"crypto/x509"
	"fmt"
	"net/url"

	"watchhouse/internal/telemetry"
)

const (
	SchemaVersion = 1
	MaxItems      = 500
	MaxBodyBytes  = 8 * 1024 * 1024
)

type Item struct {
	Sequence int64           `json:"sequence"`
	EventID  string          `json:"event_id"`
	Event    telemetry.Event `json:"event"`
}

type BatchRequest struct {
	SchemaVersion int    `json:"schema_version"`
	Items         []Item `json:"items"`
}

type Receipt struct {
	Sequence int64  `json:"sequence"`
	EventID  string `json:"event_id"`
}

type BatchResponse struct {
	SchemaVersion int       `json:"schema_version"`
	Receipts      []Receipt `json:"receipts"`
}

func (request BatchRequest) Validate(authenticatedHost string) error {
	if request.SchemaVersion != SchemaVersion || !telemetry.ValidHost(authenticatedHost) || len(request.Items) < 1 || len(request.Items) > MaxItems {
		return fmt.Errorf("invalid event batch envelope")
	}
	sequences := map[int64]struct{}{}
	identities := map[string]struct{}{}
	for index, item := range request.Items {
		if item.Sequence < 1 || item.EventID != item.Event.EventID || item.Event.HostID != authenticatedHost {
			return fmt.Errorf("event batch item %d identity mismatch", index)
		}
		if err := item.Event.Validate(); err != nil {
			return fmt.Errorf("event batch item %d: %w", index, err)
		}
		if _, exists := sequences[item.Sequence]; exists {
			return fmt.Errorf("event batch repeats sequence")
		}
		if _, exists := identities[item.EventID]; exists {
			return fmt.Errorf("event batch repeats event identity")
		}
		sequences[item.Sequence] = struct{}{}
		identities[item.EventID] = struct{}{}
	}
	return nil
}

func (response BatchResponse) ValidateExact(items []Item) error {
	if response.SchemaVersion != SchemaVersion || len(response.Receipts) != len(items) || len(items) < 1 || len(items) > MaxItems {
		return fmt.Errorf("receipt set has wrong envelope or cardinality")
	}
	expected := make(map[int64]string, len(items))
	for _, item := range items {
		expected[item.Sequence] = item.EventID
	}
	seen := make(map[int64]struct{}, len(items))
	for _, receipt := range response.Receipts {
		identity, exists := expected[receipt.Sequence]
		if !exists || identity != receipt.EventID {
			return fmt.Errorf("receipt is not an exact submitted identity")
		}
		if _, duplicate := seen[receipt.Sequence]; duplicate {
			return fmt.Errorf("receipt sequence repeated")
		}
		seen[receipt.Sequence] = struct{}{}
	}
	return nil
}

func HostFromCertificate(certificate *x509.Certificate) (string, error) {
	return identityFromCertificate(certificate, "host")
}

func UserFromCertificate(certificate *x509.Certificate) (string, error) {
	return identityFromCertificate(certificate, "user")
}

func identityFromCertificate(certificate *x509.Certificate, kind string) (string, error) {
	if certificate == nil || len(certificate.URIs) != 1 {
		return "", fmt.Errorf("client certificate requires exactly one URI SAN")
	}
	identity := certificate.URIs[0]
	if identity == nil || identity.Scheme != "spiffe" || identity.Host != "watchhouse" || identity.User != nil || identity.RawQuery != "" || identity.Fragment != "" || identity.RawPath != "" {
		return "", fmt.Errorf("invalid client identity URI SAN")
	}
	prefix := "/" + kind + "/"
	if len(identity.Path) <= len(prefix) || identity.Path[:len(prefix)] != prefix {
		return "", fmt.Errorf("invalid client identity URI SAN")
	}
	principal := identity.Path[len(prefix):]
	if !telemetry.ValidHost(principal) {
		return "", fmt.Errorf("invalid authenticated principal ID")
	}
	return principal, nil
}

func HostURI(host string) (*url.URL, error) {
	return identityURI("host", host)
}

func UserURI(user string) (*url.URL, error) {
	return identityURI("user", user)
}

func identityURI(kind, identity string) (*url.URL, error) {
	if !telemetry.ValidHost(identity) {
		return nil, fmt.Errorf("invalid identity")
	}
	return url.Parse("spiffe://watchhouse/" + kind + "/" + identity)
}
