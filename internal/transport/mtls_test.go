package transport

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"watchhouse/internal/spool"
)

type tlsTrackingHandler struct {
	inner   http.Handler
	version uint16
}

func (handler *tlsTrackingHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.TLS != nil {
		handler.version = request.TLS.Version
	}
	handler.inner.ServeHTTP(response, request)
}

func startMutualTLSServer(t *testing.T, files testPKI, store BatchStore) (*httptest.Server, *tlsTrackingHandler) {
	t.Helper()
	configuration, err := LoadServerTLS(files.ca, files.serverCert, files.serverKey, "control.test")
	if err != nil {
		t.Fatal(err)
	}
	tracking := &tlsTrackingHandler{inner: Handler{Store: store}}
	server := httptest.NewUnstartedServer(tracking)
	server.TLS = configuration
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, tracking
}

func TestRealMutualTLSBindsCertificateHostAndReceipts(t *testing.T) {
	files := makeTestPKI(t, "host-1")
	store := &memoryBatchStore{}
	server, tracking := startMutualTLSServer(t, files, store)
	configuration, host, err := LoadClientTLS(files.ca, files.clientCert, files.clientKey, "control.test")
	if err != nil || host != "host-1" {
		t.Fatalf("client identity %q %v", host, err)
	}
	client, err := NewClient(server.URL, configuration)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	receipts, err := client.Deliver(context.Background(), []spool.Item{queuedItem(1)})
	if err != nil || len(receipts) != 1 || store.host != "host-1" || tracking.version != tls.VersionTLS13 {
		t.Fatalf("receipts %+v store %+v TLS %x error %v", receipts, store, tracking.version, err)
	}
}

func TestRealMutualTLSRejectsClaimedHostAndUntrustedPeers(t *testing.T) {
	files := makeTestPKI(t, "host-1")
	store := &memoryBatchStore{}
	server, _ := startMutualTLSServer(t, files, store)
	configuration, _, err := LoadClientTLS(files.ca, files.clientCert, files.clientKey, "control.test")
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(server.URL, configuration)
	if err != nil {
		t.Fatal(err)
	}
	if receipts, err := client.Deliver(context.Background(), []spool.Item{queuedItemForHost(1, "other-host")}); err == nil || receipts != nil {
		t.Fatalf("claimed host receipts %+v error %v", receipts, err)
	}
	client.Close()
	if len(store.items) != 0 {
		t.Fatal("claimed-host event reached store")
	}

	rogue := makeTestPKI(t, "host-1")
	untrustedClient, _, err := LoadClientTLS(files.ca, rogue.clientCert, rogue.clientKey, "control.test")
	if err != nil {
		t.Fatal(err)
	}
	client, err = NewClient(server.URL, untrustedClient)
	if err != nil {
		t.Fatal(err)
	}
	if receipts, err := client.Deliver(context.Background(), []spool.Item{queuedItem(2)}); err == nil || receipts != nil {
		t.Fatalf("untrusted-client receipts %+v error %v", receipts, err)
	}
	client.Close()

	wrongRoots, _, err := LoadClientTLS(rogue.ca, files.clientCert, files.clientKey, "control.test")
	if err != nil {
		t.Fatal(err)
	}
	client, err = NewClient(server.URL, wrongRoots)
	if err != nil {
		t.Fatal(err)
	}
	if receipts, err := client.Deliver(context.Background(), []spool.Item{queuedItem(3)}); err == nil || receipts != nil {
		t.Fatalf("untrusted-server receipts %+v error %v", receipts, err)
	}
	client.Close()
}

func queuedItemForHost(sequence int64, host string) spool.Item {
	item := testItem(sequence, host)
	return spool.Item{Sequence: item.Sequence, EventID: item.EventID, Event: item.Event}
}
