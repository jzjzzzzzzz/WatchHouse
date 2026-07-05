package boundedio

import (
	"bytes"
	"errors"
)

var ErrLimit = errors.New("output exceeded configured limit")

type Writer struct {
	buffer bytes.Buffer
	limit  int
}

func NewWriter(limit int) *Writer { return &Writer{limit: limit} }

func (w *Writer) Write(input []byte) (int, error) {
	if w.limit < 0 {
		return 0, ErrLimit
	}
	original := len(input)
	remaining := w.limit - w.buffer.Len()
	if remaining > 0 {
		if len(input) > remaining {
			input = input[:remaining]
		}
		_, _ = w.buffer.Write(input)
	}
	if original > remaining {
		return original, ErrLimit
	}
	return original, nil
}

func (w *Writer) Bytes() []byte  { return w.buffer.Bytes() }
func (w *Writer) String() string { return w.buffer.String() }
