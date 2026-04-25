// Package hostview builds bounded, read-only Linux host evidence.
package hostview

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

const (
	maxProcNetLine = 1024
	maxSockets     = 8192
)

type Socket struct {
	Family       string `json:"family"`
	LocalAddress string `json:"local_address"`
	LocalPort    uint16 `json:"local_port"`
	KernelUID    uint32 `json:"kernel_uid"`
	Inode        uint64 `json:"inode"`
}

// ParseProcNetTCP parses one Linux proc-net table. The supported deployment
// architectures are little-endian; the format exposes IPv6 as four host-order
// 32-bit words.
func ParseProcNetTCP(input io.Reader, family string) ([]Socket, error) {
	if family != "ipv4" && family != "ipv6" {
		return nil, fmt.Errorf("unsupported address family")
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), maxProcNetLine)
	var result []Socket
	line := 0
	for scanner.Scan() {
		line++
		fields := strings.Fields(scanner.Text())
		if line == 1 && len(fields) > 0 && fields[0] == "sl" {
			continue
		}
		if len(fields) < 4 {
			return nil, fmt.Errorf("proc net %s line %d has too few fields", family, line)
		}
		if fields[3] != "0A" { // TCP_LISTEN
			continue
		}
		if len(fields) < 10 {
			return nil, fmt.Errorf("proc net %s listener line %d incomplete", family, line)
		}
		addressHex, portHex, found := strings.Cut(fields[1], ":")
		if !found {
			return nil, fmt.Errorf("proc net %s listener line %d has invalid endpoint", family, line)
		}
		address, err := parseProcAddress(addressHex, family)
		if err != nil {
			return nil, fmt.Errorf("proc net %s listener line %d: %w", family, line, err)
		}
		port, err := strconv.ParseUint(portHex, 16, 16)
		if err != nil || len(portHex) != 4 {
			return nil, fmt.Errorf("proc net %s listener line %d has invalid port", family, line)
		}
		uid, err := strconv.ParseUint(fields[7], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("proc net %s listener line %d has invalid uid", family, line)
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil || inode == 0 {
			return nil, fmt.Errorf("proc net %s listener line %d has invalid inode", family, line)
		}
		result = append(result, Socket{Family: family, LocalAddress: address.String(), LocalPort: uint16(port), KernelUID: uint32(uid), Inode: inode})
		if len(result) > maxSockets {
			return nil, fmt.Errorf("proc net %s exceeds %d listeners", family, maxSockets)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan proc net %s: %w", family, err)
	}
	if line == 0 {
		return nil, fmt.Errorf("proc net %s is empty", family)
	}
	return result, nil
}

func parseProcAddress(value, family string) (netip.Addr, error) {
	raw, err := hex.DecodeString(value)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid address encoding")
	}
	if family == "ipv4" {
		if len(raw) != 4 {
			return netip.Addr{}, fmt.Errorf("invalid IPv4 width")
		}
		reverse(raw)
		return netip.AddrFrom4([4]byte(raw)), nil
	}
	if len(raw) != 16 {
		return netip.Addr{}, fmt.Errorf("invalid IPv6 width")
	}
	for start := 0; start < len(raw); start += 4 {
		reverse(raw[start : start+4])
	}
	return netip.AddrFrom16([16]byte(raw)), nil
}

func reverse(value []byte) {
	for left, right := 0, len(value)-1; left < right; left, right = left+1, right-1 {
		value[left], value[right] = value[right], value[left]
	}
}
