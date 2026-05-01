package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxTLSFileBytes = 1024 * 1024

func LoadClientTLS(caPath, certificatePath, keyPath, serverName string) (*tls.Config, string, error) {
	roots, err := loadCAPool(caPath)
	if err != nil {
		return nil, "", err
	}
	certificate, leaf, err := loadKeyPair(certificatePath, keyPath)
	if err != nil {
		return nil, "", err
	}
	host, err := HostFromCertificate(leaf)
	if err != nil {
		return nil, "", fmt.Errorf("client certificate identity: %w", err)
	}
	if serverName == "" {
		return nil, "", fmt.Errorf("TLS server name is required")
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots,
		Certificates: []tls.Certificate{certificate}, ServerName: serverName}, host, nil
}

func LoadServerTLS(caPath, certificatePath, keyPath, serverName string) (*tls.Config, error) {
	clients, err := loadCAPool(caPath)
	if err != nil {
		return nil, err
	}
	certificate, leaf, err := loadKeyPair(certificatePath, keyPath)
	if err != nil {
		return nil, err
	}
	if serverName == "" {
		return nil, fmt.Errorf("TLS server name is required")
	}
	if err := leaf.VerifyHostname(serverName); err != nil {
		return nil, fmt.Errorf("server certificate name: %w", err)
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clients}, nil
}

func loadCAPool(path string) (*x509.CertPool, error) {
	body, err := readTLSFile(path, false)
	if err != nil {
		return nil, fmt.Errorf("read private CA bundle: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(body) || len(pool.Subjects()) == 0 {
		return nil, fmt.Errorf("private CA bundle contains no certificates")
	}
	return pool, nil
}

func loadKeyPair(certificatePath, keyPath string) (tls.Certificate, *x509.Certificate, error) {
	certificatePEM, err := readTLSFile(certificatePath, false)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read TLS certificate: %w", err)
	}
	keyPEM, err := readTLSFile(keyPath, true)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read TLS private key: %w", err)
	}
	pair, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("parse TLS key pair: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return tls.Certificate{}, nil, fmt.Errorf("TLS certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("parse TLS leaf: %w", err)
	}
	pair.Leaf = leaf
	return pair, leaf, nil
}

func readTLSFile(path string, private bool) ([]byte, error) {
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("TLS paths must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("TLS material must be a regular non-symlink file")
	}
	if private && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("TLS private key has group or other permissions")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxTLSFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxTLSFileBytes {
		return nil, fmt.Errorf("TLS material exceeds %d bytes", maxTLSFileBytes)
	}
	return body, nil
}
