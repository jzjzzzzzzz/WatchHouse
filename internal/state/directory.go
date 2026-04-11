// Package state prepares private local state locations. It never repairs the
// permissions of a preexisting path silently or follows a final symlink.
package state

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type Directory struct{ Path string }

func Prepare(path string) (Directory, error) {
	if path == "" {
		return Directory{}, fmt.Errorf("state directory required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Directory{}, err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return Directory{}, fmt.Errorf("state parent must exist: %w", err)
	}
	abs = filepath.Join(parent, filepath.Base(abs))
	if err := os.Mkdir(abs, 0700); err != nil && !os.IsExist(err) {
		return Directory{}, fmt.Errorf("create state directory: %w", err)
	}
	if err := check(abs, true); err != nil {
		return Directory{}, err
	}
	return Directory{Path: abs}, nil
}

// File reserves an ordinary, owner-only state file before a database driver
// opens it. The private directory is the security boundary for sidecar files.
func (d Directory) File(name string) (string, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmt.Errorf("state filename must be a basename")
	}
	if err := check(d.Path, true); err != nil {
		return "", err
	}
	path := filepath.Join(d.Path, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("reserve state file: %w", err)
	}
	if f != nil {
		if err := f.Close(); err != nil {
			return "", err
		}
	}
	if err := check(path, false); err != nil {
		return "", err
	}
	return path, nil
}

func check(path string, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect state path: %w", err)
	}
	want := os.FileMode(0600)
	if directory {
		want = 0700
	}
	if info.Mode()&os.ModeSymlink != 0 || info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) || info.Mode().Perm() != want {
		return fmt.Errorf("state path must be a non-symlink private file or directory with mode %04o", want)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("state path must be owned by the current user")
	}
	return nil
}
