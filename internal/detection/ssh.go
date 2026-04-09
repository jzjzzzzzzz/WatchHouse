// Package detection produces findings; it cannot perform response actions.
package detection

import (
	"errors"
	"fmt"
	"time"

	"watchhouse/internal/telemetry"
)

const SSHRule = "ssh.failures-followed-by-success.v1"

var (
	ErrOutOfOrder       = errors.New("out-of-order observation; ordered replay required")
	ErrCapacity         = errors.New("detection state capacity exceeded")
	ErrIdentityConflict = errors.New("same source event identity has conflicting content")
)

type Config struct {
	Window    time.Duration
	Threshold int
	MaxEvents int
}

func DefaultConfig() Config { return Config{Window: 5 * time.Minute, Threshold: 5, MaxEvents: 8192} }

type Finding struct {
	FindingID        string    `json:"finding_id"`
	RuleID           string    `json:"rule_id"`
	HostID           string    `json:"host_id"`
	BootID           string    `json:"boot_id"`
	User             string    `json:"user"`
	SourceIP         string    `json:"source_ip"`
	ObservedAt       time.Time `json:"observed_at"`
	Priority         string    `json:"priority"`
	Summary          string    `json:"summary"`
	WindowSeconds    int64     `json:"window_seconds"`
	FailureThreshold int       `json:"failure_threshold"`
	Evidence         []string  `json:"evidence_event_ids"`
}

type key struct{ host, boot, user, ip string }

// SSHDetector is a single-owner ordered stream processor. It is deliberately
// not goroutine-safe. MaxEvents bounds dedup state and pending evidence alike.
type SSHDetector struct {
	config    Config
	watermark time.Time
	seen      map[string]telemetry.Event
	failures  map[key][]telemetry.Event
}

func NewSSH(config Config) (*SSHDetector, error) {
	if config.Window < time.Second || config.Window > 24*time.Hour || config.Window%time.Second != 0 ||
		config.Threshold < 1 || config.MaxEvents < config.Threshold || config.MaxEvents > 1_000_000 {
		return nil, fmt.Errorf("invalid detection window, threshold or capacity")
	}
	return &SSHDetector{config: config, seen: make(map[string]telemetry.Event), failures: make(map[key][]telemetry.Event)}, nil
}

// Observe preserves the boundary inclusively: failures exactly Window before
// success count. Duplicates inside retained state are ignored, but conflicts
// and late records return explicit errors instead of silently losing evidence.
func (d *SSHDetector) Observe(e telemetry.Event) (*Finding, error) {
	if err := e.Validate(); err != nil {
		return nil, fmt.Errorf("invalid input event: %w", err)
	}
	if previous, exists := d.seen[e.EventID]; exists {
		if previous.HostID != e.HostID || previous.BootID != e.BootID || previous.SourceCursor != e.SourceCursor ||
			!previous.ObservedAt.Equal(e.ObservedAt) || previous.Authentication != e.Authentication {
			return nil, ErrIdentityConflict
		}
		return nil, nil
	}
	if !d.watermark.IsZero() && e.ObservedAt.Before(d.watermark) {
		return nil, ErrOutOfOrder
	}
	d.prune(e.ObservedAt.Add(-d.config.Window))
	if len(d.seen) >= d.config.MaxEvents {
		return nil, ErrCapacity
	}
	d.watermark = e.ObservedAt
	d.seen[e.EventID] = e
	a := e.Authentication
	k := key{e.HostID, e.BootID, a.User, a.SourceIP}
	if a.Outcome == "failed" {
		pending := append(d.failures[k], e)
		// Only the most recent threshold events are needed to establish this
		// rule. All retained IDs can be resolved back to normalized evidence.
		if len(pending) > d.config.Threshold {
			pending = pending[1:]
		}
		d.failures[k] = pending
		return nil, nil
	}
	pending := d.failures[k]
	delete(d.failures, k) // A legitimate success also ends a sub-threshold run.
	if len(pending) < d.config.Threshold {
		return nil, nil
	}
	evidence := make([]string, 0, len(pending)+1)
	for _, failure := range pending {
		evidence = append(evidence, failure.EventID)
	}
	evidence = append(evidence, e.EventID)
	return &Finding{
		FindingID: telemetry.Identity(SSHRule, fmt.Sprint(d.config.Window), fmt.Sprint(d.config.Threshold), e.EventID),
		RuleID:    SSHRule, HostID: e.HostID, BootID: e.BootID, User: a.User, SourceIP: a.SourceIP,
		ObservedAt: e.ObservedAt, Priority: "high", WindowSeconds: int64(d.config.Window / time.Second),
		FailureThreshold: d.config.Threshold, Evidence: evidence,
		Summary: "Repeated authentication failures followed by success; investigate, not proof of compromise.",
	}, nil
}

func (d *SSHDetector) prune(cutoff time.Time) {
	for id, e := range d.seen {
		if e.ObservedAt.Before(cutoff) {
			delete(d.seen, id)
		}
	}
	for k, pending := range d.failures {
		i := 0
		for i < len(pending) && pending[i].ObservedAt.Before(cutoff) {
			i++
		}
		if i == len(pending) {
			delete(d.failures, k)
		} else if i > 0 {
			d.failures[k] = pending[i:]
		}
	}
}
