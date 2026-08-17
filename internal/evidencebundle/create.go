package evidencebundle

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type captured struct {
	entry Entry
	body  []byte
}

func Create(inputDirectory, outputPath string, now time.Time) (Manifest, error) {
	if !filepath.IsAbs(inputDirectory) || !filepath.IsAbs(outputPath) {
		return Manifest{}, fmt.Errorf("input and output paths must be absolute")
	}
	info, err := os.Lstat(inputDirectory)
	if err != nil {
		return Manifest{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Manifest{}, fmt.Errorf("input must be a non-symlink directory")
	}
	directory, err := os.Open(inputDirectory)
	if err != nil {
		return Manifest{}, err
	}
	defer directory.Close()
	names, err := directory.Readdirnames(MaxFiles + 1)
	if err != nil && err != io.EOF {
		return Manifest{}, err
	}
	if len(names) > MaxFiles {
		return Manifest{}, fmt.Errorf("input directory exceeds %d entries", MaxFiles)
	}
	jsonNames := []string{}
	for _, name := range names {
		if strings.HasSuffix(name, ".json") {
			jsonNames = append(jsonNames, name)
		}
	}
	if len(jsonNames) == 0 || len(jsonNames) > MaxFiles {
		return Manifest{}, fmt.Errorf("bundle requires 1..%d top-level JSON files", MaxFiles)
	}
	sort.Strings(jsonNames)
	files := make([]captured, 0, len(jsonNames))
	manifest := Manifest{SchemaVersion: 1, CreatedAt: now.UTC(), Entries: []Entry{}}
	for _, name := range jsonNames {
		path := filepath.Join(inputDirectory, name)
		before, err := os.Lstat(path)
		if err != nil {
			return Manifest{}, err
		}
		if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
			return Manifest{}, fmt.Errorf("evidence %q is not a regular file", name)
		}
		file, err := os.Open(path)
		if err != nil {
			return Manifest{}, err
		}
		opened, err := file.Stat()
		if err != nil {
			file.Close()
			return Manifest{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(file, MaxFileBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return Manifest{}, fmt.Errorf("read evidence %q", name)
		}
		after, err := os.Lstat(path)
		if err != nil {
			return Manifest{}, err
		}
		if !os.SameFile(before, opened) || !os.SameFile(opened, after) {
			return Manifest{}, fmt.Errorf("evidence %q changed while reading", name)
		}
		if len(body) > MaxFileBytes {
			return Manifest{}, fmt.Errorf("evidence %q exceeds file limit", name)
		}
		if !json.Valid(body) {
			return Manifest{}, fmt.Errorf("evidence %q is not valid JSON", name)
		}
		manifest.TotalBytes += int64(len(body))
		if manifest.TotalBytes > MaxTotalBytes {
			return Manifest{}, fmt.Errorf("bundle exceeds total byte limit")
		}
		sum := sha256.Sum256(body)
		entry := Entry{Name: name, Bytes: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}
		manifest.Entries = append(manifest.Entries, entry)
		files = append(files, captured{entry, body})
	}
	if err := writeArchive(outputPath, manifest, files); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func writeArchive(outputPath string, manifest Manifest, files []captured) error {
	parent := filepath.Dir(outputPath)
	temporary, err := os.CreateTemp(parent, ".watchhouse-evidence-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	gz := gzip.NewWriter(temporary)
	gz.Header.ModTime = time.Unix(0, 0)
	gz.Header.OS = 255
	tw := tar.NewWriter(gz)
	manifestBody, _ := json.MarshalIndent(manifest, "", "  ")
	manifestBody = append(manifestBody, '\n')
	items := append([]captured{{Entry{"manifest.json", int64(len(manifestBody)), ""}, manifestBody}}, files...)
	for _, item := range items {
		header := &tar.Header{Name: item.entry.Name, Mode: 0o600, Size: int64(len(item.body)), ModTime: time.Unix(0, 0), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(item.body); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryName, outputPath); err != nil {
		return fmt.Errorf("publish evidence bundle without overwrite: %w", err)
	}
	if err := os.Remove(temporaryName); err != nil {
		return err
	}
	parentDirectory, err := os.Open(parent)
	if err != nil {
		return err
	}
	defer parentDirectory.Close()
	return parentDirectory.Sync()
}
