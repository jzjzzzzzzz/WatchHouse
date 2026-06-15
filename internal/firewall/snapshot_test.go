package firewall

import (
	"strings"
	"testing"
	"time"
)

func TestSnapshotJSONContractFields(t *testing.T) {
	var _ = Snapshot{SchemaVersion: 1, Tables: []Table{}, BaseChains: []BaseChain{}}
	var _ = BaseChain{Family: "inet", Table: "filter", Name: "input", Type: "filter", Hook: "input", Priority: "0", Policy: "drop"}
}

func TestParseSummarizesAndSortsRuleset(t *testing.T) {
	raw := []byte(`{"nftables":[
      {"metainfo":{"version":"1.0.9"}},
      {"table":{"family":"inet","name":"z"}},
      {"table":{"family":"inet","name":"a"}},
      {"chain":{"family":"inet","table":"a","name":"forward","type":"filter","hook":"forward","prio":0,"policy":"drop"}},
      {"chain":{"family":"inet","table":"a","name":"regular"}},
      {"rule":{"family":"inet","table":"a","chain":"forward","expr":[]}},
      {"set":{"family":"inet","table":"a","name":"blocked"}},
      {"map":{"family":"inet","table":"a","name":"ports"}},
      {"counter":{"family":"inet","table":"a","name":"hits"}}
    ]}`)
	at := time.Date(2026, 10, 5, 20, 1, 2, 0, time.FixedZone("EDT", -4*3600))
	got, err := Parse(raw, at)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || got.ObservedAt.Location() != time.UTC {
		t.Fatalf("metadata: %#v", got)
	}
	if len(got.Tables) != 2 || got.Tables[0].Name != "a" {
		t.Fatalf("tables: %#v", got.Tables)
	}
	if len(got.BaseChains) != 1 || got.BaseChains[0].Priority != "0" || got.BaseChains[0].Policy != "drop" {
		t.Fatalf("chains: %#v", got.BaseChains)
	}
	if got.RuleCount != 1 || got.SetCount != 1 || got.MapCount != 1 || got.OtherCount != 1 {
		t.Fatalf("counts: %#v", got)
	}
	if len(got.SHA256) != 64 {
		t.Fatalf("digest: %q", got.SHA256)
	}
}

func TestParseRejectsUnsafeOrAmbiguousInput(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"empty", "", "output size"},
		{"unknown root", `{"nftables":[],"extra":1}`, "unknown field"},
		{"multiple kinds", `{"nftables":[{"rule":{},"set":{}}]}`, "exactly one"},
		{"missing family", `{"nftables":[{"table":{"name":"filter"}}]}`, "family"},
		{"control", "{\"nftables\":[{\"table\":{\"family\":\"inet\",\"name\":\"bad\\u0001\"}}]}", "control"},
		{"fractional priority", `{"nftables":[{"chain":{"family":"inet","table":"filter","name":"input","hook":"input","prio":0.5}}]}`, "integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input), time.Now())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err=%v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseRejectsOversize(t *testing.T) {
	_, err := Parse(make([]byte, MaxRulesetBytes+1), time.Now())
	if err == nil {
		t.Fatal("expected size error")
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	f.Add([]byte(`{"nftables":[]}`))
	f.Add([]byte(`{"nftables":[{"chain":{"family":"inet","table":"filter","name":"input","hook":"input","prio":"filter"}}]}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxRulesetBytes+1 {
			t.Skip()
		}
		_, _ = Parse(raw, time.Unix(0, 0))
	})
}
