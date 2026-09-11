package networkcompare

import (
	"net/netip"
	"sort"
	"time"

	"watchhouse/internal/dockerports"
	"watchhouse/internal/hostview"
)

type PublishedComparison struct {
	Binding           dockerports.Binding `json:"binding"`
	ListenerObserved  bool                `json:"listener_observed"`
	MatchingListeners []hostview.Listener `json:"matching_listeners"`
	Interpretation    string              `json:"interpretation"`
}

type Report struct {
	SchemaVersion         int                   `json:"schema_version"`
	ObservedAt            time.Time             `json:"observed_at"`
	ListenerNamespace     string                `json:"listener_namespace"`
	Published             []PublishedComparison `json:"published"`
	UnmatchedTCPListeners []hostview.Listener   `json:"unmatched_tcp_listeners"`
	Caveat                string                `json:"caveat"`
}

func Compare(listeners hostview.HostSnapshot, docker dockerports.Report, at time.Time) Report {
	report := Report{SchemaVersion: 1, ObservedAt: at.UTC(), ListenerNamespace: listeners.NetworkNamespace, Published: []PublishedComparison{}, UnmatchedTCPListeners: []hostview.Listener{}, Caveat: "absence of a userspace listener can be normal for kernel NAT; this is correlation, not reachability"}
	matched := map[int]bool{}
	for _, binding := range docker.Bindings {
		item := PublishedComparison{Binding: binding, MatchingListeners: []hostview.Listener{}, Interpretation: "no userspace TCP listener observed; Docker kernel NAT may still publish this endpoint"}
		if binding.Protocol == "tcp" {
			for index, listener := range listeners.Listeners {
				if listener.LocalPort == binding.HostPort && addressMatches(binding.HostIP, listener.LocalAddress) {
					item.MatchingListeners = append(item.MatchingListeners, listener)
					matched[index] = true
				}
			}
		}
		if len(item.MatchingListeners) > 0 {
			item.ListenerObserved = true
			item.Interpretation = "same-port compatible TCP listener observed in the sampled namespace"
		}
		report.Published = append(report.Published, item)
	}
	for index, listener := range listeners.Listeners {
		if !matched[index] {
			report.UnmatchedTCPListeners = append(report.UnmatchedTCPListeners, listener)
		}
	}
	sort.Slice(report.Published, func(i, j int) bool {
		a, b := report.Published[i].Binding, report.Published[j].Binding
		if a.HostPort != b.HostPort {
			return a.HostPort < b.HostPort
		}
		return a.HostIP < b.HostIP
	})
	return report
}

func addressMatches(binding, listener string) bool {
	b, bErr := netip.ParseAddr(binding)
	l, lErr := netip.ParseAddr(listener)
	if bErr != nil || lErr != nil {
		return false
	}
	if b.Is4() != l.Is4() {
		return false
	}
	return b.IsUnspecified() || l.IsUnspecified() || b == l
}
