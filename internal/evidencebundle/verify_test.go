package evidencebundle

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifyCreatedBundle(t *testing.T) {
	input := t.TempDir()
	if err := os.WriteFile(filepath.Join(input, "event.json"), []byte(`{"id":"one"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "bundle.tar.gz")
	created, err := Create(input, archive, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	verified, err := Verify(archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(verified.Entries) != 1 || verified.Entries[0] != created.Entries[0] || verified.TotalBytes != created.TotalBytes {
		t.Fatalf("created=%#v verified=%#v", created, verified)
	}
}

func TestVerifyDetectsArchiveCorruption(t *testing.T) {
	input := t.TempDir()
	if err := os.WriteFile(filepath.Join(input, "event.json"), []byte(`{"id":"one"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if _, err := Create(input, archive, time.Now()); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(archive)
	body[len(body)/2] ^= 0xff
	corrupt := filepath.Join(t.TempDir(), "corrupt.tar.gz")
	if err := os.WriteFile(corrupt, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(corrupt); err == nil {
		t.Fatal("accepted corrupted archive")
	}
}

func TestVerifyRejectsTraversalEntry(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.tar.gz")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	body := []byte(`{}`)
	if err := tw.WriteHeader(&tar.Header{Name: "../escape.json", Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(archive); err == nil {
		t.Fatal("accepted traversal entry")
	}
}
