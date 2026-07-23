//go:build linux

package diskaudit

import (
	"fmt"
	"math/bits"
	"syscall"
	"time"
)

func inspect(path string) (Sample, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return Sample{}, err
	}
	if stat.Bsize <= 0 {
		return Sample{}, fmt.Errorf("filesystem reports invalid block size")
	}
	blockSize := uint64(stat.Bsize)
	if high, total := bits.Mul64(stat.Blocks, blockSize); high != 0 {
		return Sample{}, fmt.Errorf("total byte count overflows uint64")
	} else {
		if high, available := bits.Mul64(stat.Bavail, blockSize); high != 0 {
			return Sample{}, fmt.Errorf("available byte count overflows uint64")
		} else {
			return Sample{FilesystemType: fmt.Sprintf("0x%x", stat.Type), TotalBytes: total, AvailableBytes: available, Inodes: stat.Files, AvailableInodes: stat.Ffree}, nil
		}
	}
}

func Collect(now func() time.Time) (Report, error) {
	results, err := Evaluate(Paths, inspect)
	if err != nil {
		return Report{}, err
	}
	return NewReport(now(), results), nil
}
