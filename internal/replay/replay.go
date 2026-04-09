// Package replay processes bounded journal records without trusting a fixture
// to be authenticated host telemetry.
package replay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"watchhouse/internal/detection"
	"watchhouse/internal/telemetry"
)

type Stats struct {
	Records   int  `json:"records"`
	Matched   int  `json:"matched"`
	Unmatched int  `json:"unmatched"`
	Findings  int  `json:"findings"`
	Complete  bool `json:"complete"`
}

type Output struct {
	Type    string             `json:"type"`
	Event   *telemetry.Event   `json:"event,omitempty"`
	Finding *detection.Finding `json:"finding,omitempty"`
}

// Run is fail-fast. Already-emitted lines remain valid, but cannot be treated
// as a complete report when an error or incomplete stats is returned.
func Run(in io.Reader, out io.Writer, host string, config detection.Config, clock func() time.Time) (Stats, error) {
	var stats Stats
	if !telemetry.ValidHost(host) || clock == nil {
		return stats, fmt.Errorf("host and clock required")
	}
	detector, err := detection.NewSSH(config)
	if err != nil {
		return stats, err
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), telemetry.MaxRecordBytes+1)
	encoder := json.NewEncoder(out)
	for scanner.Scan() {
		stats.Records++
		e, matched, err := telemetry.ParseJournal(scanner.Bytes(), host, clock())
		if err != nil {
			return stats, fmt.Errorf("record %d: %w", stats.Records, err)
		}
		if !matched {
			stats.Unmatched++
			continue
		}
		stats.Matched++
		finding, err := detector.Observe(e)
		if err != nil {
			return stats, fmt.Errorf("record %d: %w", stats.Records, err)
		}
		if err := encoder.Encode(Output{Type: "event", Event: &e}); err != nil {
			return stats, fmt.Errorf("event output: %w", err)
		}
		if finding != nil {
			if err := encoder.Encode(Output{Type: "finding", Finding: finding}); err != nil {
				return stats, fmt.Errorf("finding output: %w", err)
			}
			stats.Findings++
		}
	}
	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("journal input: %w", err)
	}
	stats.Complete = true
	return stats, nil
}
