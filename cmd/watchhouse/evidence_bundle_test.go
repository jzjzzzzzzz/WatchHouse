package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEvidenceBundleCreateAndVerify(t *testing.T) {
	input := t.TempDir()
	if err := os.WriteFile(filepath.Join(input, "result.json"), []byte(`{"passed":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "evidence.tar.gz")
	var created, createErr bytes.Buffer
	if code := runEvidenceBundle([]string{"create", "--input", input, "--output", archive}, &created, &createErr); code != 0 {
		t.Fatalf("code=%d out=%q err=%q", code, created.String(), createErr.String())
	}
	if !strings.Contains(created.String(), `"name":"result.json"`) {
		t.Fatalf("manifest=%q", created.String())
	}
	var verified, verifyErr bytes.Buffer
	if code := runEvidenceBundle([]string{"verify", "--archive", archive}, &verified, &verifyErr); code != 0 {
		t.Fatalf("code=%d out=%q err=%q", code, verified.String(), verifyErr.String())
	}
	if verified.String() != created.String() {
		t.Fatalf("created=%q verified=%q", created.String(), verified.String())
	}
}

func TestRunEvidenceBundleRejectsInvalidUseAndCorruption(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"create"}, {"verify"}} {
		var out, errOut bytes.Buffer
		if code := runEvidenceBundle(args, &out, &errOut); code != 2 {
			t.Fatalf("args=%v code=%d err=%q", args, code, errOut.String())
		}
	}
	archive := filepath.Join(t.TempDir(), "bad.tar.gz")
	if err := os.WriteFile(archive, []byte("not gzip"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := runEvidenceBundle([]string{"verify", "--archive", archive}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "open gzip") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}
