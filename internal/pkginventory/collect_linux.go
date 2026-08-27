//go:build linux

package pkginventory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"watchhouse/internal/boundedio"
)

var dpkgQueryPaths = []string{"/usr/bin/dpkg-query", "/bin/dpkg-query"}

func Collect(ctx context.Context, now func() time.Time) (Report, error) {
	path := ""
	for _, candidate := range dpkgQueryPaths {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o022 == 0 {
			path = candidate
			break
		}
	}
	if path == "" {
		return Report{}, fmt.Errorf("trusted dpkg-query executable not found")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	stdout, stderr := boundedio.NewWriter(MaxOutputBytes), boundedio.NewWriter(16<<10)
	cmd := exec.CommandContext(ctx, path, "-W", "-f=${binary:Package}\\t${Version}\\t${Architecture}\\n")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(nil), stdout, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Report{}, fmt.Errorf("dpkg-query timeout: %w", ctx.Err())
		}
		if errors.Is(err, boundedio.ErrLimit) {
			return Report{}, boundedio.ErrLimit
		}
		return Report{}, fmt.Errorf("dpkg-query: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return ParseDPKG(stdout.Bytes(), now())
}
