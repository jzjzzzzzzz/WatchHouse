package transport

import (
	"context"
	"net/http"
	"testing"
)

func TestProbeClientRequiresExactReceipt(t *testing.T) {
	result := validProbeResult()
	id, err := ProbeObservationID("outside-1", result)
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{origin: "https://control.test", http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/probe-observations" {
			t.Fatal("incorrect probe publish request")
		}
		return responseFor(http.StatusOK, ProbeObservationReceipt{SchemaVersion: 1, ObservationID: id}), nil
	})}}
	receipt, err := client.PublishProbeObservation(context.Background(), "outside-1", result)
	if err != nil || receipt.ObservationID != id {
		t.Fatalf("receipt %+v error %v", receipt, err)
	}
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return responseFor(http.StatusOK, ProbeObservationReceipt{SchemaVersion: 1, ObservationID: "wrong"}), nil
	})
	if _, err := client.PublishProbeObservation(context.Background(), "outside-1", result); err == nil {
		t.Fatal("mismatched probe receipt accepted")
	}
}
