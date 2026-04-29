// Package listenerpolicy validates operator declarations and evaluates bounded
// TCP listener evidence without making reachability claims.
package listenerpolicy

import (
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strings"

	"watchhouse/internal/strictjson"
)

const (
	SchemaVersion  = 1
	maxPolicyBytes = 128 * 1024
	maxListeners   = 256
	maxUnits       = 16
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Policy struct {
	SchemaVersion int           `json:"schema_version"`
	PolicyID      string        `json:"policy_id"`
	Listeners     []Declaration `json:"listeners"`
}

type Declaration struct {
	ResourceID   string   `json:"resource_id"`
	Family       string   `json:"family"`
	LocalAddress string   `json:"local_address"`
	LocalPort    uint16   `json:"local_port"`
	AllowedUnits []string `json:"allowed_units"`
}

func Decode(input io.Reader) (Policy, error) {
	result, err := strictjson.Decode[Policy](input, maxPolicyBytes)
	if err != nil {
		return Policy{}, fmt.Errorf("decode listener policy: %w", err)
	}
	if err := result.Validate(); err != nil {
		return Policy{}, err
	}
	return result, nil
}

func (policy Policy) Validate() error {
	if policy.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported listener policy schema version")
	}
	if !identifierPattern.MatchString(policy.PolicyID) {
		return fmt.Errorf("invalid listener policy ID")
	}
	if len(policy.Listeners) > maxListeners {
		return fmt.Errorf("listener policy exceeds %d declarations", maxListeners)
	}
	resources := map[string]struct{}{}
	endpoints := map[string]struct{}{}
	for index, declaration := range policy.Listeners {
		if !identifierPattern.MatchString(declaration.ResourceID) {
			return fmt.Errorf("listener declaration %d has invalid resource ID", index)
		}
		if _, exists := resources[declaration.ResourceID]; exists {
			return fmt.Errorf("duplicate listener resource ID")
		}
		resources[declaration.ResourceID] = struct{}{}
		if declaration.Family != "ipv4" && declaration.Family != "ipv6" {
			return fmt.Errorf("listener declaration %d has invalid family", index)
		}
		address, err := netip.ParseAddr(declaration.LocalAddress)
		if err != nil || address.String() != declaration.LocalAddress || (declaration.Family == "ipv4") != address.Is4() {
			return fmt.Errorf("listener declaration %d has noncanonical or mismatched address", index)
		}
		if declaration.LocalPort == 0 {
			return fmt.Errorf("listener declaration %d has invalid port", index)
		}
		endpoint := fmt.Sprintf("%s|%s|%d", declaration.Family, declaration.LocalAddress, declaration.LocalPort)
		if _, exists := endpoints[endpoint]; exists {
			return fmt.Errorf("duplicate listener endpoint")
		}
		endpoints[endpoint] = struct{}{}
		if len(declaration.AllowedUnits) > maxUnits {
			return fmt.Errorf("listener declaration %d exceeds allowed-unit bound", index)
		}
		units := map[string]struct{}{}
		for _, unit := range declaration.AllowedUnits {
			if len(unit) <= len(".service") || len(unit) > 256 || !strings.HasSuffix(unit, ".service") || strings.Contains(unit, "/") || hasControl(unit) {
				return fmt.Errorf("listener declaration %d has invalid systemd unit", index)
			}
			if _, exists := units[unit]; exists {
				return fmt.Errorf("listener declaration %d repeats an allowed unit", index)
			}
			units[unit] = struct{}{}
		}
	}
	return nil
}

func hasControl(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}
