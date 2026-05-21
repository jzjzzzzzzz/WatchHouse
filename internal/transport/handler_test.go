package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"watchhouse/internal/authz"
	"watchhouse/internal/strictjson"
)

type memoryBatchStore struct {
	host  string
	items []Item
	err   error
}

type memoryQueryStore struct {
	host    string
	limit   int
	before  int64
	records []EventRecord
}

type memoryFindingStore struct {
	host    string
	limit   int
	before  int64
	records []FindingRecord
}

type auditDecision struct{ principal, resource, host, decision string }
type memoryAuditor struct {
	decisions []auditDecision
	err       error
}

func (auditor *memoryAuditor) RecordQueryDecision(_ context.Context, principal string, _ authz.Role, resource, host, decision string) error {
	auditor.decisions = append(auditor.decisions, auditDecision{principal, resource, host, decision})
	return auditor.err
}

func (store *memoryFindingStore) QueryFindings(_ context.Context, host string, limit int, before int64) ([]FindingRecord, error) {
	store.host, store.limit, store.before = host, limit, before
	return store.records, nil
}

func (store *memoryQueryStore) QueryEvents(_ context.Context, host string, limit int, before int64) ([]EventRecord, error) {
	store.host, store.limit, store.before = host, limit, before
	return store.records, nil
}

func humanRequest(t *testing.T, user, rawQuery string) *http.Request {
	t.Helper()
	identity, err := UserURI(user)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{URIs: []*url.URL{identity}}
	request := httptest.NewRequest(http.MethodGet, "https://control.test/v1/events?"+rawQuery, nil)
	request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}, VerifiedChains: [][]*x509.Certificate{{certificate}}}
	return request
}

func viewerRoles(t *testing.T) *authz.Map {
	t.Helper()
	roles, err := authz.Decode(strings.NewReader(`{"schema_version":1,"principals":{"alice":"viewer"}}`))
	if err != nil {
		t.Fatal(err)
	}
	return roles
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

func TestHumanQueryIsRoleBoundedAndPaginated(t *testing.T) {
	store := &memoryQueryStore{records: []EventRecord{}}
	auditor := &memoryAuditor{}
	response := httptest.NewRecorder()
	Handler{Queries: store, Auditor: auditor, Roles: viewerRoles(t)}.ServeHTTP(response, humanRequest(t, "alice", "host=host-1&limit=7&before=42"))
	if response.Code != http.StatusOK || store.host != "host-1" || store.limit != 7 || store.before != 42 || !strings.Contains(response.Body.String(), `"records":[]`) || len(auditor.decisions) != 1 || auditor.decisions[0].decision != "allowed" {
		t.Fatalf("response %d body %q query %+v", response.Code, response.Body.String(), store)
	}
}

func TestHumanFindingQueryUsesSameAuthorizationAndBounds(t *testing.T) {
	store := &memoryFindingStore{records: []FindingRecord{}}
	auditor := &memoryAuditor{}
	request := humanRequest(t, "alice", "host=host-1&limit=7&before=42")
	request.URL.Path = "/v1/findings"
	response := httptest.NewRecorder()
	Handler{Findings: store, Auditor: auditor, Roles: viewerRoles(t)}.ServeHTTP(response, request)
	if response.Code != http.StatusOK || store.host != "host-1" || store.limit != 7 || store.before != 42 || !strings.Contains(response.Body.String(), `"records":[]`) {
		t.Fatalf("response %d body %q query %+v", response.Code, response.Body.String(), store)
	}
}

func TestPrincipalTypesAndBadQueriesFailClosed(t *testing.T) {
	auditor := &memoryAuditor{}
	handler := Handler{Store: &memoryBatchStore{}, Queries: &memoryQueryStore{}, Auditor: auditor, Roles: viewerRoles(t)}
	agentQuery := authenticatedRequest(t, nil)
	agentQuery.Method, agentQuery.URL.Path, agentQuery.URL.RawQuery = http.MethodGet, "/v1/events", "host=host-1"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, agentQuery)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("agent query status %d", response.Code)
	}

	humanIngest := humanRequest(t, "alice", "")
	humanIngest.Method, humanIngest.URL.Path, humanIngest.Body = http.MethodPost, "/v1/events/batch", io.NopCloser(bytes.NewReader(requestBody(t)))
	humanIngest.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, humanIngest)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("human ingest status %d", response.Code)
	}

	for _, query := range []string{"host=host-1&host=other", "host=../bad", "host=host-1&limit=201", "host=host-1&before=0", "host=host-1&extra=x"} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, humanRequest(t, "alice", query))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("query %q status %d", query, response.Code)
		}
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, humanRequest(t, "unknown", "host=host-1"))
	if response.Code != http.StatusForbidden {
		t.Fatalf("unknown user status %d", response.Code)
	}
	if len(auditor.decisions) != 1 || auditor.decisions[0].principal != "unknown" || auditor.decisions[0].decision != "denied" {
		t.Fatalf("denied decision not audited: %+v", auditor.decisions)
	}
}

func TestAllowedQueryFailsClosedWithoutDurableAudit(t *testing.T) {
	request := humanRequest(t, "alice", "host=host-1")
	response := httptest.NewRecorder()
	Handler{Queries: &memoryQueryStore{}, Roles: viewerRoles(t)}.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("query without auditor status %d", response.Code)
	}
}
