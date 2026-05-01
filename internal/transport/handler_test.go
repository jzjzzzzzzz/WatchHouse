package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"watchhouse/internal/strictjson"
)

type memoryBatchStore struct {
	host  string
	items []Item
	err   error
}

func (store *memoryBatchStore) CommitBatch(_ context.Context, host string, items []Item) error {
	store.host, store.items = host, append([]Item(nil), items...)
	return store.err
}

func authenticatedRequest(t *testing.T, body []byte) *http.Request {
	t.Helper()
	identity, err := HostURI("host-1")
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{URIs: []*url.URL{identity}}
	request := httptest.NewRequest(http.MethodPost, "https://control.test/v1/events/batch", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}, VerifiedChains: [][]*x509.Certificate{{certificate}}}
	return request
}

func requestBody(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(BatchRequest{SchemaVersion: 1, Items: []Item{testItem(7, "host-1")}})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestHandlerCommitsBeforeExactReceipt(t *testing.T) {
	store := &memoryBatchStore{}
	response := httptest.NewRecorder()
	Handler{Store: store}.ServeHTTP(response, authenticatedRequest(t, requestBody(t)))
	if response.Code != http.StatusOK || store.host != "host-1" || len(store.items) != 1 {
		t.Fatalf("response %d store %+v", response.Code, store)
	}
	decoded, err := strictjson.Decode[BatchResponse](response.Body, MaxBodyBytes)
	if err != nil || decoded.ValidateExact(store.items) != nil {
		t.Fatalf("response %s: %v", response.Body.String(), err)
	}
}

func TestHandlerRejectsUnauthenticatedAndMalformedRequests(t *testing.T) {
	valid := requestBody(t)
	cases := []struct {
		mutate func(*http.Request)
		code   int
	}{
		{func(request *http.Request) { request.TLS = nil }, http.StatusUnauthorized},
		{func(request *http.Request) { request.TLS.VerifiedChains = nil }, http.StatusUnauthorized},
		{func(request *http.Request) { request.Method = http.MethodGet }, http.StatusMethodNotAllowed},
		{func(request *http.Request) { request.URL.Path = "/other" }, http.StatusNotFound},
		{func(request *http.Request) { request.Header.Set("Content-Type", "text/plain") }, http.StatusUnsupportedMediaType},
		{func(request *http.Request) { request.ContentLength = MaxBodyBytes + 1 }, http.StatusRequestEntityTooLarge},
	}
	for index, test := range cases {
		store := &memoryBatchStore{}
		request := authenticatedRequest(t, valid)
		test.mutate(request)
		response := httptest.NewRecorder()
		Handler{Store: store}.ServeHTTP(response, request)
		if response.Code != test.code || len(store.items) != 0 {
			t.Fatalf("case %d code %d store %+v", index, response.Code, store)
		}
	}
	request := authenticatedRequest(t, []byte(`{"schema_version":1,"schema_version":1,"items":[]}`))
	response := httptest.NewRecorder()
	Handler{Store: &memoryBatchStore{}}.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate JSON status %d", response.Code)
	}
}

func TestHandlerDoesNotReceiptStoreFailure(t *testing.T) {
	for _, test := range []struct {
		err  error
		code int
	}{{ErrEventConflict, http.StatusConflict}, {errors.New("down"), http.StatusServiceUnavailable}} {
		response := httptest.NewRecorder()
		Handler{Store: &memoryBatchStore{err: test.err}}.ServeHTTP(response, authenticatedRequest(t, requestBody(t)))
		if response.Code != test.code || bytes.Contains(response.Body.Bytes(), []byte(`"receipts"`)) {
			t.Fatalf("failure response %d %q", response.Code, response.Body.String())
		}
	}
}
