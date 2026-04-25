package hostview

import (
	"fmt"
	"strings"
	"testing"
)

func syntheticStat(pid int, comm, start string) string {
	// Fields after comm: state (3), then 18 values through starttime (22).
	return fmt.Sprintf("%d (%s) S %s %s\n", pid, comm, strings.Repeat("1 ", 18), start)
}

func TestParseStableProcessFields(t *testing.T) {
	comm, start, err := parseProcessStat(syntheticStat(42, "name with ) paren", "9876"), 42)
	if err != nil || comm != "name with ) paren" || start != 9876 {
		t.Fatalf("stat %q %d %v", comm, start, err)
	}
	uid, err := parseEffectiveUID("Name:\ttest\nUid:\t1000\t1001\t1002\t1003\n")
	if err != nil || uid != 1001 {
		t.Fatalf("uid %d %v", uid, err)
	}
	unit, err := parseSystemdUnit("0::/system.slice/ssh.service\n")
	if err != nil || unit != "ssh.service" {
		t.Fatalf("unit %q %v", unit, err)
	}
}

func TestMalformedProcessFieldsRejected(t *testing.T) {
	for _, stat := range []string{"", syntheticStat(41, "test", "9"), syntheticStat(42, "bad\nname", "9"), "42 (x) S 1"} {
		if _, _, err := parseProcessStat(stat, 42); err == nil {
			t.Fatalf("stat accepted: %q", stat)
		}
	}
	for _, status := range []string{"", "Uid: 1 2 3\n", "Uid: 1 2 3 4\nUid: 1 2 3 4\n", "Uid: 1 x 3 4\n"} {
		if _, err := parseEffectiveUID(status); err == nil {
			t.Fatalf("status accepted: %q", status)
		}
	}
	for _, cgroup := range []string{"invalid\n", "0::/a.service/b.service\n", "0::/bad\x01.service\n"} {
		if _, err := parseSystemdUnit(cgroup); err == nil {
			t.Fatalf("cgroup accepted: %q", cgroup)
		}
	}
}

func TestNoSystemdUnitIsExplicitlyEmpty(t *testing.T) {
	unit, err := parseSystemdUnit("0::/user.slice/user-1000.slice/session-1.scope\n")
	if err != nil || unit != "" {
		t.Fatalf("unit %q %v", unit, err)
	}
}
