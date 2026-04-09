package detection

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"watchhouse/internal/telemetry"
)

var start = time.Unix(1775642400, 0).UTC()

func event(n int, outcome string, second int) telemetry.Event {
	e := telemetry.Event{SchemaVersion: 1, HostID: "lab-1", BootID: "0123456789abcdef0123456789abcdef",
		Source: "journald", SourceCursor: fmt.Sprintf("s=fixture;i=%d", n), Kind: "ssh.authentication",
		ObservedAt: start.Add(time.Duration(second) * time.Second), ReceivedAt: start,
		Authentication: telemetry.Authentication{Outcome: outcome, Method: "password", User: "alice", SourceIP: "192.0.2.10", SourcePort: 51000}}
	e.EventID = telemetry.Identity(e.HostID, e.BootID, e.SourceCursor)
	return e
}

func detector(t *testing.T) *SSHDetector {
	t.Helper()
	d, err := NewSSH(Config{Window: 5 * time.Minute, Threshold: 3, MaxEvents: 32})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSequenceEvidenceAndDedup(t *testing.T) {
	d := detector(t)
	var expected []string
	for i := 0; i < 3; i++ {
		e := event(i, "failed", i)
		expected = append(expected, e.EventID)
		for repeat := 0; repeat < 2; repeat++ {
			finding, err := d.Observe(e)
			if finding != nil || err != nil {
				t.Fatalf("unexpected failed observation: %v", err)
			}
		}
	}
	success := event(3, "accepted", 3)
	f, err := d.Observe(success)
	expected = append(expected, success.EventID)
	if err != nil || f == nil || !reflect.DeepEqual(f.Evidence, expected) || f.Priority != "high" {
		t.Fatalf("bad finding: %+v, %v", f, err)
	}
	if again, err := d.Observe(success); again != nil || err != nil {
		t.Fatal("duplicate success re-emitted finding")
	}
	if next, err := d.Observe(event(4, "accepted", 4)); next != nil || err != nil {
		t.Fatal("success reused old failures")
	}
	d2 := detector(t)
	for i := 0; i < 3; i++ {
		d2.Observe(event(i, "failed", i))
	}
	f2, _ := d2.Observe(success)
	if f.FindingID != f2.FindingID {
		t.Fatal("finding identity unstable across replay")
	}
}

func TestIsolationAndWindow(t *testing.T) {
	for _, change := range []string{"user", "ip", "host", "boot", "expired", "boundary"} {
		t.Run(change, func(t *testing.T) {
			d := detector(t)
			for i := 0; i < 3; i++ {
				d.Observe(event(i, "failed", 0))
			}
			e := event(3, "accepted", 1)
			switch change {
			case "user":
				e.Authentication.User = "bob"
			case "ip":
				e.Authentication.SourceIP = "192.0.2.11"
			case "host":
				e.HostID = "lab-2"
			case "boot":
				e.BootID = "1123456789abcdef0123456789abcdef"
			case "expired":
				e.ObservedAt = start.Add(301 * time.Second)
			case "boundary":
				e.ObservedAt = start.Add(300 * time.Second)
			}
			e.EventID = telemetry.Identity(e.HostID, e.BootID, e.SourceCursor)
			f, err := d.Observe(e)
			if err != nil || (f != nil) != (change == "boundary") {
				t.Fatalf("finding=%+v err=%v", f, err)
			}
		})
	}
}

func TestSubThresholdSuccessResetsRun(t *testing.T) {
	d := detector(t)
	for i, outcome := range []string{"failed", "failed", "accepted", "failed", "accepted"} {
		if f, err := d.Observe(event(i, outcome, i)); f != nil || err != nil {
			t.Fatal("sub-threshold run was incorrectly merged")
		}
	}
}

func TestErrorsDoNotConsumeEvents(t *testing.T) {
	d := detector(t)
	e := event(0, "failed", 10)
	d.Observe(e)
	conflict := e
	conflict.Authentication.User = "other"
	if _, err := d.Observe(conflict); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	if _, err := d.Observe(event(1, "failed", 9)); !errors.Is(err, ErrOutOfOrder) {
		t.Fatalf("want late, got %v", err)
	}
	if _, err := d.Observe(event(1, "failed", 11)); err != nil {
		t.Fatal("rejected event consumed identity")
	}
	bad := event(2, "failed", 12)
	bad.EventID = "bad"
	if _, err := d.Observe(bad); err == nil {
		t.Fatal("invalid event accepted")
	}
}

func TestCapacityAndPruning(t *testing.T) {
	d, _ := NewSSH(Config{Window: time.Second, Threshold: 2, MaxEvents: 2})
	d.Observe(event(0, "failed", 0))
	d.Observe(event(1, "failed", 0))
	if _, err := d.Observe(event(2, "accepted", 0)); !errors.Is(err, ErrCapacity) {
		t.Fatal("capacity silently exceeded")
	}
	if _, err := d.Observe(event(2, "accepted", 2)); err != nil {
		t.Fatal("expired state did not free capacity")
	}
	if len(d.seen) != 1 || len(d.failures) != 0 {
		t.Fatal("expired state retained")
	}
}

func TestBoundedEvidence(t *testing.T) {
	d := detector(t)
	for i := 0; i < 20; i++ {
		d.Observe(event(i, "failed", i))
	}
	f, _ := d.Observe(event(20, "accepted", 20))
	if f == nil || len(f.Evidence) != 4 || f.Evidence[0] != event(17, "failed", 17).EventID {
		t.Fatal("did not retain recent threshold evidence")
	}
}

func TestInvalidConfigurations(t *testing.T) {
	for _, config := range []Config{{}, {time.Millisecond, 1, 2}, {25 * time.Hour, 1, 2}, {time.Second, 0, 2}, {time.Second, 3, 2}, {time.Second, 1, 1_000_001}} {
		if _, err := NewSSH(config); err == nil {
			t.Errorf("accepted invalid config %+v", config)
		}
	}
}
