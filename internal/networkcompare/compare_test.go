package networkcompare

import (
	"strings"
	"testing"
	"time"

	"watchhouse/internal/dockerports"
	"watchhouse/internal/hostview"
)

func TestCompareMatchesCompatibleTCPBindings(t *testing.T) {
	listeners := hostview.HostSnapshot{NetworkNamespace: "net:[42]", Listeners: []hostview.Listener{
		{Socket: hostview.Socket{Family: "ipv4", LocalAddress: "0.0.0.0", LocalPort: 8080, Inode: 1}},
		{Socket: hostview.Socket{Family: "ipv6", LocalAddress: "::1", LocalPort: 8443, Inode: 2}},
		{Socket: hostview.Socket{Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 22, Inode: 3}},
	}}
	docker := dockerports.Report{Bindings: []dockerports.Binding{
		{Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80},
		{Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8443, ContainerPort: 443},
	}}
	got := Compare(listeners, docker, time.Now())
	if !got.Published[0].ListenerObserved || len(got.Published[0].MatchingListeners) != 1 {
		t.Fatalf("%#v", got.Published[0])
	}
	if got.Published[1].ListenerObserved {
		t.Fatalf("cross-family match: %#v", got.Published[1])
	}
	if len(got.UnmatchedTCPListeners) != 2 || got.ListenerNamespace != "net:[42]" {
		t.Fatalf("%#v", got)
	}
}

func TestCompareDoesNotTreatUDPAsTCPListener(t *testing.T) {
	listeners := hostview.HostSnapshot{Listeners: []hostview.Listener{{Socket: hostview.Socket{Family: "ipv4", LocalAddress: "0.0.0.0", LocalPort: 53, Inode: 1}}}}
	docker := dockerports.Report{Bindings: []dockerports.Binding{{Protocol: "udp", HostIP: "0.0.0.0", HostPort: 53, ContainerPort: 53}}}
	got := Compare(listeners, docker, time.Now())
	if got.Published[0].ListenerObserved || !strings.Contains(got.Published[0].Interpretation, "kernel NAT") {
		t.Fatalf("%#v", got.Published[0])
	}
}

func TestAddressMatchesWildcardWithinFamily(t *testing.T) {
	for _, test := range []struct {
		binding, listener string
		want              bool
	}{
		{"0.0.0.0", "127.0.0.1", true}, {"127.0.0.1", "0.0.0.0", true}, {"::", "::1", true},
		{"0.0.0.0", "::", false}, {"127.0.0.1", "192.0.2.1", false}, {"bad", "0.0.0.0", false},
	} {
		if got := addressMatches(test.binding, test.listener); got != test.want {
			t.Fatalf("%s %s got=%v", test.binding, test.listener, got)
		}
	}
}
