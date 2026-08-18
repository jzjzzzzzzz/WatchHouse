package evidencebundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
)

const MaxArchiveBytes = 40 << 20

func Verify(archivePath string) (Manifest, error) {
	info, err := os.Lstat(archivePath)
	if err != nil {
		return Manifest{}, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > MaxArchiveBytes {
		return Manifest{}, fmt.Errorf("archive must be a regular file of 1..%d bytes", MaxArchiveBytes)
	}
	file, err := os.Open(archivePath)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	gz, err := gzip.NewReader(io.LimitReader(file, MaxArchiveBytes+1))
	if err != nil {
		return Manifest{}, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	bodies := map[string][]byte{}
	var archiveTotal int64
	for count := 0; ; count++ {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Manifest{}, fmt.Errorf("read tar: %w", err)
		}
		if count > MaxFiles || header.Typeflag != tar.TypeReg || header.Name == "" || path.Base(header.Name) != header.Name || header.Size < 0 || header.Size > MaxFileBytes {
			return Manifest{}, fmt.Errorf("unsafe or excessive archive entry")
		}
		if _, duplicate := bodies[header.Name]; duplicate {
			return Manifest{}, fmt.Errorf("duplicate archive entry %q", header.Name)
		}
		body, err := io.ReadAll(io.LimitReader(tr, MaxFileBytes+1))
		if err != nil || int64(len(body)) != header.Size {
			return Manifest{}, fmt.Errorf("read archive entry %q", header.Name)
		}
		archiveTotal += int64(len(body))
		if archiveTotal > MaxTotalBytes+MaxFileBytes {
			return Manifest{}, fmt.Errorf("archive expands beyond total limit")
		}
		bodies[header.Name] = body
	}
	manifestBody, ok := bodies["manifest.json"]
	if !ok {
		return Manifest{}, fmt.Errorf("archive manifest is missing")
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(manifestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Manifest{}, fmt.Errorf("manifest contains trailing data")
	}
	if manifest.SchemaVersion != 1 || len(manifest.Entries) == 0 || len(manifest.Entries) > MaxFiles {
		return Manifest{}, fmt.Errorf("invalid manifest schema or entry count")
	}
	seen := map[string]bool{}
	var total int64
	for _, entry := range manifest.Entries {
		if entry.Name == "manifest.json" || path.Base(entry.Name) != entry.Name || seen[entry.Name] {
			return Manifest{}, fmt.Errorf("invalid manifest entry %q", entry.Name)
		}
		seen[entry.Name] = true
		body, ok := bodies[entry.Name]
		if !ok {
			return Manifest{}, fmt.Errorf("manifest entry %q missing", entry.Name)
		}
		sum := sha256.Sum256(body)
		if entry.Bytes != int64(len(body)) || entry.SHA256 != hex.EncodeToString(sum[:]) {
			return Manifest{}, fmt.Errorf("manifest mismatch for %q", entry.Name)
		}
		total += entry.Bytes
	}
	if len(bodies) != len(manifest.Entries)+1 || total != manifest.TotalBytes {
		return Manifest{}, fmt.Errorf("archive contains unmanifested data or total mismatch")
	}
	return manifest, nil
}
