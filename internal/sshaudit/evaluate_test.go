package sshaudit

import (
	"strings"
	"testing"
)

func secureFixture() []byte {
	return []byte(`permitrootlogin no
passwordauthentication no
kbdinteractiveauthentication no
permitemptypasswords no
pubkeyauthentication yes
x11forwarding no
maxauthtries 3
ignorerhosts yes
hostbasedauthentication no
permituserenvironment no
port 22
`)
}

func TestEvaluateEffectiveConfigPassesSecureFixture(t *testing.T) {
	got, err := Evaluate(secureFixture())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(serverControls) {
		t.Fatalf("len=%d", len(got))
	}
	for _, result := range got {
		if result.Status != Pass {
			t.Fatalf("%#v", result)
		}
	}
}

func TestEvaluateReportsFailureAndMissingDirective(t *testing.T) {
	raw := strings.Replace(string(secureFixture()), "permitrootlogin no", "permitrootlogin prohibit-password", 1)
	raw = strings.Replace(raw, "x11forwarding no\n", "", 1)
	got, err := Evaluate([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Result{}
	for _, result := range got {
		byID[result.ControlID] = result
	}
	if byID["SSH-001"].Status != Fail || byID["SSH-001"].Observed != "prohibit-password" {
		t.Fatalf("%#v", byID["SSH-001"])
	}
	if byID["SSH-006"].Status != Error {
		t.Fatalf("%#v", byID["SSH-006"])
	}
}

func TestEvaluateRejectsMalformedAmbiguousOrOversizeOutput(t *testing.T) {
	tests := [][]byte{
		nil,
		[]byte("malformed\n"),
		[]byte("permitrootlogin no\npermitrootlogin yes\n"),
		append([]byte("permitrootlogin "), append(make([]byte, 70<<10), '\n')...),
		make([]byte, MaxOutputBytes+1),
	}
	for _, raw := range tests {
		if _, err := Evaluate(raw); err == nil {
			t.Fatalf("accepted %d bytes", len(raw))
		}
	}
}

func FuzzEvaluateNeverPanics(f *testing.F) {
	f.Add(secureFixture())
	f.Add([]byte("permitrootlogin no\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxOutputBytes+1 {
			t.Skip()
		}
		_, _ = Evaluate(raw)
	})
}
