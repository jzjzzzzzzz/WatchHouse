package evidencebundle

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateDeterministicBundle(t *testing.T) {
	input := t.TempDir()
	if err := os.WriteFile(filepath.Join(input, "z.json"), []byte(`{"z":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "a.json"), []byte(`{"a":2}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "notes.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 5, 20, 0, 0, 0, time.UTC)
	first, second := filepath.Join(t.TempDir(), "first.tar.gz"), filepath.Join(t.TempDir(), "second.tar.gz")
	manifest, err := Create(input, first, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Create(input, second, at); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(first)
	b, _ := os.ReadFile(second)
	if sha256.Sum256(a) != sha256.Sum256(b) {
		t.Fatal("archives are not deterministic")
	}
	if len(manifest.Entries) != 2 || manifest.Entries[0].Name != "a.json" || manifest.Entries[1].Name != "z.json" {
		t.Fatalf("%#v", manifest)
	}
	if info, err := os.Stat(first); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("info=%v err=%v", info, err)
	}
	entries := readArchive(t, first)
	if len(entries) != 3 || !json.Valid(entries["manifest.json"]) || string(entries["a.json"]) != `{"a":2}` {
		t.Fatalf("%v", entries)
	}
}

func readArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	result := map[string][]byte{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		result[header.Name] = body
	}
	return result
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	for _, body := range [][]byte{[]byte("not json"), make([]byte, MaxFileBytes+1)} {
		input := t.TempDir()
		if err := os.WriteFile(filepath.Join(input, "bad.json"), body, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Create(input, filepath.Join(t.TempDir(), "bundle.tar.gz"), time.Now()); err == nil {
			t.Fatalf("accepted %d bytes", len(body))
		}
	}
	input := t.TempDir()
	target := filepath.Join(input, "target")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(input, "link.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(input, filepath.Join(t.TempDir(), "bundle.tar.gz"), time.Now()); err == nil {
		t.Fatal("accepted symlink")
	}
}
