package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpoolCLIIngestReopenAndInspect(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	fixture := "../../tests/fixtures/ssh-sequence.journal.jsonl"
	for _, args := range [][]string{
		{"spool", "init", "--state", dir},
		{"spool", "ingest", "--state", dir, "--host", "lab-1", "--input", fixture},
		{"spool", "ingest", "--state", dir, "--host", "lab-1", "--input", fixture},
		{"spool", "status", "--state", dir},
		{"spool", "check", "--state", dir},
	} {
		var out, errOut bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &errOut); code != 0 {
			t.Fatalf("args %v code %d %s", args, code, &errOut)
		}
		var item map[string]any
		if err := json.Unmarshal(out.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		if args[1] == "status" && item["stats"].(map[string]any)["pending_records"] != float64(7) {
			t.Fatal("restart changed record count")
		}
		if args[1] == "check" && item["result"].(map[string]any)["valid"] != true {
			t.Fatal("audit did not validate persisted queue")
		}
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"spool", "peek", "--state", dir, "--limit", "2"}, strings.NewReader(""), &out, &errOut); code != 0 {
		t.Fatalf("peek %d %s", code, &errOut)
	}
	if len(strings.Split(strings.TrimSpace(out.String()), "\n")) != 2 {
		t.Fatal("peek limit ignored")
	}
}

func TestSpoolCLICapacityAndMalformedInput(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	var out, errOut bytes.Buffer
	code := run([]string{"spool", "ingest", "--state", dir, "--host", "lab-1", "--max-records", "1", "--input", "../../tests/fixtures/ssh-sequence.journal.jsonl"}, strings.NewReader(""), &out, &errOut)
	if code != 1 || !strings.Contains(out.String(), `"complete":false`) || !strings.Contains(errOut.String(), "capacity") {
		t.Fatalf("capacity code %d out %s error %s", code, &out, &errOut)
	}
	out.Reset()
	errOut.Reset()
	code = run([]string{"spool", "ingest", "--state", filepath.Join(t.TempDir(), "state"), "--host", "lab-1"}, strings.NewReader("{malformed}\n"), &out, &errOut)
	if code != 1 || !strings.Contains(out.String(), `"inserted":0`) {
		t.Fatal("malformed input persisted")
	}
}

func TestSpoolCLISafeArgumentFailures(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"spool"}, 2}, {[]string{"spool", "other"}, 2},
		{[]string{"spool", "init"}, 2}, {[]string{"spool", "init", "-h"}, 0},
		{[]string{"spool", "init", "--state", dir, "extra"}, 2},
		{[]string{"spool", "status", "--state", dir}, 1},
		{[]string{"spool", "peek", "--state", dir, "--limit", "501"}, 2},
		{[]string{"spool", "ingest", "--state", dir}, 2},
		{[]string{"spool", "ingest", "--state", dir, "--host", "lab-1", "--input", "/missing-fixture-watchhouse"}, 1},
	} {
		var out, errOut bytes.Buffer
		if got := run(tc.args, strings.NewReader(""), &out, &errOut); got != tc.code {
			t.Errorf("%v got %d want %d: %s", tc.args, got, tc.code, &errOut)
		}
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("invalid arguments created state")
	}
}
