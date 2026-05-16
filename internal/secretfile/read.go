// Package secretfile reads small deployment secrets without following the final
// symlink or accepting group/world-readable files.
package secretfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Read(path string, maxBytes int64) (string, error) {
	if path == "" || !filepath.IsAbs(path) || maxBytes < 1 || maxBytes > 1024*1024 {
		return "", fmt.Errorf("secret requires an absolute path and bounded size")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o037 != 0 {
		return "", fmt.Errorf("secret must be regular, non-group-writable, and inaccessible to other users")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > maxBytes {
		return "", fmt.Errorf("secret exceeds %d bytes", maxBytes)
	}
	value := strings.TrimSuffix(string(body), "\n")
	if value == "" || strings.ContainsAny(value, "\r\n\x00") || hasControl(value) {
		return "", fmt.Errorf("secret is empty or contains line/control separators")
	}
	return value, nil
}

func hasControl(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}
