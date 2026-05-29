package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"watchhouse/internal/hostview"
	"watchhouse/internal/spool"
)

func reportSnapshot() hostview.HostSnapshot {
	return hostview.HostSnapshot{Type: "tcp_listener_snapshot", ObservedAt: time.Unix(1770000000, 0).UTC(),
		BootID: "12345678-1234-1234-1234-123456789abc", NetworkNamespace: "net:[4026531840]",
		Listeners: []hostview.Listener{{Socket: hostview.Socket{Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 443, KernelUID: 1000, Inode: 42}, Ownership: "unknown_unmapped"}}}
}

func TestReportListenersQueuesBeforeTransportFailure(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	var out, errOut bytes.Buffer
	code := runReportListeners([]string{"--host", "host-1", "--state", state, "--endpoint", "https://127.0.0.1:1",
		"--ca", "/missing/ca", "--cert", "/missing/cert", "--key", "/missing/key", "--server-name", "control.test"},
		&out, &errOut, func() (hostview.HostSnapshot, error) { return reportSnapshot(), nil })
	if code != 1 {
		t.Fatalf("exit %d output %q error %q", code, out.String(), errOut.String())
	}
	store, err := spool.Open(context.Background(), state, spool.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.PeekListenerSnapshots(context.Background(), 1, 1024*1024)
	if err != nil || len(items) != 1 || items[0].HostID != "host-1" {
		t.Fatalf("retained listener snapshot %+v error %v", items, err)
	}
}
