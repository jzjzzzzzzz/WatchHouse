package certaudit

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func AuditFile(path string, now time.Time, minimumRemaining time.Duration) (Report, error) {
	if path == "" || !filepath.IsAbs(path) {
		return Report{}, fmt.Errorf("certificate path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return Report{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Report{}, fmt.Errorf("certificate must be a regular non-symlink file")
	}
	file, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, MaxPEMBytes+1))
	if err != nil {
		return Report{}, err
	}
	return Audit(body, now, minimumRemaining)
}
