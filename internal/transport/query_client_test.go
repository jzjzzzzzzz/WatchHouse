package transport

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func queryRecord(sequence int64, host string) EventRecord {
	item := testItem(sequence, host)
	return EventRecord{IngestSequence: sequence, HostID: host, EventID: item.EventID, IngestedAt: time.Unix(10, 0).UTC(), Event: item.Event}
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
