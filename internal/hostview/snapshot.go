package hostview

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxProcEntries = 65536
	maxPIDs        = 32768
	maxFDs         = 4096
)

var (
	bootIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	netNSPattern  = regexp.MustCompile(`^net:\[[0-9]+\]$`)
	socketPattern = regexp.MustCompile(`^socket:\[([0-9]+)\]$`)
)

type Listener struct {
	Socket
	Ownership string            `json:"ownership"`
	Owners    []ProcessIdentity `json:"owners"`
}

type Quality struct {
	ProcessDirectories int  `json:"process_directories"`
	ProcessesScanned   int  `json:"processes_scanned"`
	PermissionDenied   int  `json:"permission_denied"`
	Vanished           int  `json:"vanished"`
	Malformed          int  `json:"malformed"`
	PIDTruncated       bool `json:"pid_truncated"`
	FDTruncated        int  `json:"fd_truncated_processes"`
}

type HostSnapshot struct {
	Type             string     `json:"type"`
	ObservedAt       time.Time  `json:"observed_at"`
	BootID           string     `json:"boot_id"`
	NetworkNamespace string     `json:"network_namespace"`
	Listeners        []Listener `json:"listeners"`
	Quality          Quality    `json:"quality"`
}

func scanProc(root string, observedAt time.Time) (HostSnapshot, error) {
	result := HostSnapshot{Type: "tcp_listener_snapshot", ObservedAt: observedAt.UTC()}
	boot, err := readBounded(filepath.Join(root, "sys/kernel/random/boot_id"), 128)
	if err != nil {
		return result, fmt.Errorf("read valid kernel boot ID: %w", err)
	}
	if !bootIDPattern.MatchString(strings.TrimSpace(boot)) {
		return result, fmt.Errorf("read valid kernel boot ID: invalid format")
	}
	result.BootID = strings.TrimSpace(boot)
	ns, err := os.Readlink(filepath.Join(root, "self/ns/net"))
	if err != nil {
		return result, fmt.Errorf("read valid network namespace: %w", err)
	}
	if !netNSPattern.MatchString(ns) {
		return result, fmt.Errorf("read valid network namespace: invalid format")
	}
	result.NetworkNamespace = ns
	var sockets []Socket
	for _, table := range []struct{ path, family string }{{"net/tcp", "ipv4"}, {"net/tcp6", "ipv6"}} {
		file, err := os.Open(filepath.Join(root, table.path))
		if err != nil {
			return result, fmt.Errorf("open proc %s: %w", table.family, err)
		}
		parsed, parseErr := ParseProcNetTCP(file, table.family)
		closeErr := file.Close()
		if parseErr != nil {
			return result, parseErr
		}
		if closeErr != nil {
			return result, fmt.Errorf("close proc %s: %w", table.family, closeErr)
		}
		sockets = append(sockets, parsed...)
	}
	owners := make(map[uint64][]ProcessIdentity)
	wanted := make(map[uint64]struct{}, len(sockets))
	for _, socket := range sockets {
		wanted[socket.Inode] = struct{}{}
	}
	entries, truncated, err := readDirBounded(root, maxProcEntries)
	if err != nil {
		return result, fmt.Errorf("list proc: %w", err)
	}
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err == nil && pid > 0 && entry.IsDir() {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	if truncated || len(pids) > maxPIDs {
		result.Quality.PIDTruncated = true
		if len(pids) > maxPIDs {
			pids = pids[:maxPIDs]
		}
	}
	result.Quality.ProcessDirectories = len(pids)
	for _, pid := range pids {
		scanProcess(root, pid, wanted, owners, &result.Quality)
	}
	for _, socket := range sockets {
		listener := Listener{Socket: socket, Owners: owners[socket.Inode]}
		sort.Slice(listener.Owners, func(i, j int) bool {
			if listener.Owners[i].PID != listener.Owners[j].PID {
				return listener.Owners[i].PID < listener.Owners[j].PID
			}
			return listener.Owners[i].StartTimeTicks < listener.Owners[j].StartTimeTicks
		})
		switch {
		case len(listener.Owners) > 0:
			listener.Ownership = "attributed"
		case result.Quality.PermissionDenied > 0:
			listener.Ownership = "unknown_permission"
		case result.Quality.PIDTruncated || result.Quality.FDTruncated > 0 || result.Quality.Vanished > 0 || result.Quality.Malformed > 0:
			listener.Ownership = "unknown_partial"
		default:
			listener.Ownership = "unknown_unmapped"
		}
		result.Listeners = append(result.Listeners, listener)
	}
	sort.Slice(result.Listeners, func(i, j int) bool {
		a, b := result.Listeners[i], result.Listeners[j]
		if a.Family != b.Family {
			return a.Family < b.Family
		}
		if a.LocalAddress != b.LocalAddress {
			return a.LocalAddress < b.LocalAddress
		}
		if a.LocalPort != b.LocalPort {
			return a.LocalPort < b.LocalPort
		}
		return a.Inode < b.Inode
	})
	return result, nil
}

func scanProcess(root string, pid int, wanted map[uint64]struct{}, owners map[uint64][]ProcessIdentity, quality *Quality) {
	base := filepath.Join(root, strconv.Itoa(pid))
	beforeBody, err := readBounded(filepath.Join(base, "stat"), 4096)
	if err != nil {
		classifyProcessError(err, quality)
		return
	}
	comm, before, err := parseProcessStat(beforeBody, pid)
	if err != nil {
		quality.Malformed++
		return
	}
	status, err := readBounded(filepath.Join(base, "status"), 64*1024)
	if err != nil {
		classifyProcessError(err, quality)
		return
	}
	uid, err := parseEffectiveUID(status)
	if err != nil {
		quality.Malformed++
		return
	}
	cgroup, err := readBounded(filepath.Join(base, "cgroup"), 64*1024)
	if err != nil {
		classifyProcessError(err, quality)
		return
	}
	unit, err := parseSystemdUnit(cgroup)
	if err != nil {
		quality.Malformed++
		return
	}
	fds, truncated, err := readDirBounded(filepath.Join(base, "fd"), maxFDs)
	if err != nil {
		classifyProcessError(err, quality)
		return
	}
	if truncated {
		quality.FDTruncated++
	}
	inodes := make(map[uint64]struct{})
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(base, "fd", fd.Name()))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if errors.Is(err, fs.ErrPermission) {
				quality.PermissionDenied++
				return
			}
			quality.Malformed++
			return
		}
		match := socketPattern.FindStringSubmatch(target)
		if match == nil {
			continue
		}
		inode, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil || inode == 0 {
			quality.Malformed++
			return
		}
		if _, exists := wanted[inode]; exists {
			inodes[inode] = struct{}{}
		}
	}
	afterBody, err := readBounded(filepath.Join(base, "stat"), 4096)
	if err != nil {
		classifyProcessError(err, quality)
		return
	}
	_, after, err := parseProcessStat(afterBody, pid)
	if err != nil {
		quality.Malformed++
		return
	}
	if before != after {
		quality.Vanished++
		return
	}
	quality.ProcessesScanned++
	identity := ProcessIdentity{PID: pid, StartTimeTicks: before, EffectiveUID: uid, Comm: comm, SystemdUnit: unit}
	for inode := range inodes {
		owners[inode] = append(owners[inode], identity)
	}
}

func classifyProcessError(err error, quality *Quality) {
	switch {
	case errors.Is(err, fs.ErrPermission):
		quality.PermissionDenied++
	case errors.Is(err, fs.ErrNotExist):
		quality.Vanished++
	default:
		quality.Malformed++
	}
}

func readBounded(path string, limit int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > limit {
		return "", fmt.Errorf("file exceeds %d-byte bound", limit)
	}
	return string(body), nil
}

func readDirBounded(path string, limit int) ([]fs.DirEntry, bool, error) {
	directory, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, false, err
	}
	if len(entries) > limit {
		return entries[:limit], true, nil
	}
	return entries, false, nil
}
