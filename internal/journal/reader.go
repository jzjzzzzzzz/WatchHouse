// Package journal provides a bounded Linux journalctl snapshot, not a durable
// journal subscription. It never runs a shell or elevates its own privileges.
package journal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"watchhouse/internal/telemetry"
)

var (
	ErrUnsupported = errors.New("native journal snapshot requires Linux")
	ErrDiagnostics = errors.New("journalctl reported diagnostics; journal visibility not confirmed")
	ErrOutputLimit = errors.New("journalctl output exceeds capture limit")
)

type Capture struct {
	Data []byte
}

type runner func(context.Context, []string, io.Writer, io.Writer) error

func Read(ctx context.Context, limit int) (Capture, error) {
	if runtime.GOOS != "linux" {
		return Capture{}, ErrUnsupported
	}
	return collect(ctx, limit, func(ctx context.Context, args []string, out, diagnostics io.Writer) error {
		cmd := exec.CommandContext(ctx, "/usr/bin/journalctl", args...)
		cmd.Env = []string{"LANG=C", "LC_ALL=C", "SYSTEMD_COLORS=0"}
		cmd.Stdout, cmd.Stderr = out, diagnostics
		cmd.WaitDelay = time.Second
		return cmd.Run()
	})
}

func collect(ctx context.Context, limit int, execute runner) (Capture, error) {
	if limit < 1 || limit > 1000 {
		return Capture{}, fmt.Errorf("snapshot limit must be between 1 and 1000")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var data, diagnostics bytes.Buffer
	out := &limitedWriter{buffer: &data, remaining: limit * (telemetry.MaxRecordBytes + 1)}
	errOut := &limitedWriter{buffer: &diagnostics, remaining: 16 * 1024}
	args := []string{"--no-pager", "--output=json", "--lines=" + strconv.Itoa(limit), "_COMM=sshd", "+", "_COMM=sshd-session"}
	if err := execute(ctx, args, out, errOut); err != nil {
		if ctx.Err() != nil {
			return Capture{}, fmt.Errorf("journal capture: %w", ctx.Err())
		}
		return Capture{}, fmt.Errorf("journalctl failed: %w", err)
	}
	if out.exceeded || errOut.exceeded {
		return Capture{}, ErrOutputLimit
	}
	// journalctl may exit zero after warning about permission-limited access.
	// Do not quietly treat that partial view as a confirmed complete snapshot.
	if diagnostics.Len() != 0 {
		return Capture{}, ErrDiagnostics
	}
	return Capture{Data: data.Bytes()}, nil
}

type limitedWriter struct {
	buffer    *bytes.Buffer
	remaining int
	exceeded  bool
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		w.exceeded = true
		return 0, ErrOutputLimit
	}
	n, err := w.buffer.Write(p)
	w.remaining -= n
	return n, err
}
