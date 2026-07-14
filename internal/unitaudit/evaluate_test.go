package unitaudit

import (
	"strings"
	"testing"
)

func hardenedUnit() []byte {
	return []byte(`LoadState=loaded
ActiveState=active
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
CapabilityBoundingSet=
`)
}

func TestParseHardenedUnit(t *testing.T) {
	got, err := Parse("watchhouse-control.service", hardenedUnit())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "watchhouse-control.service" || got.LoadState != "loaded" || got.ActiveState != "active" {
		t.Fatalf("%#v", got)
	}
	if len(got.Results) != len(properties)+1 {
		t.Fatalf("results=%d", len(got.Results))
	}
	for _, result := range got.Results {
		if result.Status != Pass {
			t.Fatalf("%#v", result)
		}
	}
}

func TestParseDistinguishesFailureAndMissingEvidence(t *testing.T) {
	raw := strings.Replace(string(hardenedUnit()), "ProtectSystem=strict", "ProtectSystem=full", 1)
	raw = strings.Replace(raw, "PrivateTmp=yes\n", "", 1)
	got, err := Parse("watchhouse-control.service", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Result{}
	for _, result := range got.Results {
		byName[result.Property] = result
	}
	if byName["ProtectSystem"].Status != Fail || byName["ProtectSystem"].Observed != "full" {
		t.Fatalf("%#v", byName["ProtectSystem"])
	}
	if byName["PrivateTmp"].Status != Error {
		t.Fatalf("%#v", byName["PrivateTmp"])
	}
}

func TestParseRejectsMalformedSystemctlOutput(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte("bad\n"), []byte("LoadState=loaded\nLoadState=masked\n"), make([]byte, MaxOutputBytes+1)} {
		if _, err := Parse("unit.service", raw); err == nil {
			t.Fatalf("accepted %d bytes", len(raw))
		}
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	f.Add(hardenedUnit())
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxOutputBytes+1 {
			t.Skip()
		}
		_, _ = Parse("unit.service", raw)
	})
}
