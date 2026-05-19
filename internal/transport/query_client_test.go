package transport

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func queryRecord(sequence int64, host string) EventRecord {
	item := testItem(sequence, host)
	return EventRecord{IngestSequence: sequence, HostID: host, EventID: item.EventID, IngestedAt: time.Unix(10, 0).UTC(), Event: item.Event}
}

func findingRecord(sequence int64, host string) FindingRecord {
	digest := strings.Repeat("a", 64)
	stamp := time.Unix(10, 0).UTC()
	return FindingRecord{FindingSequence: sequence, FindingID: digest, HostID: host, RuleID: "ssh.failure_then_success",
		ObservedAt: stamp, Priority: "medium", Summary: "authentication succeeded after failures", WindowSeconds: 300,
		Threshold: 5, EvidenceEventIDs: []string{digest}, CreatedAt: stamp, LastEvaluatedAt: stamp}
}

func TestQueryClientValidatesOrderedHostBoundPage(t *testing.T) {
	client := &Client{origin: "https://control.test", http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Query().Get("host") != "host-1" || request.URL.Query().Get("before") != "20" {
			t.Fatal("incorrect query request")
		}
		return responseFor(http.StatusOK, EventPage{SchemaVersion: 1, Records: []EventRecord{queryRecord(19, "host-1"), queryRecord(18, "host-1")}}), nil
	})}}
	page, err := client.QueryEvents(context.Background(), "host-1", 2, 20)
	if err != nil || len(page.Records) != 2 {
		t.Fatalf("page %+v error %v", page, err)
	}
}

func TestQueryClientRejectsInvalidRecordsAndStatus(t *testing.T) {
	client := &Client{origin: "https://control.test", http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return responseFor(http.StatusOK, EventPage{SchemaVersion: 1, Records: []EventRecord{queryRecord(2, "host-1"), queryRecord(3, "host-1")}}), nil
	})}}
	if _, err := client.QueryEvents(context.Background(), "host-1", 2, 0); err == nil {
		t.Fatal("out-of-order page accepted")
	}
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return responseFor(http.StatusForbidden, struct{}{}), nil })
	if _, err := client.QueryEvents(context.Background(), "host-1", 2, 0); err == nil {
		t.Fatal("authorization failure accepted")
	}
	if _, err := client.QueryEvents(context.Background(), "../host", 2, 0); err == nil {
		t.Fatal("invalid host accepted")
	}
}

func TestFindingQueryClientValidatesEvidenceAndOrder(t *testing.T) {
	client := &Client{origin: "https://control.test", http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/findings" || request.URL.Query().Get("before") != "20" {
			t.Fatal("incorrect finding request")
		}
		return responseFor(http.StatusOK, FindingPage{SchemaVersion: 1, Records: []FindingRecord{findingRecord(19, "host-1")}}), nil
	})}}
	page, err := client.QueryFindings(context.Background(), "host-1", 2, 20)
	if err != nil || len(page.Records) != 1 {
		t.Fatalf("page %+v error %v", page, err)
	}
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		bad := findingRecord(2, "host-1")
		bad.EvidenceEventIDs[0] = "not-a-digest"
		return responseFor(http.StatusOK, FindingPage{SchemaVersion: 1, Records: []FindingRecord{bad}}), nil
	})
	if _, err := client.QueryFindings(context.Background(), "host-1", 2, 0); err == nil {
		t.Fatal("invalid finding evidence accepted")
	}
}
