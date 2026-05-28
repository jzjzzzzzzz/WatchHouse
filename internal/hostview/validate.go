package hostview

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	"watchhouse/internal/telemetry"
)

// Validate treats a snapshot as untrusted wire input. Collection already has
// bounds, but control-plane ingestion must independently enforce them.
func (snapshot HostSnapshot) Validate() error {
	if snapshot.Type != "tcp_listener_snapshot" || snapshot.ObservedAt.IsZero() || snapshot.ObservedAt.Year() < 1970 || snapshot.ObservedAt.Year() > 9999 ||
		!bootIDPattern.MatchString(snapshot.BootID) || !netNSPattern.MatchString(snapshot.NetworkNamespace) || len(snapshot.Listeners) > 2*maxSockets {
		return fmt.Errorf("invalid listener snapshot identity or bounds")
	}
	quality := snapshot.Quality
	if quality.ProcessDirectories < 0 || quality.ProcessDirectories > maxPIDs || quality.ProcessesScanned < 0 || quality.ProcessesScanned > maxPIDs ||
		quality.PermissionDenied < 0 || quality.PermissionDenied > maxPIDs || quality.Vanished < 0 || quality.Vanished > maxPIDs ||
		quality.Malformed < 0 || quality.Malformed > maxPIDs || quality.FDTruncated < 0 || quality.FDTruncated > maxPIDs ||
		quality.ProcessesScanned > quality.ProcessDirectories {
		return fmt.Errorf("invalid listener snapshot quality")
	}
	for index := range snapshot.Listeners {
		listener := snapshot.Listeners[index]
		if err := validateListener(listener); err != nil {
			return fmt.Errorf("invalid listener %d: %w", index, err)
		}
		if index > 0 && !listenerLess(snapshot.Listeners[index-1], listener) {
			return fmt.Errorf("listeners are not in strict canonical order")
		}
	}
	return nil
}

func SnapshotIdentity(host string, snapshot HostSnapshot) string {
	return telemetry.Identity(host, snapshot.BootID, snapshot.NetworkNamespace, snapshot.ObservedAt.UTC().Format(time.RFC3339Nano))
}

func validateListener(listener Listener) error {
	address, err := netip.ParseAddr(listener.LocalAddress)
	if err != nil || address.Zone() != "" || address.String() != listener.LocalAddress || listener.LocalPort == 0 || listener.Inode == 0 ||
		(listener.Family != "ipv4" && listener.Family != "ipv6") || (listener.Family == "ipv4") != address.Is4() || len(listener.Owners) > maxPIDs {
		return fmt.Errorf("invalid socket identity")
	}
	if listener.Ownership != "attributed" && listener.Ownership != "unknown_permission" && listener.Ownership != "unknown_partial" && listener.Ownership != "unknown_unmapped" {
		return fmt.Errorf("invalid ownership quality")
	}
	if (listener.Ownership == "attributed") != (len(listener.Owners) > 0) {
		return fmt.Errorf("ownership contradicts owners")
	}
	for index, owner := range listener.Owners {
		if owner.PID < 1 || owner.PID > 1<<30 || owner.StartTimeTicks == 0 || len(owner.Comm) < 1 || len(owner.Comm) > 64 || hasControl(owner.Comm) ||
			(owner.SystemdUnit != "" && (!strings.HasSuffix(owner.SystemdUnit, ".service") || len(owner.SystemdUnit) > 256 || hasControl(owner.SystemdUnit))) {
			return fmt.Errorf("invalid process owner")
		}
		if index > 0 && !ownerLess(listener.Owners[index-1], owner) {
			return fmt.Errorf("owners are not in strict canonical order")
		}
	}
	return nil
}

func listenerLess(a, b Listener) bool {
	if a.Family != b.Family {
		return a.Family < b.Family
	}
	if a.LocalAddress != b.LocalAddress {
		return a.LocalAddress < b.LocalAddress
	}
	if a.LocalPort != b.LocalPort {
		return a.LocalPort < b.LocalPort
	}
	return a.Inode < b.Inode
}

func ownerLess(a, b ProcessIdentity) bool {
	return a.PID < b.PID || (a.PID == b.PID && a.StartTimeTicks < b.StartTimeTicks)
}
