package firewall

import "testing"

func TestSnapshotJSONContractFields(t *testing.T) {
	var _ = Snapshot{SchemaVersion: 1, Tables: []Table{}, BaseChains: []BaseChain{}}
	var _ = BaseChain{Family: "inet", Table: "filter", Name: "input", Type: "filter", Hook: "input", Priority: "0", Policy: "drop"}
}
