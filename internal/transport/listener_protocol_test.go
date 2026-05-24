package transport

import (
	"testing"
	"time"

	"watchhouse/internal/hostview"
)

func validListenerSnapshot() hostview.HostSnapshot {
	return hostview.HostSnapshot{
		Type: "tcp_listener_snapshot", ObservedAt: time.Unix(1770000000, 0).UTC(),
		BootID: "12345678-1234-1234-1234-123456789abc", NetworkNamespace: "net:[4026531840]",
		Listeners: []hostview.Listener{{Socket: hostview.Socket{Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 443, KernelUID: 1000, Inode: 42},
			Ownership: "attributed", Owners: []hostview.ProcessIdentity{{PID: 7, StartTimeTicks: 99, EffectiveUID: 1000, Comm: "nginx", SystemdUnit: "nginx.service"}}}},
		Quality: hostview.Quality{ProcessDirectories: 1, ProcessesScanned: 1},
	}
}

func TestListenerSnapshotIdentityBindsHostAndKernelContext(t *testing.T) {
	snapshot := validListenerSnapshot()
	request := ListenerSnapshotRequest{SchemaVersion: 1, Snapshot: snapshot, SnapshotID: ListenerSnapshotID("host-1", snapshot)}
	if err := request.Validate("host-1"); err != nil {
		t.Fatal(err)
	}
	if err := request.Validate("host-2"); err == nil {
		t.Fatal("snapshot identity replayed under another host")
	}
	request.Snapshot.NetworkNamespace = "net:[42]"
	if err := request.Validate("host-1"); err == nil {
		t.Fatal("changed namespace accepted under old identity")
	}
}
