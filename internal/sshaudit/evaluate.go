package sshaudit

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const MaxOutputBytes = 1 << 20

type control struct {
	id, directive, expected string
	accept                  func(string) bool
}

func equals(values ...string) func(string) bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return func(value string) bool { return set[value] }
}

func maxInt(max int) func(string) bool {
	return func(value string) bool { n, err := strconv.Atoi(value); return err == nil && n >= 1 && n <= max }
}

var serverControls = []control{
	{"SSH-001", "permitrootlogin", "no", equals("no")},
	{"SSH-002", "passwordauthentication", "no", equals("no")},
	{"SSH-003", "kbdinteractiveauthentication", "no", equals("no")},
	{"SSH-004", "permitemptypasswords", "no", equals("no")},
	{"SSH-005", "pubkeyauthentication", "yes", equals("yes")},
	{"SSH-006", "x11forwarding", "no", equals("no")},
	{"SSH-007", "maxauthtries", "1..4", maxInt(4)},
	{"SSH-008", "ignorerhosts", "yes", equals("yes")},
	{"SSH-009", "hostbasedauthentication", "no", equals("no")},
	{"SSH-010", "permituserenvironment", "no", equals("no")},
}

func Evaluate(raw []byte) ([]Result, error) {
	if len(raw) == 0 || len(raw) > MaxOutputBytes {
		return nil, fmt.Errorf("sshd output size must be 1..%d bytes", MaxOutputBytes)
	}
	values := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return nil, fmt.Errorf("malformed sshd output line")
		}
		key := strings.ToLower(parts[0])
		value := strings.ToLower(strings.Join(parts[1:], " "))
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("duplicate sshd directive %q", key)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan sshd output: %w", err)
	}
	results := make([]Result, 0, len(serverControls))
	for _, check := range serverControls {
		result := Result{ControlID: check.id, Directive: check.directive, Expected: check.expected}
		value, ok := values[check.directive]
		if !ok {
			result.Status, result.Detail = Error, "directive absent from effective configuration"
		} else if check.accept(value) {
			result.Status, result.Observed = Pass, value
		} else {
			result.Status, result.Observed, result.Detail = Fail, value, "effective value violates server profile"
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ControlID < results[j].ControlID })
	return results, nil
}
