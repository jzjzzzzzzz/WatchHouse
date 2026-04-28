package listenerpolicy

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"time"

	"watchhouse/internal/hostview"
)

type Finding struct {
	FindingID        string             `json:"finding_id"`
	Conclusion       string             `json:"conclusion"`
	PolicyID         string             `json:"policy_id"`
	ResourceID       string             `json:"resource_id,omitempty"`
	BootID           string             `json:"boot_id"`
	NetworkNamespace string             `json:"network_namespace"`
	ObservedAt       time.Time          `json:"observed_at"`
	Expected         *Declaration       `json:"expected,omitempty"`
	Actual           *hostview.Listener `json:"actual,omitempty"`
	EvidenceScope    string             `json:"evidence_scope"`
}

type Evaluation struct {
	Type             string    `json:"type"`
	PolicyID         string    `json:"policy_id"`
	BootID           string    `json:"boot_id"`
	NetworkNamespace string    `json:"network_namespace"`
	ObservedAt       time.Time `json:"observed_at"`
	Observed         int       `json:"observed_listeners"`
	Declared         int       `json:"declared_listeners"`
	Findings         []Finding `json:"findings"`
}

func Evaluate(policy Policy, snapshot hostview.HostSnapshot) (Evaluation, error) {
	var result Evaluation
	if err := policy.Validate(); err != nil {
		return result, err
	}
	if snapshot.Type != "tcp_listener_snapshot" || snapshot.BootID == "" || snapshot.NetworkNamespace == "" || snapshot.ObservedAt.IsZero() {
		return result, fmt.Errorf("invalid listener snapshot identity")
	}
	result = Evaluation{Type: "listener_policy_evaluation", PolicyID: policy.PolicyID,
		BootID: snapshot.BootID, NetworkNamespace: snapshot.NetworkNamespace,
		ObservedAt: snapshot.ObservedAt, Observed: len(snapshot.Listeners), Declared: len(policy.Listeners)}
	declarations := make(map[string]Declaration, len(policy.Listeners))
	seen := make(map[string]bool, len(policy.Listeners))
	for _, declaration := range policy.Listeners {
		declarations[endpointDeclaration(declaration)] = declaration
	}
	for index := range snapshot.Listeners {
		listener := snapshot.Listeners[index]
		if listener.Family != "ipv4" && listener.Family != "ipv6" || listener.LocalAddress == "" || listener.LocalPort == 0 || listener.Inode == 0 {
			return Evaluation{}, fmt.Errorf("invalid observed listener at index %d", index)
		}
		key := endpointListener(listener)
		declaration, declared := declarations[key]
		if !declared {
			actual := listener
			result.Findings = append(result.Findings, makeFinding(result, "unexpected_listener", "", nil, &actual))
			continue
		}
		seen[key] = true
		if len(declaration.AllowedUnits) == 0 {
			continue
		}
		if listener.Ownership != "attributed" {
			expected, actual := declaration, listener
			result.Findings = append(result.Findings, makeFinding(result, "ownership_unknown", declaration.ResourceID, &expected, &actual))
			continue
		}
		allowed := false
		for _, owner := range listener.Owners {
			for _, unit := range declaration.AllowedUnits {
				if owner.SystemdUnit == unit {
					allowed = true
					break
				}
			}
			if allowed {
				break
			}
		}
		if !allowed {
			expected, actual := declaration, listener
			result.Findings = append(result.Findings, makeFinding(result, "owner_mismatch", declaration.ResourceID, &expected, &actual))
		}
	}
	for index := range policy.Listeners {
		declaration := policy.Listeners[index]
		if !seen[endpointDeclaration(declaration)] {
			expected := declaration
			result.Findings = append(result.Findings, makeFinding(result, "missing_listener", declaration.ResourceID, &expected, nil))
		}
	}
	sort.Slice(result.Findings, func(i, j int) bool { return result.Findings[i].FindingID < result.Findings[j].FindingID })
	return result, nil
}

func makeFinding(base Evaluation, conclusion, resource string, expected *Declaration, actual *hostview.Listener) Finding {
	parts := []string{base.PolicyID, base.BootID, base.NetworkNamespace, conclusion, resource}
	if expected != nil {
		parts = append(parts, endpointDeclaration(*expected))
	}
	if actual != nil {
		parts = append(parts, endpointListener(*actual), strconv.FormatUint(actual.Inode, 10))
	}
	return Finding{FindingID: evidenceID(parts...), Conclusion: conclusion, PolicyID: base.PolicyID,
		ResourceID: resource, BootID: base.BootID, NetworkNamespace: base.NetworkNamespace,
		ObservedAt: base.ObservedAt, Expected: expected, Actual: actual,
		EvidenceScope: "current network namespace listener state; reachability and port publication not evaluated"}
}

func endpointDeclaration(value Declaration) string {
	return fmt.Sprintf("%s|%s|%d", value.Family, value.LocalAddress, value.LocalPort)
}
func endpointListener(value hostview.Listener) string {
	return fmt.Sprintf("%s|%s|%d", value.Family, value.LocalAddress, value.LocalPort)
}

func evidenceID(parts ...string) string {
	digest := sha256.New()
	var length [4]byte
	for _, part := range parts {
		binary.BigEndian.PutUint32(length[:], uint32(len(part)))
		digest.Write(length[:])
		digest.Write([]byte(part))
	}
	return fmt.Sprintf("%x", digest.Sum(nil))
}
