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
		{[]string{"self"}, "", 0}, {[]string{"self", "/proc/1/environ"}, "", 2},
		{[]string{"deliver", "--help"}, "", 0}, {[]string{"deliver"}, "", 2},
		{[]string{"query-events", "--help"}, "", 0}, {[]string{"query-events"}, "", 2},
		{[]string{"query-findings", "--help"}, "", 0}, {[]string{"query-findings"}, "", 2},
		{[]string{"report-listeners", "--help"}, "", 0}, {[]string{"report-listeners"}, "", 2},
		{[]string{"deliver-listeners", "--help"}, "", 0}, {[]string{"deliver-listeners"}, "", 2},
		{[]string{"probe-https", "--help"}, "", 0}, {[]string{"probe-https"}, "", 2},
		{[]string{"report-probe", "--help"}, "", 0}, {[]string{"report-probe"}, "", 2},
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
