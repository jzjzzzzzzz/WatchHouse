package boundedio

import (
	"errors"
	"testing"
)

func TestWriterRetainsBoundedPrefix(t *testing.T) {
	w := NewWriter(5)
	if n, err := w.Write([]byte("abc")); n != 3 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if n, err := w.Write([]byte("defg")); n != 4 || !errors.Is(err, ErrLimit) {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got := w.String(); got != "abcde" {
		t.Fatalf("got %q", got)
	}
}

func TestWriterExactLimitSucceeds(t *testing.T) {
	w := NewWriter(3)
	if n, err := w.Write([]byte("abc")); n != 3 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
