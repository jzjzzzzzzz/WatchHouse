package listenerpolicy

import (
	"testing"
	"time"

	"watchhouse/internal/hostview"
)

func listener(address string, port uint16, inode uint64, ownership, unit string) hostview.Listener {
	value := hostview.Listener{Socket: hostview.Socket{Family: "ipv4", LocalAddress: address, LocalPort: port, KernelUID: 0, Inode: inode}, Ownership: ownership}
	if ownership == "attributed" {
		value.Owners = []hostview.ProcessIdentity{{PID: int(inode), StartTimeTicks: inode + 1, EffectiveUID: 0, Comm: "fixture", SystemdUnit: unit}}
	}
	return value
}

func TestEvaluateAllListenerConclusions(t *testing.T) {
	policy := Policy{SchemaVersion: 1, PolicyID: "edge-v1", Listeners: []Declaration{
		{ResourceID: "good", Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 80, AllowedUnits: []string{"nginx.service"}},
		{ResourceID: "wrong-owner", Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 81, AllowedUnits: []string{"nginx.service"}},
		{ResourceID: "unknown-owner", Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 82, AllowedUnits: []string{"nginx.service"}},
		{ResourceID: "missing", Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 83},
	}}
	snapshot := hostview.HostSnapshot{Type: "tcp_listener_snapshot", BootID: "boot", NetworkNamespace: "net:[1]", ObservedAt: time.Unix(1, 0).UTC(), Listeners: []hostview.Listener{
		listener("127.0.0.1", 80, 1, "attributed", "nginx.service"),
		listener("127.0.0.1", 81, 2, "attributed", "other.service"),
		listener("127.0.0.1", 82, 3, "unknown_permission", ""),
		listener("0.0.0.0", 9000, 4, "attributed", "rogue.service"),
	}}
	got, err := Evaluate(policy, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	conclusions := map[string]int{}
	for _, finding := range got.Findings {
		conclusions[finding.Conclusion]++
	}
	for _, conclusion := range []string{"owner_mismatch", "ownership_unknown", "missing_listener", "unexpected_listener"} {
		if conclusions[conclusion] != 1 {
			t.Fatalf("%s findings: %+v", conclusion, got.Findings)
		}
	}
	if len(got.Findings) != 4 {
		t.Fatalf("findings: %+v", got.Findings)
	}
	again, err := Evaluate(policy, snapshot)
	if err != nil || again.Findings[0].FindingID != got.Findings[0].FindingID {
		t.Fatal("finding identity is unstable")
	}
}

func TestEndpointOnlyDeclarationAcceptsAnyAttributedOwner(t *testing.T) {
	policy := Policy{SchemaVersion: 1, PolicyID: "endpoint", Listeners: []Declaration{{ResourceID: "service", Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 80}}}
	snapshot := hostview.HostSnapshot{Type: "tcp_listener_snapshot", BootID: "boot", NetworkNamespace: "net:[1]", ObservedAt: time.Now(), Listeners: []hostview.Listener{listener("127.0.0.1", 80, 1, "unknown_permission", "")}}
	got, err := Evaluate(policy, snapshot)
	if err != nil || len(got.Findings) != 0 {
		t.Fatalf("evaluation %+v %v", got, err)
	}
}

func TestInvalidSnapshotRejected(t *testing.T) {
	policy := Policy{SchemaVersion: 1, PolicyID: "empty"}
	for _, snapshot := range []hostview.HostSnapshot{{}, {Type: "tcp_listener_snapshot", BootID: "boot", NetworkNamespace: "net:[1]", ObservedAt: time.Now(), Listeners: []hostview.Listener{{Socket: hostview.Socket{Family: "ipv4"}}}}} {
		if _, err := Evaluate(policy, snapshot); err == nil {
			t.Fatal("invalid snapshot accepted")
		}
	}
}
