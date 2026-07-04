//go:build linux

package sshaudit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

const auditContext = "user=root,host=localhost,addr=127.0.0.1"

var sshdPaths = []string{"/usr/sbin/sshd", "/sbin/sshd"}

func Collect(ctx context.Context, now func() time.Time) (Report, error) {
	path := ""
	for _, candidate := range sshdPaths {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o022 == 0 {
			path = candidate
			break
		}
	}
	if path == "" {
		return Report{}, fmt.Errorf("trusted sshd executable not found")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-T", "-C", auditContext)
	cmd.Stdin = bytes.NewReader(nil)
	var stderr bytes.Buffer
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Report{}, err
	}
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return Report{}, fmt.Errorf("start sshd config test: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(stdout, MaxOutputBytes+1))
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return Report{}, fmt.Errorf("sshd config test timeout: %w", ctx.Err())
	}
	if readErr != nil {
		return Report{}, fmt.Errorf("read sshd config test: %w", readErr)
	}
	if waitErr != nil {
		return Report{}, fmt.Errorf("sshd config test: %w: %s", waitErr, bytes.TrimSpace(stderr.Bytes()))
	}
	if len(body) > MaxOutputBytes {
		return Report{}, fmt.Errorf("sshd output exceeds %d bytes", MaxOutputBytes)
	}
	results, err := Evaluate(body)
	if err != nil {
		return Report{}, err
	}
	return NewReport(now(), auditContext, results), nil
}
