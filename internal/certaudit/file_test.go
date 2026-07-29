package certaudit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditFileReadsRegularAbsolutePath(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "client.pem")
	if err := os.WriteFile(path, certificatePEM(t, now.Add(-time.Hour), now.Add(90*24*time.Hour)), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := AuditFile(path, now, 30*24*time.Hour)
	if err != nil || got.Status != Pass {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestAuditFileRejectsRelativeAndSymlinkPaths(t *testing.T) {
	if _, err := AuditFile("relative.pem", time.Now(), 0); err == nil {
		t.Fatal("accepted relative path")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := AuditFile(link, time.Now(), 0); err == nil {
		t.Fatal("accepted symlink")
	}
}
