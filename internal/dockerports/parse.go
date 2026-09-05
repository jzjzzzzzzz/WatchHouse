package dockerports

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"watchhouse/internal/strictjson"
)

type wireBinding struct {
	HostIP   string `json:"host_ip"`
	HostPort string `json:"host_port"`
}
type wireContainer struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	NetworkMode string                   `json:"network_mode"`
	Ports       map[string][]wireBinding `json:"ports"`
}

func Parse(raw []byte, observedAt time.Time) (Report, error) {
	if len(raw) > MaxInspectBytes {
		return Report{}, fmt.Errorf("Docker inspect output exceeds %d bytes", MaxInspectBytes)
	}
	report := Report{SchemaVersion: 1, ObservedAt: observedAt.UTC(), Bindings: []Binding{}}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 256<<10)
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		container, err := strictjson.Decode[wireContainer](bytes.NewReader(scanner.Bytes()), 256<<10)
		if err != nil {
			return Report{}, fmt.Errorf("decode Docker inspect row: %w", err)
		}
		if len(container.ID) != 64 || !hexString(container.ID) || container.Name == "" || len(container.Name) > 256 || container.NetworkMode == "" || len(container.NetworkMode) > 128 || seen[container.ID] {
			return Report{}, fmt.Errorf("invalid or duplicate Docker container identity")
		}
		seen[container.ID] = true
		report.ContainerCount++
		if report.ContainerCount > MaxContainers {
			return Report{}, fmt.Errorf("container count exceeds %d", MaxContainers)
		}
		for endpoint, bindings := range container.Ports {
			portText, protocol, ok := strings.Cut(endpoint, "/")
			if !ok || (protocol != "tcp" && protocol != "udp") {
				return Report{}, fmt.Errorf("invalid container endpoint %q", endpoint)
			}
			containerPort, err := parsePort(portText)
			if err != nil {
				return Report{}, err
			}
			for _, item := range bindings {
				if net.ParseIP(item.HostIP) == nil {
					return Report{}, fmt.Errorf("invalid Docker host IP")
				}
				hostPort, err := parsePort(item.HostPort)
				if err != nil {
					return Report{}, err
				}
				report.Bindings = append(report.Bindings, Binding{container.ID, strings.TrimPrefix(container.Name, "/"), container.NetworkMode, protocol, containerPort, item.HostIP, hostPort})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Report{}, err
	}
	sort.Slice(report.Bindings, func(i, j int) bool {
		a, b := report.Bindings[i], report.Bindings[j]
		return a.HostIP+fmt.Sprintf("%05d", a.HostPort)+a.Protocol+a.ContainerID < b.HostIP+fmt.Sprintf("%05d", b.HostPort)+b.Protocol+b.ContainerID
	})
	report.PublishedBindingCount = len(report.Bindings)
	return report, nil
}

func parsePort(value string) (uint16, error) {
	number, err := strconv.ParseUint(value, 10, 16)
	if err != nil || number == 0 {
		return 0, fmt.Errorf("invalid port %q", value)
	}
	return uint16(number), nil
}
func hexString(value string) bool {
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
