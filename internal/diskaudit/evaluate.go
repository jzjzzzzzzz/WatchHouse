package diskaudit

import (
	"fmt"
	"sort"
)

type Sample struct {
	FilesystemType  string
	TotalBytes      uint64
	AvailableBytes  uint64
	Inodes          uint64
	AvailableInodes uint64
}

type Inspector func(string) (Sample, error)

var Paths = []string{"/", "/var/lib/watchhouse"}

const (
	MinimumAvailableBytes   = uint64(1 << 30)
	MinimumAvailablePercent = 15.0
	MinimumInodePercent     = 10.0
)

func Evaluate(paths []string, inspect Inspector) ([]Result, error) {
	seen := map[string]bool{}
	results := make([]Result, 0, len(paths))
	for _, path := range paths {
		if path == "" || path[0] != '/' || seen[path] {
			return nil, fmt.Errorf("invalid or duplicate filesystem path %q", path)
		}
		seen[path] = true
		result := Result{Path: path, Expected: ">=1 GiB and >=15% blocks; >=10% inodes"}
		sample, err := inspect(path)
		if err != nil {
			result.Status, result.Detail = Error, err.Error()
			results = append(results, result)
			continue
		}
		result.FilesystemType, result.TotalBytes, result.AvailableBytes = sample.FilesystemType, sample.TotalBytes, sample.AvailableBytes
		result.Inodes, result.AvailableInodes = sample.Inodes, sample.AvailableInodes
		if sample.TotalBytes == 0 {
			result.Status, result.Detail = Error, "filesystem reports zero total bytes"
			results = append(results, result)
			continue
		}
		result.AvailablePercent = 100 * float64(sample.AvailableBytes) / float64(sample.TotalBytes)
		inodePercent := 100.0
		if sample.Inodes > 0 {
			inodePercent = 100 * float64(sample.AvailableInodes) / float64(sample.Inodes)
		}
		if sample.AvailableBytes < MinimumAvailableBytes {
			result.Status, result.Detail = Fail, "available bytes below threshold"
		} else if result.AvailablePercent < MinimumAvailablePercent {
			result.Status, result.Detail = Fail, "available block percentage below threshold"
		} else if inodePercent < MinimumInodePercent {
			result.Status, result.Detail = Fail, "available inode percentage below threshold"
		} else {
			result.Status = Pass
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results, nil
}
