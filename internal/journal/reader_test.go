package journal

import (
	"context"
	"errors"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"watchhouse/internal/telemetry"
)

func TestBoundedArguments(t *testing.T) {
	expected := []string{"--no-pager", "--output=json", "--lines=20", "_COMM=sshd", "+", "_COMM=sshd-session"}
	got, err := collect(context.Background(), 20, func(ctx context.Context, args []string, out, errOut io.Writer) error {
		if !reflect.DeepEqual(args, expected) {
			t.Fatalf("unexpected argv: %v", args)
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("missing subprocess timeout")
		}
		_, err := io.WriteString(out, "{}\n")
		return err
	})
	if err != nil || string(got.Data) != "{}\n" {
		t.Fatalf("capture %q error %v", got.Data, err)
	}
}

func TestFailureAndDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		diagnostic   string
		runErr, want error
	}{
		{"", errors.New("process failed"), nil},
		{"permission-limited journal", nil, ErrDiagnostics},
	} {
		got, err := collect(context.Background(), 2, func(_ context.Context, _ []string, out, diag io.Writer) error {
			io.WriteString(out, "{}\n")
			io.WriteString(diag, tc.diagnostic)
			return tc.runErr
		})
		if err == nil || len(got.Data) != 0 || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Fatalf("partial capture leaked: %v", err)
		}
	}
}

func TestLimitsAndCancellation(t *testing.T) {
	for _, limit := range []int{0, -1, 1001} {
		if _, err := collect(context.Background(), limit, func(context.Context, []string, io.Writer, io.Writer) error {
			t.Fatal("invalid limit executed")
			return nil
		}); err == nil {
			t.Fatal("invalid limit accepted")
		}
	}
	_, err := collect(context.Background(), 1, func(_ context.Context, _ []string, out, _ io.Writer) error {
		_, err := io.WriteString(out, strings.Repeat("x", telemetry.MaxRecordBytes+2))
		return err
	})
	if !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("output limit not enforced: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = collect(ctx, 1, func(ctx context.Context, _ []string, _, _ io.Writer) error { return ctx.Err() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestUnsupportedPlatform(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-Linux behavior only")
	}
	if _, err := Read(context.Background(), 10); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unexpected platform result %v", err)
	}
}
