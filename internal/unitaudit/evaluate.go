package unitaudit

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

const MaxOutputBytes = 256 << 10

var Units = []string{
	"watchhouse-collect.service",
	"watchhouse-control.service",
	"watchhouse-deliver.service",
	"watchhouse-probe.service",
	"watchhouse-probe-deliver.service",
}

type propertyControl struct {
	name, expected string
	accepted       map[string]bool
}

var properties = []propertyControl{
	{"NoNewPrivileges", "yes", map[string]bool{"yes": true}},
	{"ProtectSystem", "strict", map[string]bool{"strict": true}},
	{"ProtectHome", "yes, read-only, or tmpfs", map[string]bool{"yes": true, "read-only": true, "tmpfs": true}},
	{"PrivateTmp", "yes or disconnected", map[string]bool{"yes": true, "disconnected": true}},
	{"CapabilityBoundingSet", "empty", map[string]bool{"": true}},
}

func Parse(unit string, raw []byte) (UnitReport, error) {
	if unit == "" || len(raw) == 0 || len(raw) > MaxOutputBytes {
		return UnitReport{}, fmt.Errorf("systemd output for unit must be 1..%d bytes", MaxOutputBytes)
	}
	values := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || key == "" {
			return UnitReport{}, fmt.Errorf("malformed systemd property")
		}
		if _, exists := values[key]; exists {
			return UnitReport{}, fmt.Errorf("duplicate systemd property %q", key)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return UnitReport{}, err
	}
	report := UnitReport{Name: unit, LoadState: values["LoadState"], ActiveState: values["ActiveState"], Results: []Result{}}
	load := Result{Unit: unit, Property: "LoadState", Observed: report.LoadState, Expected: "loaded"}
	if report.LoadState == "loaded" {
		load.Status = Pass
	} else {
		load.Status, load.Detail = Error, "unit definition is not loaded"
	}
	report.Results = append(report.Results, load)
	for _, control := range properties {
		result := Result{Unit: unit, Property: control.name, Expected: control.expected}
		value, exists := values[control.name]
		if !exists {
			result.Status, result.Detail = Error, "property absent from systemctl output"
		} else if control.accepted[value] {
			result.Status, result.Observed = Pass, value
		} else {
			result.Status, result.Observed, result.Detail = Fail, value, "effective unit property violates profile"
		}
		report.Results = append(report.Results, result)
	}
	return report, nil
}
