package firewall

import "time"

const (
	MaxRulesetBytes = 4 << 20
	MaxEntries      = 100_000
)

// Snapshot is a deliberately lossy inventory of an nftables ruleset. It does
// not claim that a port is reachable: packet traversal also depends on network
// namespaces, routing, conntrack, and upstream controls.
type Snapshot struct {
	SchemaVersion int         `json:"schema_version"`
	ObservedAt    time.Time   `json:"observed_at"`
	SHA256        string      `json:"sha256"`
	Tables        []Table     `json:"tables"`
	BaseChains    []BaseChain `json:"base_chains"`
	RuleCount     int         `json:"rule_count"`
	SetCount      int         `json:"set_count"`
	MapCount      int         `json:"map_count"`
	OtherCount    int         `json:"other_count"`
}

type Table struct {
	Family string `json:"family"`
	Name   string `json:"name"`
}

type BaseChain struct {
	Family   string `json:"family"`
	Table    string `json:"table"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Hook     string `json:"hook"`
	Priority string `json:"priority"`
	Policy   string `json:"policy"`
}
