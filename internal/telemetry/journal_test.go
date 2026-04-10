package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func record(message string) []byte {
	b, _ := json.Marshal(map[string]any{
		"_COMM": "sshd", "_UID": "0", "_BOOT_ID": "0123456789abcdef0123456789abcdef",
		"__CURSOR": "s=fixture;i=1", "__REALTIME_TIMESTAMP": "1775642400123456", "MESSAGE": message,
	})
	return b
}

func TestAuthenticationForms(t *testing.T) {
	tests := []struct {
		message, outcome, ip string
		invalid              bool
	}{
		{"Failed password for invalid user guest from 192.0.2.10 port 51000 ssh2", "failed", "192.0.2.10", true},
		{"Accepted publickey for alice from 2001:db8::1 port 51000 ssh2: ED25519 SHA256:fixture", "accepted", "2001:db8::1", false},
		{"Failed publickey for alice from ::ffff:192.0.2.10 port 51000 ssh2 [preauth]", "failed", "192.0.2.10", false},
		{"Accepted password for alice from 192.0.2.10 port 51000 ssh2", "accepted", "192.0.2.10", false},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			e, matched, err := ParseJournal(record(tt.message), "lab-1", time.Now())
			if err != nil || !matched {
				t.Fatalf("matched=%v err=%v", matched, err)
			}
			if e.Authentication.Outcome != tt.outcome || e.Authentication.SourceIP != tt.ip || e.Authentication.InvalidUser != tt.invalid {
				t.Fatalf("unexpected auth: %+v", e.Authentication)
			}
			if e.ObservedAt.Nanosecond() != 123456000 {
				t.Fatal("lost microsecond precision")
			}
			encoded, _ := json.Marshal(e)
			if strings.Contains(string(encoded), "SHA256:fixture") || strings.Contains(string(encoded), "MESSAGE") {
				t.Fatal("raw message leaked")
			}
		})
	}
}

func TestMalformedJournal(t *testing.T) {
	good := string(record("Failed password for alice from 192.0.2.10 port 51000 ssh2"))
	tests := []string{
		`[]`, `{`, good + `{}`, strings.Replace(good, `"_UID":"0"`, `"_UID":0`, 1),
		strings.Replace(good, `"_UID":"0"`, `"_UID":"0","_UID":"1000"`, 1),
		strings.Replace(good, `"_UID":"0"`, `"_UID":["0"]`, 1),
		strings.Replace(good, `"__CURSOR":"s=fixture;i=1",`, "", 1),
		strings.Replace(good, "1775642400123456", "9999999999999999999", 1),
		strings.Replace(good, "192.0.2.10", "not-an-ip", 1),
		strings.Replace(good, "51000", "65536", 1),
		strings.Replace(good, "51000", "0", 1),
		strings.Replace(good, "0123456789abcdef0123456789abcdef", "bad-boot", 1),
		strings.Replace(good, "ssh2", `ssh2\nforged`, 1),
		strings.Repeat(" ", MaxRecordBytes+1),
		string(record("Accepted password for invalid user alice from 192.0.2.10 port 51000 ssh2")),
	}
	for i, line := range tests {
		if _, matched, err := ParseJournal([]byte(line), "lab-1", time.Now()); err == nil || matched {
			t.Errorf("case %d did not reject malformed input", i)
		}
	}
}

func TestTrustedSourceAndUnmatched(t *testing.T) {
	good := string(record("Accepted password for alice from 192.0.2.10 port 51000 ssh2"))
	for _, line := range []string{
		strings.Replace(good, `"_COMM":"sshd"`, `"_COMM":"logger","SYSLOG_IDENTIFIER":"sshd"`, 1),
		strings.Replace(good, `"_UID":"0"`, `"_UID":"1000"`, 1),
		string(record("Connection closed by authenticating user alice 192.0.2.10 port 51000 [preauth]")),
	} {
		if _, matched, err := ParseJournal([]byte(line), "lab-1", time.Now()); err != nil || matched {
			t.Fatalf("irrelevant record matched: %v", err)
		}
	}
	line := strings.Replace(good, `"_COMM":"sshd"`, `"_COMM":"sshd-session"`, 1)
	if _, matched, err := ParseJournal([]byte(line), "lab-1", time.Now()); err != nil || !matched {
		t.Fatal("sshd-session unsupported")
	}
}

func TestStableIdentityAndValidation(t *testing.T) {
	line := record("Accepted password for alice from 192.0.2.10 port 51000 ssh2")
	a, _, _ := ParseJournal(line, "lab-1", time.Now())
	b, _, _ := ParseJournal(line, "lab-1", time.Now().Add(time.Hour))
	if a.EventID != b.EventID {
		t.Fatal("receive time changed source identity")
	}
	if Identity("ab", "c") == Identity("a", "bc") {
		t.Fatal("ambiguous identity encoding")
	}
	b.EventID = "forged"
	if b.Validate() == nil {
		t.Fatal("forged identity accepted")
	}
	if _, _, err := ParseJournal(line, "bad host", time.Now()); err == nil {
		t.Fatal("invalid host accepted")
	}
	if _, _, err := ParseJournal(line, "lab-1", time.Time{}); err == nil {
		t.Fatal("missing receive time accepted")
	}
}

func TestErrorsDoNotEchoUntrustedKeys(t *testing.T) {
	line := []byte(`{"secret-fixture-token":1,"secret-fixture-token":2}`)
	if _, _, err := ParseJournal(line, "lab-1", time.Now()); err == nil || strings.Contains(err.Error(), "secret-fixture-token") {
		t.Fatal("duplicate key was not safely rejected")
	}
	bad := append(record("Accepted password for alice from 192.0.2.10 port 51000 ssh2"), 0xff)
	if _, _, err := ParseJournal(bad, "lab-1", time.Now()); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func FuzzParseJournal(f *testing.F) {
	f.Add(record("Accepted publickey for alice from 192.0.2.1 port 22 ssh2"))
	f.Add([]byte(`{"_COMM":"sshd","MESSAGE":null}`))
	f.Fuzz(func(t *testing.T, line []byte) {
		e, matched, err := ParseJournal(line, "fuzz-host", time.Unix(1775642400, 0))
		if matched && (err != nil || e.Validate() != nil) {
			t.Fatal("parser emitted an invalid event")
		}
	})
}
