//go:build linux

package firewall

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

const maxStderrBytes = 16 << 10

var nftPaths = []string{"/usr/sbin/nft", "/sbin/nft"}

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
	stdout := boundedio.NewWriter(MaxRulesetBytes)
	stderr := boundedio.NewWriter(maxStderrBytes)
	cmd := exec.CommandContext(ctx, path, "--json", "list", "ruleset")
	cmd.Stdin = bytes.NewReader(nil)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return Snapshot{}, fmt.Errorf("nft command timeout: %w", ctx.Err())
	}
	if err != nil {
		if errors.Is(err, boundedio.ErrLimit) {
			return Snapshot{}, boundedio.ErrLimit
		}
		return Snapshot{}, fmt.Errorf("nft list ruleset: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return Parse(stdout.Bytes(), now())
}
