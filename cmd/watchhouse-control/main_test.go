package main

import (
	"os"
	"testing"
)

func TestListenRequiresLiteralIPAndNonzeroPort(t *testing.T) {
	for _, valid := range []string{"127.0.0.1:8443", "0.0.0.0:443", "[::1]:8443"} {
		if err := validateListen(valid); err != nil {
			t.Fatalf("valid %q: %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "localhost:8443", "127.0.0.1", "127.0.0.1:0"} {
		if err := validateListen(invalid); err == nil {
			t.Fatalf("invalid %q accepted", invalid)
		}
	}
}

func TestControlCLIRejectsIncompleteConfiguration(t *testing.T) {
	if code := run([]string{"--help"}, os.Stderr); code != 0 {
		t.Fatalf("help code %d", code)
	}
	if code := run(nil, os.Stderr); code != 2 {
		t.Fatalf("empty code %d", code)
	}
}
