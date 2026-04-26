package main

import (
	"bytes"
	"errors"
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
