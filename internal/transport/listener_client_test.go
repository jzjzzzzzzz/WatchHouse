package transport

import (
	"context"
	"net/http"
	"testing"
)

func TestListenerClientRequiresExactReceipt(t *testing.T) {
	snapshot := validListenerSnapshot()
	expected := ListenerSnapshotID("host-1", snapshot)
	client := &Client{origin: "https://control.test", http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/listener-snapshots" {
			t.Fatal("incorrect listener publish request")
		}
		return responseFor(http.StatusOK, ListenerSnapshotReceipt{SchemaVersion: 1, SnapshotID: expected}), nil
	})}}
	receipt, err := client.PublishListenerSnapshot(context.Background(), "host-1", snapshot)
	if err != nil || receipt.SnapshotID != expected {
		t.Fatalf("receipt %+v error %v", receipt, err)
	}
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return responseFor(http.StatusOK, ListenerSnapshotReceipt{SchemaVersion: 1, SnapshotID: "wrong"}), nil
	})
	if _, err := client.PublishListenerSnapshot(context.Background(), "host-1", snapshot); err == nil {
		t.Fatal("mismatched listener receipt accepted")
	}
}
