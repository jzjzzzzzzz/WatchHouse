package controlstore

import (
	"testing"

	"watchhouse/internal/detection"
)

func TestProcessorConfigurationIsBounded(t *testing.T) {
	if _, err := NewProcessor(nil, detection.DefaultConfig(), 100); err == nil {
		t.Fatal("nil store accepted")
	}
	if _, err := NewProcessor(&Store{}, detection.DefaultConfig(), 100001); err == nil {
		t.Fatal("unbounded scan accepted")
	}
	bad := detection.DefaultConfig()
	bad.Threshold = 0
	if _, err := NewProcessor(&Store{}, bad, 100); err == nil {
		t.Fatal("invalid rule config accepted")
	}
}
