//go:build linux

package fileaudit

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func inspect(path string) (Metadata, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Metadata{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Metadata{}, fmt.Errorf("stat metadata unavailable")
	}
	return Metadata{UID: stat.Uid, GID: stat.Gid, Mode: uint32(info.Mode().Perm()), Regular: info.Mode().IsRegular(), Symlink: info.Mode()&os.ModeSymlink != 0}, nil
}

func Collect(now func() time.Time) (Report, error) {
	results, err := Evaluate(ServerFiles, inspect)
	if err != nil {
		return Report{}, err
	}
	return NewReport(now(), results), nil
}
