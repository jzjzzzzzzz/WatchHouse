package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	for _, test := range []struct {
		args  []string
		input string
		code  int
	}{
		{nil, "", 0}, {[]string{"help"}, "", 0}, {[]string{"other"}, "", 2},
		{[]string{"replay", "-h"}, "", 0}, {[]string{"replay", "--no-such-flag"}, "", 2},
		{[]string{"replay", "--host", "lab-1", "extra"}, "", 2},
		{[]string{"replay"}, "", 2},
		{[]string{"replay", "--host", "lab-1"}, "", 0},
		{[]string{"replay", "--host", "lab-1"}, "{malformed}\n", 1},
		{[]string{"replay", "--host", "lab-1", "--threshold", "0"}, "", 2},
		{[]string{"snapshot", "--host", "lab-1", "--input", "-"}, "", 2},
		{[]string{"replay", "--host", "lab-1", "--input", "/no-such-watchhouse-fixture"}, "", 1},
	} {
		var out, errOut bytes.Buffer
		if code := run(test.args, strings.NewReader(test.input), &out, &errOut); code != test.code {
			t.Errorf("args %v got %d want %d: %s", test.args, code, test.code, &errOut)
		}
	}
}

func TestFixtureCLIFileAndStdin(t *testing.T) {
	path := "../../tests/fixtures/ssh-sequence.journal.jsonl"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{path, "-"} {
		var out, errOut bytes.Buffer
		code := run([]string{"replay", "--host", "lab-1", "--input", input}, bytes.NewReader(data), &out, &errOut)
		if code != 0 {
			t.Fatalf("code %d: %s", code, &errOut)
		}
		if !strings.Contains(out.String(), `"type":"finding"`) {
			t.Fatal("finding missing")
		}
		var summary map[string]any
		if err := json.Unmarshal(errOut.Bytes(), &summary); err != nil {
			t.Fatal("summary must be standalone JSON")
		}
	}
}
