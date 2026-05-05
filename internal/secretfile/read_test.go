package secretfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPrivateBoundedSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := Read(path, 32)
	if err != nil || value != "value" {
		t.Fatalf("value %q error %v", value, err)
	}
}

func TestRejectWeakSecretFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "secret")
	if err := os.WriteFile(path, []byte("value"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path, 32); err == nil {
		t.Fatal("world-readable secret accepted")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path string
		max  int64
	}{{link, 32}, {"relative", 32}, {path, 2}, {path, 0}} {
		if _, err := Read(test.path, test.max); err == nil {
			t.Fatalf("accepted path %q max %d", test.path, test.max)
		}
	}
	if err := os.WriteFile(path, []byte("two\nlines\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path, 32); err == nil {
		t.Fatal("multiline secret accepted")
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 33)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path, 32); err == nil {
		t.Fatal("oversized secret accepted")
	}
}
