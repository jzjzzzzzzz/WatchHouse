package runtimeinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestOwnPrivilegeParsing(t *testing.T) {
	status := "Name:\twatchhouse\nNoNewPrivs:\t1\nCapEff:\t0000000000000000\nSeccomp:\t2\n"
	result, err := ParseLinuxStatus(status)
	if err != nil || !result.NoNewPrivileges || result.EffectiveCapabilities != "0000000000000000" || result.SeccompMode != 2 {
		t.Fatalf("security %+v %v", result, err)
	}
}

func TestMalformedSecurityStatusRejected(t *testing.T) {
	valid := "NoNewPrivs: 1\nCapEff: 0000000000000000\nSeccomp: 2\n"
	for _, body := range []string{"", valid + "CapEff: 0000000000000000\n", strings.Replace(valid, "NoNewPrivs: 1", "NoNewPrivs: 2", 1), strings.Replace(valid, "Seccomp: 2", "Seccomp: 3", 1), strings.Replace(valid, "0000000000000000", "invalid", 1), strings.Repeat("x", 64*1024+1)} {
		if _, err := ParseLinuxStatus(body); err == nil {
			t.Fatal("malformed own privilege state accepted")
		}
	}
}

func TestSelfDoesNotClaimLinuxSecurityOnOtherPlatforms(t *testing.T) {
	result, err := Self()
	if err != nil {
		t.Fatal(err)
	}
	if result.OS != runtime.GOOS {
		t.Fatal("incorrect platform")
	}
	if runtime.GOOS != "linux" && (!result.SecurityUnavailable || result.LinuxSecurity != nil) {
		t.Fatal("unsupported platform claimed kernel sandbox")
	}
}
