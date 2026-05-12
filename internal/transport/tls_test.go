package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testPKI struct{ ca, serverCert, serverKey, clientCert, clientKey string }

func makeTestPKI(t *testing.T, kind, clientIdentity string) testPKI {
	t.Helper()
	directory := t.TempDir()
	now := time.Now()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Watchhouse test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	writePEM := func(name, blockType string, body []byte, mode os.FileMode) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: body}), mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	caPath := writePEM("ca.pem", "CERTIFICATE", caDER, 0o644)
	issue := func(serial int64, name string, dns []string, uris bool, usages []x509.ExtKeyUsage) (string, string) {
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
			NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), DNSNames: dns,
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usages}
		if uris {
			var identity *url.URL
			var err error
			if kind == "host" {
				identity, err = HostURI(clientIdentity)
			} else if kind == "user" {
				identity, err = UserURI(clientIdentity)
			} else {
				t.Fatal("invalid test identity kind")
			}
			if err != nil {
				t.Fatal(err)
			}
			template.URIs = []*url.URL{identity}
		}
		der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		keyDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		return writePEM(name+".pem", "CERTIFICATE", der, 0o644), writePEM(name+"-key.pem", "PRIVATE KEY", keyDER, 0o600)
	}
	serverCert, serverKey := issue(2, "server", []string{"control.test"}, false, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCert, clientKey := issue(3, "agent", nil, true, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	return testPKI{caPath, serverCert, serverKey, clientCert, clientKey}
}

func TestLoadMutualTLSMaterial(t *testing.T) {
	files := makeTestPKI(t, "host", "host-1")
	client, host, err := LoadClientTLS(files.ca, files.clientCert, files.clientKey, "control.test")
	if err != nil || host != "host-1" || client.MinVersion != tls.VersionTLS13 || client.InsecureSkipVerify {
		t.Fatalf("client host %q config %+v error %v", host, client, err)
	}
	server, err := LoadServerTLS(files.ca, files.serverCert, files.serverKey, "control.test")
	if err != nil || server.ClientAuth != tls.RequireAndVerifyClientCert || server.MinVersion != tls.VersionTLS13 {
		t.Fatalf("server %+v %v", server, err)
	}
}

func TestTLSMaterialRejectsWeakPathsAndIdentity(t *testing.T) {
	files := makeTestPKI(t, "host", "host-1")
	if err := os.Chmod(files.clientKey, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadClientTLS(files.ca, files.clientCert, files.clientKey, "control.test"); err == nil {
		t.Fatal("world-readable client key accepted")
	}
	if err := os.Chmod(files.clientKey, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(files.clientKey), "key-link.pem")
	if err := os.Symlink(files.clientKey, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadClientTLS(files.ca, files.clientCert, link, "control.test"); err == nil {
		t.Fatal("symlink key accepted")
	}
	if _, err := LoadServerTLS(files.ca, files.serverCert, files.serverKey, "wrong.test"); err == nil {
		t.Fatal("wrong server name accepted")
	}
	if _, _, err := LoadClientTLS("relative-ca.pem", files.clientCert, files.clientKey, "control.test"); err == nil {
		t.Fatal("relative TLS path accepted")
	}
}

func TestHumanTLSLoaderRequiresHumanURIKind(t *testing.T) {
	files := makeTestPKI(t, "user", "alice")
	configuration, user, err := LoadHumanTLS(files.ca, files.clientCert, files.clientKey, "control.test")
	if err != nil || user != "alice" || len(configuration.Certificates) != 1 {
		t.Fatalf("human identity %q config %+v error %v", user, configuration, err)
	}
	if _, _, err := LoadClientTLS(files.ca, files.clientCert, files.clientKey, "control.test"); err == nil {
		t.Fatal("human certificate accepted by agent TLS loader")
	}
}
