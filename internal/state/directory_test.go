package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateDirectoryAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	d, err := Prepare(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.File("queue.db")
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal("state file not private")
	}
	if again, err := Prepare(path); err != nil || again.Path != d.Path {
		t.Fatalf("reopen %v", err)
	}
	if _, err := d.File("queue.db"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectUnsafePathsWithoutRepair(t *testing.T) {
	parent := t.TempDir()
	unsafe := filepath.Join(parent, "shared")
	if err := os.Mkdir(unsafe, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(unsafe); err == nil {
		t.Fatal("unsafe directory accepted")
	}
	info, _ := os.Stat(unsafe)
	if info.Mode().Perm() != 0755 {
		t.Fatal("silently repaired existing permissions")
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(unsafe, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(link); err == nil {
		t.Fatal("symlink directory accepted")
	}
	if _, err := Prepare(""); err == nil {
		t.Fatal("empty directory accepted")
	}
	if _, err := Prepare(filepath.Join(parent, "missing", "child")); err == nil {
		t.Fatal("missing parent accepted")
	}
}

func TestRejectStateFileEscapeAndSymlinks(t *testing.T) {
	d, err := Prepare(filepath.Join(t.TempDir(), "state"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "../escape", "/etc/passwd", "sub/file"} {
		if _, err := d.File(name); err == nil {
			t.Errorf("unsafe basename %q accepted", name)
		}
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(d.Path, "linked.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.File("linked.db"); err == nil {
		t.Fatal("symlink file accepted")
	}
	if err := os.WriteFile(filepath.Join(d.Path, "public.db"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := d.File("public.db"); err == nil {
		t.Fatal("public state file accepted")
	}
	if err := os.Mkdir(filepath.Join(d.Path, "directory.db"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := d.File("directory.db"); err == nil {
		t.Fatal("directory accepted as file")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "untouched" {
		t.Fatal("symlink target modified")
	}
}
