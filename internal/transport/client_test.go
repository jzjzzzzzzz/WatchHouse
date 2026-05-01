package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"watchhouse/internal/spool"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func queuedItem(sequence int64) spool.Item {
	item := testItem(sequence, "host-1")
	return spool.Item{Sequence: item.Sequence, EventID: item.EventID, Event: item.Event}
}

func responseFor(status int, value any) *http.Response {
	body, _ := json.Marshal(value)
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(body))}
}

func TestClientRequiresExplicitPrivateTLSIdentity(t *testing.T) {
	files := makeTestPKI(t, "host-1")
	valid, _, err := LoadClientTLS(files.ca, files.clientCert, files.clientKey, "control.test")
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient("https://127.0.0.1:8443/", valid)
	if err != nil || client.endpoint != "https://127.0.0.1:8443/v1/events/batch" {
		t.Fatalf("client %+v %v", client, err)
	}
	client.Close()
	noIdentity := valid.Clone()
	noIdentity.Certificates = nil
	insecure := valid.Clone()
	insecure.InsecureSkipVerify = true
	for _, test := range []struct {
		endpoint string
		config   *tls.Config
	}{
		{"http://control.test", valid}, {"https://control.test/path", valid}, {"https://user@control.test", valid},
		{"https://control.test", nil}, {"https://control.test", insecure},
		{"https://control.test", noIdentity},
	} {
		if _, err := NewClient(test.endpoint, test.config); err == nil {
			t.Fatalf("accepted endpoint %q config %+v", test.endpoint, test.config)
		}
	}
}

func TestClientAcceptsOnlyExactReceipts(t *testing.T) {
	queued := []spool.Item{queuedItem(7)}
	client := &Client{endpoint: "https://control.test/v1/events/batch", http: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
			t.Fatal("incorrect event request")
		}
		return responseFor(http.StatusOK, BatchResponse{SchemaVersion: 1, Receipts: []Receipt{{Sequence: 7, EventID: queued[0].EventID}}}), nil
	})}}
	receipts, err := client.Deliver(context.Background(), queued)
	if err != nil || len(receipts) != 1 || receipts[0].EventID != queued[0].EventID {
		t.Fatalf("receipts %+v %v", receipts, err)
	}
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return responseFor(http.StatusOK, BatchResponse{SchemaVersion: 1}), nil
	})
	if _, err := client.Deliver(context.Background(), queued); err == nil {
		t.Fatal("partial receipt accepted")
	}
}

func TestClientNeverReturnsReceiptsOnHTTPFailure(t *testing.T) {
	client := &Client{endpoint: "https://control.test/v1/events/batch", http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return responseFor(http.StatusServiceUnavailable, BatchResponse{SchemaVersion: 1, Receipts: []Receipt{{Sequence: 1, EventID: "fake"}}}), nil
	})}}
	if receipts, err := client.Deliver(context.Background(), []spool.Item{queuedItem(1)}); err == nil || receipts != nil {
		t.Fatalf("failure receipts %+v %v", receipts, err)
	}
	if _, err := client.Deliver(context.Background(), nil); err == nil {
		t.Fatal("empty batch accepted")
	}
}
