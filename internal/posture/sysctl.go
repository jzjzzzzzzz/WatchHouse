package posture

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

type SysctlControl struct {
	ID       string
	Path     string
	Expected []string
	Reason   string
}

var ServerSysctls = []SysctlControl{
	{"NET-001", "net/ipv4/conf/all/accept_redirects", []string{"0"}, "ignore IPv4 ICMP redirects"},
	{"NET-002", "net/ipv4/conf/default/accept_redirects", []string{"0"}, "new interfaces ignore IPv4 redirects"},
	{"NET-003", "net/ipv4/conf/all/send_redirects", []string{"0"}, "do not emit redirects"},
	{"NET-004", "net/ipv4/conf/default/send_redirects", []string{"0"}, "new interfaces do not emit redirects"},
	{"NET-005", "net/ipv4/conf/all/accept_source_route", []string{"0"}, "reject source-routed IPv4"},
	{"NET-006", "net/ipv4/conf/default/accept_source_route", []string{"0"}, "new interfaces reject source routing"},
	{"NET-007", "net/ipv4/conf/all/rp_filter", []string{"1", "2"}, "enable reverse-path filtering"},
	{"NET-008", "net/ipv4/conf/default/rp_filter", []string{"1", "2"}, "new interfaces use reverse-path filtering"},
	{"NET-009", "net/ipv4/tcp_syncookies", []string{"1"}, "enable SYN cookies"},
	{"NET-010", "net/ipv6/conf/all/accept_redirects", []string{"0"}, "ignore IPv6 redirects"},
	{"KERN-001", "kernel/randomize_va_space", []string{"2"}, "full address-space randomization"},
	{"KERN-002", "kernel/kptr_restrict", []string{"1", "2"}, "restrict kernel pointer exposure"},
	{"KERN-003", "kernel/dmesg_restrict", []string{"1"}, "restrict unprivileged kernel log access"},
	{"KERN-004", "fs/protected_hardlinks", []string{"1"}, "protect hardlink creation"},
	{"KERN-005", "fs/protected_symlinks", []string{"1"}, "protect symlink traversal"},
}

func EvaluateSysctls(root fs.FS, controls []SysctlControl) ([]Result, error) {
	seen := map[string]bool{}
	results := make([]Result, 0, len(controls))
	for _, control := range controls {
		if control.ID == "" || seen[control.ID] || !validProcPath(control.Path) || len(control.Expected) == 0 {
			return nil, fmt.Errorf("invalid or duplicate sysctl control %q", control.ID)
		}
		seen[control.ID] = true
		result := Result{ControlID: control.ID, Expected: strings.Join(control.Expected, " or "), Source: "/proc/sys/" + control.Path}
		body, err := fs.ReadFile(root, control.Path)
		if err != nil {
			result.Status, result.Detail = Error, err.Error()
		} else {
			result.Observed = strings.TrimSpace(string(body))
			result.Status = Fail
			for _, expected := range control.Expected {
				if result.Observed == expected {
					result.Status = Pass
					break
				}
			}
			if result.Status == Fail {
				result.Detail = control.Reason
			}
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ControlID < results[j].ControlID })
	return results, nil
}

func validProcPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.ContainsAny(path, "\x00\r\n") {
		return false
	}
	return strings.HasPrefix(path, "net/") || strings.HasPrefix(path, "kernel/") || strings.HasPrefix(path, "fs/")
}
