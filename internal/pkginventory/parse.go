package pkginventory

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

func ParseDPKG(raw []byte, observedAt time.Time) (Report, error) {
	if len(raw) == 0 || len(raw) > MaxOutputBytes {
		return Report{}, fmt.Errorf("package output size must be 1..%d bytes", MaxOutputBytes)
	}
	packages := []Package{}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 256<<10)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 3 || !validField(fields[0], 256) || !validField(fields[1], 1024) || !validField(fields[2], 128) {
			return Report{}, fmt.Errorf("malformed package inventory line")
		}
		key := strings.Join(fields, "\x00")
		if seen[key] {
			return Report{}, fmt.Errorf("duplicate package inventory row")
		}
		seen[key] = true
		packages = append(packages, Package{Name: fields[0], Version: fields[1], Architecture: fields[2]})
		if len(packages) > MaxPackages {
			return Report{}, fmt.Errorf("package count exceeds %d", MaxPackages)
		}
	}
	if err := scanner.Err(); err != nil {
		return Report{}, fmt.Errorf("scan package inventory: %w", err)
	}
	sort.Slice(packages, func(i, j int) bool {
		a, b := packages[i], packages[j]
		return a.Name+"\x00"+a.Architecture+"\x00"+a.Version < b.Name+"\x00"+b.Architecture+"\x00"+b.Version
	})
	sum := sha256.Sum256(raw)
	return Report{SchemaVersion: 1, ObservedAt: observedAt.UTC(), Manager: "dpkg", PackageCount: len(packages), SHA256: hex.EncodeToString(sum[:]), Packages: packages}, nil
}

func validField(value string, max int) bool {
	if value == "" || len(value) > max {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
