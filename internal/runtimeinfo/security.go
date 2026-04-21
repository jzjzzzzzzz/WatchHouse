// Package runtimeinfo reads only the collector's own privilege state.
package runtimeinfo

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type Security struct {
	NoNewPrivileges       bool   `json:"no_new_privileges"`
	EffectiveCapabilities string `json:"effective_capabilities"`
	SeccompMode           int    `json:"seccomp_mode"`
}

type Snapshot struct {
	OS                  string    `json:"os"`
	UID                 int       `json:"uid"`
	GID                 int       `json:"gid"`
	LinuxSecurity       *Security `json:"linux_security,omitempty"`
	SecurityUnavailable bool      `json:"security_unavailable"`
}

func Self() (Snapshot, error) {
	result := Snapshot{OS: runtime.GOOS, UID: os.Geteuid(), GID: os.Getegid()}
	if runtime.GOOS != "linux" {
		result.SecurityUnavailable = true
		return result, nil
	}
	body, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return result, fmt.Errorf("read own kernel status: %w", err)
	}
	security, err := ParseLinuxStatus(string(body))
	if err != nil {
		return result, err
	}
	result.LinuxSecurity = &security
	return result, nil
}

func ParseLinuxStatus(body string) (Security, error) {
	var result Security
	if len(body) > 64*1024 {
		return result, fmt.Errorf("kernel status exceeds bound")
	}
	values := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		name, value, found := strings.Cut(scanner.Text(), ":")
		if !found {
			continue
		}
		if name != "NoNewPrivs" && name != "CapEff" && name != "Seccomp" {
			continue
		}
		if _, exists := values[name]; exists {
			return result, fmt.Errorf("duplicate own security status field")
		}
		values[name] = strings.TrimSpace(value)
	}
	if scanner.Err() != nil || len(values) != 3 {
		return result, fmt.Errorf("incomplete own security status")
	}
	if values["NoNewPrivs"] != "0" && values["NoNewPrivs"] != "1" {
		return result, fmt.Errorf("invalid no-new-privileges state")
	}
	mode, err := strconv.Atoi(values["Seccomp"])
	if err != nil || mode < 0 || mode > 2 {
		return result, fmt.Errorf("invalid seccomp mode")
	}
	capability, err := strconv.ParseUint(values["CapEff"], 16, 64)
	if err != nil || len(values["CapEff"]) != 16 {
		return result, fmt.Errorf("invalid effective capability mask")
	}
	result.NoNewPrivileges = values["NoNewPrivs"] == "1"
	result.EffectiveCapabilities = fmt.Sprintf("%016x", capability)
	result.SeccompMode = mode
	return result, nil
}
