// Package telemetry normalizes read-only host observations.
package telemetry

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"time"
)

const MaxRecordBytes = 64 * 1024

var hostPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var bootPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Authentication struct {
	Outcome     string `json:"outcome"`
	Method      string `json:"method"`
	User        string `json:"user"`
	InvalidUser bool   `json:"invalid_user"`
	SourceIP    string `json:"source_ip"`
	SourcePort  uint16 `json:"source_port"`
}

type Event struct {
	SchemaVersion  int            `json:"schema_version"`
	EventID        string         `json:"event_id"`
	HostID         string         `json:"host_id"`
	BootID         string         `json:"boot_id"`
	Source         string         `json:"source"`
	SourceCursor   string         `json:"source_cursor"`
	ObservedAt     time.Time      `json:"observed_at"`
	ReceivedAt     time.Time      `json:"received_at"`
	Kind           string         `json:"kind"`
	Authentication Authentication `json:"authentication"`
}

// Identity uses length prefixes, so concatenation cannot alias distinct fields.
func Identity(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(part)))
		h.Write(n[:])
		h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ValidHost(host string) bool { return hostPattern.MatchString(host) }

func clean(s string, limit int) bool {
	if len(s) == 0 || len(s) > limit {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

func (e Event) Validate() error {
	if e.SchemaVersion != 1 || e.Source != "journald" || e.Kind != "ssh.authentication" {
		return fmt.Errorf("unsupported event schema or kind")
	}
	if !ValidHost(e.HostID) || !bootPattern.MatchString(e.BootID) || !clean(e.SourceCursor, 4096) {
		return fmt.Errorf("invalid host, boot or source cursor")
	}
	if e.EventID != Identity(e.HostID, e.BootID, e.SourceCursor) {
		return fmt.Errorf("event identity does not match its source")
	}
	if e.ObservedAt.IsZero() || e.ReceivedAt.IsZero() || e.ObservedAt.Year() < 1970 || e.ObservedAt.Year() > 9999 {
		return fmt.Errorf("invalid event timestamp")
	}
	a := e.Authentication
	if a.Outcome != "failed" && a.Outcome != "accepted" {
		return fmt.Errorf("invalid authentication outcome")
	}
	if a.Method != "password" && a.Method != "publickey" {
		return fmt.Errorf("unsupported authentication method")
	}
	if !clean(a.User, 256) || strings.ContainsAny(a.User, " \t") || a.SourcePort == 0 {
		return fmt.Errorf("invalid authentication user or port")
	}
	ip, err := netip.ParseAddr(a.SourceIP)
	if err != nil || ip.Zone() != "" || ip.String() != a.SourceIP {
		return fmt.Errorf("invalid or noncanonical source IP")
	}
	if a.Outcome == "accepted" && a.InvalidUser {
		return fmt.Errorf("invalid user cannot authenticate successfully")
	}
	return nil
}
