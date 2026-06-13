//go:build linux

package firewall

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

const maxStderrBytes = 16 << 10

var nftPaths = []string{"/usr/sbin/nft", "/sbin/nft"}

type overflowWriter struct {
	buf bytes.Buffer
	max int
}

func (w *overflowWriter) Write(p []byte) (int, error) {
	n := len(p)
	remaining := w.max - w.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = w.buf.Write(p)
	}
	if n > remaining {
		return n, errOutputLimit
	}
	return n, nil
}

var errOutputLimit = errors.New("command output exceeded configured limit")

func Collect(ctx context.Context, now func() time.Time) (Snapshot, error) {
	path := ""
	for _, candidate := range nftPaths {
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode()&0o022 == 0 {
			path = candidate
			break
		}
	}
	if path == "" {
		return Snapshot{}, fmt.Errorf("trusted nft executable not found")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stdout := &overflowWriter{max: MaxRulesetBytes}
	stderr := &overflowWriter{max: maxStderrBytes}
	cmd := exec.CommandContext(ctx, path, "--json", "list", "ruleset")
	cmd.Stdin = bytes.NewReader(nil)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return Snapshot{}, fmt.Errorf("nft command timeout: %w", ctx.Err())
	}
	if err != nil {
		if errors.Is(err, errOutputLimit) {
			return Snapshot{}, errOutputLimit
		}
		message, _ := io.ReadAll(bytes.NewReader(stderr.buf.Bytes()))
		return Snapshot{}, fmt.Errorf("nft list ruleset: %w: %s", err, bytes.TrimSpace(message))
	}
	return Parse(stdout.buf.Bytes(), now())
}
