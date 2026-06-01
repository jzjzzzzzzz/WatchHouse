package extprobe

import (
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func LoadRoots(path string) (*x509.CertPool, error) {
	if path == "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system trust roots: %w", err)
		}
		return roots, nil
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("probe CA path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("probe CA must be a non-writable regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || len(body) > 1024*1024 {
		return nil, fmt.Errorf("probe CA bundle has invalid size")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(body) {
		return nil, fmt.Errorf("probe CA bundle contains no certificates")
	}
	return roots, nil
}
