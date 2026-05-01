package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"watchhouse/internal/strictjson"
)

var ErrEventConflict = errors.New("authenticated event identity has conflicting content")

type BatchStore interface {
	CommitBatch(context.Context, string, []Item) error
}

type Handler struct{ Store BatchStore }

func (handler Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/v1/events/batch" {
		http.NotFound(response, request)
		return
	}
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
	response.WriteHeader(http.StatusOK)
	// A write failure occurs after durable commit. The client will not Ack and
	// retries; server-side event identity makes that retry idempotent.
	_ = encodeResponse(response, BatchResponse{SchemaVersion: SchemaVersion, Receipts: receipts})
}

func authenticatedHost(state *tls.ConnectionState) (string, error) {
	if state == nil || len(state.PeerCertificates) == 0 || len(state.VerifiedChains) == 0 {
		return "", fmt.Errorf("unverified client certificate")
	}
	leaf := state.PeerCertificates[0]
	if len(state.VerifiedChains[0]) == 0 || !state.VerifiedChains[0][0].Equal(leaf) {
		return "", fmt.Errorf("verified chain does not identify peer leaf")
	}
	return HostFromCertificate(leaf)
}
