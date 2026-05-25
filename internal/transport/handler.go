package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"watchhouse/internal/authz"
	"watchhouse/internal/strictjson"
	"watchhouse/internal/telemetry"
)

var ErrEventConflict = errors.New("authenticated event identity has conflicting content")

type BatchStore interface {
	CommitBatch(context.Context, string, []Item) error
}

type ListenerSnapshotStore interface {
	CommitListenerSnapshot(context.Context, string, ListenerSnapshotRequest) error
}

type Handler struct {
	Store     BatchStore
	Listeners ListenerSnapshotStore
	Queries   EventQueryStore
	Findings  FindingQueryStore
	Auditor   QueryAuditor
	Roles     *authz.Map
}

func (handler Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/v1/events/batch":
		handler.ingest(response, request)
	case "/v1/events":
		handler.query(response, request)
	case "/v1/listener-snapshots":
		handler.ingestListenerSnapshot(response, request)
	case "/v1/findings":
		handler.queryFindings(response, request)
	default:
		http.NotFound(response, request)
	}
}

func (handler Handler) ingestListenerSnapshot(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if handler.Listeners == nil {
		http.Error(response, "listener receiver unavailable", http.StatusServiceUnavailable)
		return
	}
	host, err := authenticatedHost(request.TLS)
	if err != nil {
		http.Error(response, "authenticated host certificate required", http.StatusUnauthorized)
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(response, "application/json required", http.StatusUnsupportedMediaType)
		return
	}
	if request.ContentLength > MaxBodyBytes {
		http.Error(response, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	payload, err := strictjson.Decode[ListenerSnapshotRequest](request.Body, MaxBodyBytes)
	if err != nil || payload.Validate(host) != nil {
		http.Error(response, "invalid listener snapshot", http.StatusBadRequest)
		return
	}
	if err := handler.Listeners.CommitListenerSnapshot(request.Context(), host, payload); err != nil {
		http.Error(response, "listener store unavailable", http.StatusServiceUnavailable)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(response).Encode(ListenerSnapshotReceipt{SchemaVersion: SchemaVersion, SnapshotID: payload.SnapshotID})
}

func (handler Handler) queryFindings(response http.ResponseWriter, request *http.Request) {
	_, host, limit, before, ok := handler.authorizedQuery(response, request, "findings")
	if !ok {
		return
	}
	if handler.Findings == nil {
		http.Error(response, "finding service unavailable", http.StatusServiceUnavailable)
		return
	}
	records, err := handler.Findings.QueryFindings(request.Context(), host, limit, before)
	if err != nil {
		http.Error(response, "finding service unavailable", http.StatusServiceUnavailable)
		return
	}
	if records == nil {
		records = make([]FindingRecord, 0)
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = encodeFindingPage(response, FindingPage{SchemaVersion: SchemaVersion, Records: records})
}

func (handler Handler) ingest(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if handler.Store == nil {
		http.Error(response, "receiver unavailable", http.StatusServiceUnavailable)
		return
	}
	host, err := authenticatedHost(request.TLS)
	if err != nil {
		http.Error(response, "authenticated host certificate required", http.StatusUnauthorized)
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(response, "application/json required", http.StatusUnsupportedMediaType)
		return
	}
	if request.ContentLength > MaxBodyBytes {
		http.Error(response, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	batch, err := strictjson.Decode[BatchRequest](request.Body, MaxBodyBytes)
	if err != nil || batch.Validate(host) != nil {
		http.Error(response, "invalid event batch", http.StatusBadRequest)
		return
	}
	if err := handler.Store.CommitBatch(request.Context(), host, batch.Items); err != nil {
		if errors.Is(err, ErrEventConflict) {
			http.Error(response, "event identity conflict", http.StatusConflict)
		} else {
			http.Error(response, "event store unavailable", http.StatusServiceUnavailable)
		}
		return
	}
	receipts := make([]Receipt, len(batch.Items))
	for index, item := range batch.Items {
		receipts[index] = Receipt{Sequence: item.Sequence, EventID: item.EventID}
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusOK)
	// A write failure occurs after durable commit. The client will not Ack and
	// retries; server-side event identity makes that retry idempotent.
	_ = encodeResponse(response, BatchResponse{SchemaVersion: SchemaVersion, Receipts: receipts})
}

func (handler Handler) query(response http.ResponseWriter, request *http.Request) {
	_, host, limit, before, ok := handler.authorizedQuery(response, request, "events")
	if !ok {
		return
	}
	if handler.Queries == nil {
		http.Error(response, "query service unavailable", http.StatusServiceUnavailable)
		return
	}
	records, err := handler.Queries.QueryEvents(request.Context(), host, limit, before)
	if err != nil {
		http.Error(response, "query service unavailable", http.StatusServiceUnavailable)
		return
	}
	if records == nil {
		records = make([]EventRecord, 0)
	}
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	_ = encodeEventPage(response, EventPage{SchemaVersion: SchemaVersion, Records: records})
}

func (handler Handler) authorizedQuery(response http.ResponseWriter, request *http.Request, resource string) (string, string, int, int64, bool) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return "", "", 0, 0, false
	}
	if request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
		http.Error(response, "query request body forbidden", http.StatusBadRequest)
		return "", "", 0, 0, false
	}
	leaf, err := authenticatedLeaf(request.TLS)
	if err != nil {
		http.Error(response, "authenticated human certificate required", http.StatusUnauthorized)
		return "", "", 0, 0, false
	}
	user, err := UserFromCertificate(leaf)
	if err != nil {
		http.Error(response, "authenticated human certificate required", http.StatusUnauthorized)
		return "", "", 0, 0, false
	}
	values, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		http.Error(response, "invalid query", http.StatusBadRequest)
		return "", "", 0, 0, false
	}
	for key := range values {
		if key != "host" && key != "limit" && key != "before" {
			http.Error(response, "invalid query", http.StatusBadRequest)
			return "", "", 0, 0, false
		}
	}
	if len(values["host"]) != 1 || !telemetry.ValidHost(values.Get("host")) || len(values["limit"]) > 1 || len(values["before"]) > 1 {
		http.Error(response, "invalid query", http.StatusBadRequest)
		return "", "", 0, 0, false
	}
	limit, before := 100, int64(0)
	if value := values.Get("limit"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < 1 || parsed > 200 {
			http.Error(response, "invalid query", http.StatusBadRequest)
			return "", "", 0, 0, false
		}
		limit = parsed
	}
	if value := values.Get("before"); value != "" {
		parsed, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil || parsed < 1 {
			http.Error(response, "invalid query", http.StatusBadRequest)
			return "", "", 0, 0, false
		}
		before = parsed
	}
	role, allowed := handler.Roles.CanQuery(user)
	if !allowed {
		if handler.Auditor != nil {
			_ = handler.Auditor.RecordQueryDecision(request.Context(), user, role, resource, values.Get("host"), "denied")
		}
		http.Error(response, "query role required", http.StatusForbidden)
		return "", "", 0, 0, false
	}
	if handler.Auditor == nil || handler.Auditor.RecordQueryDecision(request.Context(), user, role, resource, values.Get("host"), "allowed") != nil {
		http.Error(response, "query audit unavailable", http.StatusServiceUnavailable)
		return "", "", 0, 0, false
	}
	return user, values.Get("host"), limit, before, true
}

func authenticatedLeaf(state *tls.ConnectionState) (*x509.Certificate, error) {
	if state == nil || len(state.PeerCertificates) == 0 || len(state.VerifiedChains) == 0 {
		return nil, fmt.Errorf("unverified client certificate")
	}
	leaf := state.PeerCertificates[0]
	if len(state.VerifiedChains[0]) == 0 || !state.VerifiedChains[0][0].Equal(leaf) {
		return nil, fmt.Errorf("verified chain does not identify peer leaf")
	}
	return leaf, nil
}

func authenticatedHost(state *tls.ConnectionState) (string, error) {
	leaf, err := authenticatedLeaf(state)
	if err != nil {
		return "", err
	}
	return HostFromCertificate(leaf)
}
