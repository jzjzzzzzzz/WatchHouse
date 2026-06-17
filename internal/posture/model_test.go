package posture

import (
	"testing"
	"time"
)

func TestNewReportCountsStatuses(t *testing.T) {
	report := NewReport(time.Now(), []Result{{Status: Pass}, {Status: Fail}, {Status: Error}, {Status: Pass}})
	if report.Passed != 2 || report.Failed != 1 || report.Errors != 1 || report.SchemaVersion != 1 {
		t.Fatalf("%#v", report)
	}
	if report.ObservedAt.Location() != time.UTC {
		t.Fatalf("timestamp is not UTC: %v", report.ObservedAt)
	}
}
