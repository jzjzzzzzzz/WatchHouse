//go:build linux

package dockerports

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"watchhouse/internal/boundedio"
)

var dockerPaths = []string{"/usr/bin/docker", "/usr/local/bin/docker", "/bin/docker"}

const inspectTemplate = `{"id":{{json .Id}},"name":{{json .Name}},"network_mode":{{json .HostConfig.NetworkMode}},"ports":{{json .NetworkSettings.Ports}}}`

func Collect(ctx context.Context, now func() time.Time) (Report, error) {
	path := ""
	for _, candidate := range dockerPaths {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o022 == 0 {
			path = candidate
			break
		}
	}
	if path == "" {
		return Report{}, fmt.Errorf("trusted Docker CLI not found")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	listed, err := runDocker(ctx, path, 128<<10, "container", "ls", "--quiet", "--no-trunc")
	if err != nil {
		return Report{}, err
	}
	ids := strings.Fields(string(listed))
	if len(ids) > MaxContainers {
		return Report{}, fmt.Errorf("container count exceeds %d", MaxContainers)
	}
	for _, id := range ids {
		if len(id) != 64 || !hexString(id) {
			return Report{}, fmt.Errorf("Docker returned invalid container ID")
		}
	}
	if len(ids) == 0 {
		return Parse(nil, now())
	}
	args := []string{"container", "inspect", "--format", inspectTemplate}
	args = append(args, ids...)
	inspected, err := runDocker(ctx, path, MaxInspectBytes, args...)
	if err != nil {
		return Report{}, err
	}
	return Parse(inspected, now())
}

func runDocker(ctx context.Context, path string, limit int, args ...string) ([]byte, error) {
	stdout, stderr := boundedio.NewWriter(limit), boundedio.NewWriter(16<<10)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(nil), stdout, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Docker CLI timeout: %w", ctx.Err())
		}
		if errors.Is(err, boundedio.ErrLimit) {
			return nil, boundedio.ErrLimit
		}
		return nil, fmt.Errorf("Docker CLI: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return append([]byte(nil), stdout.Bytes()...), nil
}
