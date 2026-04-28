package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/hostview"
)

func TestListenersCommandEmitsBoundedSnapshot(t *testing.T) {
	want := hostview.HostSnapshot{Type: "tcp_listener_snapshot", ObservedAt: time.Unix(1, 0).UTC(), BootID: "synthetic", NetworkNamespace: "net:[1]"}
	var out, errOut bytes.Buffer
	code := runListeners(nil, &out, &errOut, func() (hostview.HostSnapshot, error) { return want, nil })
	if code != 0 || !strings.Contains(out.String(), `"network_namespace":"net:[1]"`) || errOut.Len() != 0 {
		t.Fatalf("code %d output %q error %q", code, out.String(), errOut.String())
	}
}

func TestListenerCheckEvaluatesStrictPolicy(t *testing.T) {
	path := t.TempDir() + "/policy.json"
	policy := `{"schema_version":1,"policy_id":"test","listeners":[{"resource_id":"expected","family":"ipv4","local_address":"127.0.0.1","local_port":443,"allowed_units":[]}]}`
	if err := os.WriteFile(path, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := hostview.HostSnapshot{Type: "tcp_listener_snapshot", BootID: "boot", NetworkNamespace: "net:[1]", ObservedAt: time.Unix(1, 0).UTC()}
	var out, errOut bytes.Buffer
	code := runListenerCheck([]string{"--policy", path}, &out, &errOut, func() (hostview.HostSnapshot, error) { return snapshot, nil })
	if code != 0 || !strings.Contains(out.String(), `"conclusion":"missing_listener"`) || errOut.Len() != 0 {
		t.Fatalf("code %d output %q error %q", code, out.String(), errOut.String())
	}
}

func TestListenerCheckRejectsMissingOrInvalidPolicy(t *testing.T) {
	called := false
	collector := func() (hostview.HostSnapshot, error) { called = true; return hostview.HostSnapshot{}, nil }
	if code := runListenerCheck(nil, ioDiscard{}, ioDiscard{}, collector); code != 2 || called {
		t.Fatal("missing policy reached collector")
	}
	path := t.TempDir() + "/bad.json"
	if err := os.WriteFile(path, []byte(`{"unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := runListenerCheck([]string{"--policy", path}, ioDiscard{}, ioDiscard{}, collector); code != 1 || called {
		t.Fatal("invalid policy reached collector")
	}
}

func TestListenersCommandRejectsInputsAndCollectionFailure(t *testing.T) {
	called := false
	collector := func() (hostview.HostSnapshot, error) { called = true; return hostview.HostSnapshot{}, nil }
	if code := runListeners([]string{"/proc/1"}, ioDiscard{}, ioDiscard{}, collector); code != 2 || called {
		t.Fatal("caller-selected proc path reached collector")
	}
	if code := runListeners(nil, ioDiscard{}, ioDiscard{}, func() (hostview.HostSnapshot, error) {
		return hostview.HostSnapshot{}, errors.New("synthetic failure")
	}); code != 1 {
		t.Fatal("collection failure did not fail command")
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(value []byte) (int, error) { return len(value), nil }
