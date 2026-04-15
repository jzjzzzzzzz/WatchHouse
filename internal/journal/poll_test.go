package journal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func pollFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../../tests/fixtures/ssh-sequence.journal.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPollStartsAtVerifiedCursorNotTail(t *testing.T) {
	data := pollFixture(t)
	wanted := []string{"--no-pager", "--output=json", "--no-tail", "--cursor=s=fixture;i=1", "_COMM=sshd", "+", "_COMM=sshd-session"}
	got, err := poll(context.Background(), "s=fixture;i=1", 2, func(ctx context.Context, args []string, consume func(io.Reader) error, _ io.Writer) error {
		if !reflect.DeepEqual(args, wanted) {
			t.Fatalf("unsafe resume args %v", args)
		}
		err := consume(bytes.NewReader(data))
		if ctx.Err() == nil {
			t.Fatal("bounded capture did not stop child")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(got.Data)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `s=fixture;i=2`) || !strings.Contains(lines[1], `s=fixture;i=3`) {
		t.Fatalf("selected tail or checkpoint itself: %s", got.Data)
	}
}

func TestPollInitialHeadAndNoNewRecords(t *testing.T) {
	data := pollFixture(t)
	got, err := poll(context.Background(), "", 1, func(_ context.Context, args []string, consume func(io.Reader) error, _ io.Writer) error {
		for _, arg := range args {
			if strings.HasPrefix(arg, "--lines") || strings.HasPrefix(arg, "--cursor") {
				t.Fatal("initial head unexpectedly tailed")
			}
		}
		return consume(bytes.NewReader(data))
	})
	if err != nil || !strings.Contains(string(got.Data), "s=fixture;i=1") {
		t.Fatal("initial record skipped")
	}
	first := bytes.Split(data, []byte("\n"))[0]
	got, err = poll(context.Background(), "s=fixture;i=1", 2, func(_ context.Context, _ []string, consume func(io.Reader) error, _ io.Writer) error {
		return consume(bytes.NewReader(first))
	})
	if err != nil || len(got.Data) != 0 {
		t.Fatal("valid cursor without new rows rejected")
	}
}

func TestPollGapAndMalformedDiscardWholeCapture(t *testing.T) {
	for _, data := range [][]byte{nil, pollFixture(t), []byte("{malformed}\n")} {
		got, err := poll(context.Background(), "missing-cursor", 3, func(_ context.Context, _ []string, consume func(io.Reader) error, _ io.Writer) error {
			return consume(bytes.NewReader(data))
		})
		if err == nil || len(got.Data) != 0 {
			t.Fatal("unverified source returned records")
		}
	}
	got, err := poll(context.Background(), "missing-cursor", 3, func(_ context.Context, _ []string, consume func(io.Reader) error, _ io.Writer) error {
		return consume(strings.NewReader(""))
	})
	if !errors.Is(err, ErrCursorGap) || len(got.Data) != 0 {
		t.Fatalf("empty gap %v", err)
	}
}

func TestPollDiagnosticsAndExecutionFailures(t *testing.T) {
	got, err := poll(context.Background(), "", 1, func(_ context.Context, _ []string, consume func(io.Reader) error, diag io.Writer) error {
		io.WriteString(diag, "limited access")
		return consume(bytes.NewReader(pollFixture(t)))
	})
	if !errors.Is(err, ErrDiagnostics) || len(got.Data) != 0 {
		t.Fatal("permission-limited view accepted")
	}
	_, err = poll(context.Background(), "", 1, func(context.Context, []string, func(io.Reader) error, io.Writer) error {
		return errors.New("process failure")
	})
	if err == nil {
		t.Fatal("process failure ignored")
	}
	for _, cursor := range []string{"bad\ncursor", strings.Repeat("x", 4097)} {
		if _, err := poll(context.Background(), cursor, 1, nil); err == nil {
			t.Fatal("bad cursor accepted")
		}
	}
	if _, err := poll(context.Background(), "", 0, nil); err == nil {
		t.Fatal("bad limit accepted")
	}
}
