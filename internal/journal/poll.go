package journal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"time"

	"watchhouse/internal/telemetry"
)

var ErrCursorGap = errors.New("native journal cursor unavailable or no longer matches the source")

type streamRunner func(context.Context, []string, func(io.Reader) error, io.Writer) error

// Poll verifies an existing cursor and captures the first records after it.
// It deliberately does not use --lines, which can select a tail and skip data.
func Poll(ctx context.Context, cursor string, limit int) (Capture, error) {
	if runtime.GOOS != "linux" {
		return Capture{}, ErrUnsupported
	}
	return poll(ctx, cursor, limit, func(ctx context.Context, args []string, consume func(io.Reader) error, diagnostics io.Writer) error {
		child, stop := context.WithCancel(ctx)
		defer stop()
		cmd := exec.CommandContext(child, "/usr/bin/journalctl", args...)
		cmd.Env = []string{"LANG=C", "LC_ALL=C", "SYSTEMD_COLORS=0"}
		cmd.Stderr = diagnostics
		cmd.WaitDelay = time.Second
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			stdout.Close()
			return err
		}
		readErr := consume(stdout)
		if readErr != nil {
			stop()
		}
		stdout.Close()
		waitErr := cmd.Wait()
		if readErr != nil {
			return readErr
		}
		return waitErr
	})
}

func poll(parent context.Context, cursor string, limit int, execute streamRunner) (Capture, error) {
	if limit < 1 || limit > 1000 || len(cursor) > 4096 {
		return Capture{}, fmt.Errorf("invalid native poll limit or cursor")
	}
	for _, r := range cursor {
		if r < 32 || r == 127 {
			return Capture{}, fmt.Errorf("invalid native cursor encoding")
		}
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	args := []string{"--no-pager", "--output=json", "--no-tail"}
	if cursor != "" {
		args = append(args, "--cursor="+cursor)
	}
	args = append(args, "_COMM=sshd", "+", "_COMM=sshd-session")
	var data, diagnostics bytes.Buffer
	errOut := &limitedWriter{buffer: &diagnostics, remaining: 16 * 1024}
	verified := cursor == ""
	cut := false
	count := 0
	err := execute(ctx, args, func(in io.Reader) error {
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 4096), telemetry.MaxRecordBytes+1)
		for scanner.Scan() {
			if err := parent.Err(); err != nil {
				return err
			}
			position, err := telemetry.JournalPosition(scanner.Bytes())
			if err != nil {
				return fmt.Errorf("native source metadata: %w", err)
			}
			if !verified {
				if position.Cursor != cursor {
					return ErrCursorGap
				}
				verified = true
				continue
			}
			data.Write(scanner.Bytes())
			data.WriteByte('\n')
			count++
			if count == limit {
				cut = true
				cancel()
				return nil
			}
		}
		return scanner.Err()
	}, errOut)
	if parent.Err() != nil {
		return Capture{}, parent.Err()
	}
	if err != nil && !cut {
		return Capture{}, fmt.Errorf("native poll: %w", err)
	}
	if errOut.exceeded {
		return Capture{}, ErrOutputLimit
	}
	if diagnostics.Len() != 0 {
		return Capture{}, ErrDiagnostics
	}
	if !verified {
		return Capture{}, ErrCursorGap
	}
	return Capture{Data: data.Bytes()}, nil
}
