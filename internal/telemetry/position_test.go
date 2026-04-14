package telemetry

import (
	"strings"
	"testing"
	"time"
)

func TestPositionForUnmatchedRecord(t *testing.T) {
	line := record("Connection closed by 192.0.2.10 port 50000 [preauth]")
	position, err := JournalPosition(line)
	if err != nil || position.Cursor != "s=fixture;i=1" || position.BootID != "0123456789abcdef0123456789abcdef" || position.ObservedAt.Nanosecond() != 123456000 {
		t.Fatalf("position %+v %v", position, err)
	}
	if _, matched, err := ParseJournal(line, "lab-1", time.Now()); err != nil || matched {
		t.Fatal("unmatched record became authentication event")
	}
}

func TestPositionRejectsMalformedMetadata(t *testing.T) {
	good := string(record("unmatched"))
	for _, line := range []string{
		`{}`, `null`, good + "extra", strings.Repeat("x", MaxRecordBytes+1),
		strings.Replace(good, "1775642400123456", "+1775642400123456", 1),
		strings.Replace(good, "1775642400123456", "-1", 1),
		strings.Replace(good, "1775642400123456", "253402300800000000", 1),
		strings.Replace(good, "s=fixture;i=1", `s=bad\ncursor`, 1),
		strings.Replace(good, "0123456789abcdef0123456789abcdef", "invalid", 1),
		strings.Replace(good, `"__CURSOR":"s=fixture;i=1"`, `"__CURSOR":null`, 1),
	} {
		if _, err := JournalPosition([]byte(line)); err == nil {
			t.Fatalf("malformed source metadata accepted")
		}
	}
}
