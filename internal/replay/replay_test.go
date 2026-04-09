package replay

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/detection"
	"watchhouse/internal/telemetry"
)

func now() time.Time { return time.Unix(1775642500, 0).UTC() }

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../../tests/fixtures/ssh-sequence.journal.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFixtureEndToEnd(t *testing.T) {
	var out bytes.Buffer
	stats, err := Run(bytes.NewReader(fixture(t)), &out, "lab-1", detection.DefaultConfig(), now)
	if err != nil || stats != (Stats{Records: 8, Matched: 7, Unmatched: 1, Findings: 1, Complete: true}) {
		t.Fatalf("stats %+v error %v", stats, err)
	}
	decoder := json.NewDecoder(&out)
	ids := make(map[string]bool)
	var finding *detection.Finding
	for {
		var item Output
		if err := decoder.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		switch item.Type {
		case "event":
			if item.Event == nil || item.Event.Validate() != nil {
				t.Fatal("invalid event output")
			}
			ids[item.Event.EventID] = true
		case "finding":
			finding = item.Finding
		default:
			t.Fatalf("unexpected type %q", item.Type)
		}
	}
	if finding == nil || len(finding.Evidence) != 6 {
		t.Fatal("missing finding evidence")
	}
	for _, id := range finding.Evidence {
		if !ids[id] {
			t.Fatalf("unresolvable evidence %s", id)
		}
	}
}

func TestFailFastAndPartialStats(t *testing.T) {
	b := append(fixture(t), []byte("{bad-json}\n")...)
	var out bytes.Buffer
	stats, err := Run(bytes.NewReader(b), &out, "lab-1", detection.DefaultConfig(), now)
	if err == nil || stats.Complete || stats.Records != 9 || stats.Findings != 1 {
		t.Fatalf("bad partial stats: %+v %v", stats, err)
	}
	if strings.Contains(err.Error(), "bad-json") {
		t.Fatal("raw input leaked into error")
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("read failure") }

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failure") }

func TestIOAndConfigurationErrors(t *testing.T) {
	for _, test := range []struct {
		in     io.Reader
		out    io.Writer
		host   string
		config detection.Config
		clock  func() time.Time
	}{
		{brokenReader{}, io.Discard, "lab-1", detection.DefaultConfig(), now},
		{bytes.NewReader(fixture(t)), brokenWriter{}, "lab-1", detection.DefaultConfig(), now},
		{strings.NewReader(strings.Repeat("x", telemetry.MaxRecordBytes+1)), io.Discard, "lab-1", detection.DefaultConfig(), now},
		{strings.NewReader(""), io.Discard, "bad host", detection.DefaultConfig(), now},
		{strings.NewReader(""), io.Discard, "lab-1", detection.Config{}, now},
		{strings.NewReader(""), io.Discard, "lab-1", detection.DefaultConfig(), nil},
	} {
		stats, err := Run(test.in, test.out, test.host, test.config, test.clock)
		if err == nil || stats.Complete {
			t.Fatal("failure claimed complete")
		}
	}
}

func TestEmptyInputAndDuplicateReplay(t *testing.T) {
	stats, err := Run(strings.NewReader(""), io.Discard, "lab-1", detection.DefaultConfig(), now)
	if err != nil || !stats.Complete || stats.Records != 0 {
		t.Fatal("empty input failed")
	}
	b := fixture(t)
	b = append(b, b...)
	stats, err = Run(bytes.NewReader(b), io.Discard, "lab-1", detection.DefaultConfig(), now)
	if err != nil || stats.Findings != 1 || !stats.Complete {
		t.Fatalf("duplicate replay %+v %v", stats, err)
	}
}
