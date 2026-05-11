package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

type Handler struct {
	Store   BatchStore
	Queries EventQueryStore
	Roles   *authz.Map
}

func (handler Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/v1/events/batch":
		handler.ingest(response, request)
	case "/v1/events":
		handler.query(response, request)
	default:
		http.NotFound(response, request)
	}
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
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if request.ContentLength != 0 || len(request.TransferEncoding) != 0 {
		http.Error(response, "query request body forbidden", http.StatusBadRequest)
		return
	}
	leaf, err := authenticatedLeaf(request.TLS)
	if err != nil {
		http.Error(response, "authenticated human certificate required", http.StatusUnauthorized)
		return
	}
	user, err := UserFromCertificate(leaf)
	if err != nil {
		http.Error(response, "authenticated human certificate required", http.StatusUnauthorized)
		return
	}
	if _, allowed := handler.Roles.CanQuery(user); !allowed {
		http.Error(response, "query role required", http.StatusForbidden)
		return
	}
	if handler.Queries == nil {
		http.Error(response, "query service unavailable", http.StatusServiceUnavailable)
		return
	}
	values, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		http.Error(response, "invalid query", http.StatusBadRequest)
		return
	}
	for key := range values {
		if key != "host" && key != "limit" && key != "before" {
			http.Error(response, "invalid query", http.StatusBadRequest)
			return
		}
	}
	if len(values["host"]) != 1 || !telemetry.ValidHost(values.Get("host")) || len(values["limit"]) > 1 || len(values["before"]) > 1 {
		http.Error(response, "invalid query", http.StatusBadRequest)
		return
	}
	limit, before := 100, int64(0)
	if value := values.Get("limit"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed < 1 || parsed > 200 {
			http.Error(response, "invalid query", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	if value := values.Get("before"); value != "" {
		parsed, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil || parsed < 1 {
			http.Error(response, "invalid query", http.StatusBadRequest)
			return
		}
		before = parsed
	}
	records, err := handler.Queries.QueryEvents(request.Context(), values.Get("host"), limit, before)
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
