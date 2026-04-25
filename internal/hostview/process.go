package hostview

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ProcessIdentity struct {
	PID            int    `json:"pid"`
	StartTimeTicks uint64 `json:"start_time_ticks"`
	EffectiveUID   uint32 `json:"effective_uid"`
	Comm           string `json:"comm"`
	SystemdUnit    string `json:"systemd_unit,omitempty"`
}

func parseProcessStat(body string, expectedPID int) (string, uint64, error) {
	if len(body) == 0 || len(body) > 4096 || strings.ContainsRune(body, '\x00') {
		return "", 0, fmt.Errorf("invalid process stat size")
	}
	open := strings.IndexByte(body, '(')
	close := strings.LastIndex(body, ") ")
	if open < 1 || close <= open {
		return "", 0, fmt.Errorf("invalid process stat framing")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(body[:open]))
	if err != nil || pid != expectedPID {
		return "", 0, fmt.Errorf("process stat PID mismatch")
	}
	comm := body[open+1 : close]
	if !utf8.ValidString(comm) || len(comm) == 0 || len(comm) > 64 || hasControl(comm) {
		return "", 0, fmt.Errorf("invalid process comm")
	}
	// The suffix begins with field 3 (state); starttime is field 22.
	fields := strings.Fields(body[close+2:])
	if len(fields) < 20 {
		return "", 0, fmt.Errorf("incomplete process stat")
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || start == 0 {
		return "", 0, fmt.Errorf("invalid process start time")
	}
	return comm, start, nil
}

func parseEffectiveUID(body string) (uint32, error) {
	if len(body) == 0 || len(body) > 64*1024 {
		return 0, fmt.Errorf("invalid process status size")
	}
	var value string
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		name, rest, found := strings.Cut(scanner.Text(), ":")
		if !found || name != "Uid" {
			continue
		}
		if value != "" {
			return 0, fmt.Errorf("duplicate process UID field")
		}
		value = strings.TrimSpace(rest)
	}
	parts := strings.Fields(value)
	if scanner.Err() != nil || len(parts) != 4 {
		return 0, fmt.Errorf("incomplete process UID field")
	}
	uid, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid effective process UID")
	}
	return uint32(uid), nil
}

func parseSystemdUnit(body string) (string, error) {
	if len(body) > 64*1024 {
		return "", fmt.Errorf("process cgroup exceeds bound")
	}
	units := map[string]struct{}{}
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 3)
		if len(parts) != 3 || !strings.HasPrefix(parts[2], "/") {
			return "", fmt.Errorf("malformed process cgroup")
		}
		for _, component := range strings.Split(parts[2], "/") {
			if strings.HasSuffix(component, ".service") {
				if len(component) == len(".service") || len(component) > 256 || hasControl(component) {
					return "", fmt.Errorf("invalid systemd service component")
				}
				units[component] = struct{}{}
			}
		}
	}
	if scanner.Err() != nil {
		return "", fmt.Errorf("scan process cgroup: %w", scanner.Err())
	}
	if len(units) > 1 {
		return "", fmt.Errorf("ambiguous systemd service membership")
	}
	for unit := range units {
		return unit, nil
	}
	return "", nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
