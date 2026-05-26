package controlstore

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"watchhouse/internal/delivery"
	"watchhouse/internal/hostview"
	"watchhouse/internal/spool"
	"watchhouse/internal/telemetry"
	"watchhouse/internal/transport"
)

type e2ePKI struct {
	server  tls.Certificate
	agent   tls.Certificate
	roots   *x509.CertPool
	clients *x509.CertPool
}

func makeE2EPKI(t *testing.T) e2ePKI {
	t.Helper()
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(100), Subject: pkix.Name{CommonName: "Watchhouse e2e CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	issue := func(serial int64, template *x509.Certificate) tls.Certificate {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		template.SerialNumber, template.NotBefore, template.NotAfter = big.NewInt(serial), now.Add(-time.Hour), now.Add(time.Hour)
		template.KeyUsage = x509.KeyUsageDigitalSignature
		der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		certificate, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
		if err != nil {
			t.Fatal(err)
		}
		return certificate
	}
	identity, _ := url.Parse("spiffe://watchhouse/host/e2e-host")
	server := issue(101, &x509.Certificate{DNSNames: []string{"control.test"}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})
	agent := issue(102, &x509.Certificate{URIs: []*url.URL{identity}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	roots, clients := x509.NewCertPool(), x509.NewCertPool()
	roots.AddCert(ca)
	clients.AddCert(ca)
	return e2ePKI{server: server, agent: agent, roots: roots, clients: clients}
}

func startE2EServer(t *testing.T, pki e2ePKI, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pki.server}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.clients}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server
}

type corruptSuccess struct{ inner http.Handler }

func (handler corruptSuccess) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	recorded := httptest.NewRecorder()
	handler.inner.ServeHTTP(recorded, request)
	if recorded.Code != http.StatusOK {
		response.WriteHeader(recorded.Code)
		_, _ = response.Write(recorded.Body.Bytes())
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(`{"schema_version":1,"receipts":[]}`))
}

func e2eEvent(index int, previous string) (telemetry.Event, spool.Checkpoint) {
	cursor := "s=e2e;i=" + string(rune('0'+index))
	event := telemetry.Event{SchemaVersion: 1, HostID: "e2e-host", BootID: "0123456789abcdef0123456789abcdef", Source: "journald", SourceCursor: cursor, ObservedAt: time.Unix(1771000000+int64(index), 0).UTC(), ReceivedAt: time.Unix(1771000100, 0).UTC(), Kind: "ssh.authentication", Authentication: telemetry.Authentication{Outcome: "failed", Method: "publickey", User: "alice", SourceIP: "192.0.2.9", SourcePort: 2222}}
	event.EventID = telemetry.Identity(event.HostID, event.BootID, event.SourceCursor)
	return event, spool.Checkpoint{HostID: event.HostID, Source: spool.SSHSource, ExpectedCursor: previous, NextCursor: cursor}
}

func TestEndToEndMutualTLSDeliveryPostgresAndReceiptRecovery(t *testing.T) {
	dsn := os.Getenv("WATCHHOUSE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL integration DSN not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	control, err := New(pool)
	if err != nil {
		t.Fatal(err)
	}
	queue, err := spool.Open(ctx, filepath.Join(t.TempDir(), "state"), spool.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Close()
	previous := ""
	for index := 1; index <= 2; index++ {
		event, checkpoint := e2eEvent(index, previous)
		if _, err := queue.Append(ctx, checkpoint, &event); err != nil {
			t.Fatal(err)
		}
		previous = checkpoint.NextCursor
	}
	pki := makeE2EPKI(t)
	server := startE2EServer(t, pki, transport.Handler{Store: control, Listeners: control})
	clientConfig := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.roots, Certificates: []tls.Certificate{pki.agent}, ServerName: "control.test"}
	client, err := transport.NewClient(server.URL, clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	result, err := delivery.Once(ctx, queue, client, 100, 1024*1024)
	client.Close()
	if err != nil || result.Acknowledged != 2 || result.PendingRecords != 0 {
		t.Fatalf("first delivery %+v error %v", result, err)
	}
	count, err := control.EventCount(ctx, "e2e-host")
	if err != nil || count != 2 {
		t.Fatalf("remote count %d error %v", count, err)
	}

	event, checkpoint := e2eEvent(3, previous)
	if _, err := queue.Append(ctx, checkpoint, &event); err != nil {
		t.Fatal(err)
	}
	badServer := startE2EServer(t, pki, corruptSuccess{inner: transport.Handler{Store: control}})
	badClient, err := transport.NewClient(badServer.URL, clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := delivery.Once(ctx, queue, badClient, 100, 1024*1024); err == nil {
		t.Fatal("corrupt receipt accepted")
	}
	badClient.Close()
	stats, err := queue.Stats(ctx)
	if err != nil || stats.PendingRecords != 1 {
		t.Fatalf("queue after lost receipt %+v error %v", stats, err)
	}
	count, err = control.EventCount(ctx, "e2e-host")
	if err != nil || count != 3 {
		t.Fatalf("remote commit before corrupt receipt count %d error %v", count, err)
	}

	retryClient, err := transport.NewClient(server.URL, clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	result, err = delivery.Once(ctx, queue, retryClient, 100, 1024*1024)
	retryClient.Close()
	if err != nil || result.Acknowledged != 1 || result.PendingRecords != 0 {
		t.Fatalf("retry %+v error %v", result, err)
	}
	count, err = control.EventCount(ctx, "e2e-host")
	if err != nil || count != 3 {
		t.Fatalf("idempotent retry count %d error %v", count, err)
	}

	snapshot := hostview.HostSnapshot{Type: "tcp_listener_snapshot", ObservedAt: time.Unix(1771001000, 0).UTC(),
		BootID: "12345678-1234-1234-1234-123456789abc", NetworkNamespace: "net:[4026531840]",
		Listeners: []hostview.Listener{{Socket: hostview.Socket{Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 8443, KernelUID: 1000, Inode: 4242}, Ownership: "unknown_unmapped"}}}
	listenerClient, err := transport.NewClient(server.URL, clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := listenerClient.PublishListenerSnapshot(ctx, "e2e-host", snapshot)
	listenerClient.Close()
	if err != nil || receipt.SnapshotID != transport.ListenerSnapshotID("e2e-host", snapshot) {
		t.Fatalf("listener receipt %+v error %v", receipt, err)
	}
	var listenerCount int
	if err := pool.QueryRow(ctx, `SELECT listener_count FROM control_listener_snapshots WHERE snapshot_id=$1`, receipt.SnapshotID).Scan(&listenerCount); err != nil || listenerCount != 1 {
		t.Fatalf("stored listener count %d error %v", listenerCount, err)
	}
}
