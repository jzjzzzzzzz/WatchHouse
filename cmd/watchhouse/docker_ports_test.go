package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/dockerports"
)

func TestRunDockerPortsEmitsReport(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runDockerPorts(nil, &out, &errOut, func(context.Context, func() time.Time) (dockerports.Report, error) {
		return dockerports.Report{SchemaVersion: 1, ContainerCount: 1, PublishedBindingCount: 1, Bindings: []dockerports.Binding{{ContainerName: "edge", Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 8443, ContainerPort: 443}}}, nil
	})
	if code != 0 || !strings.Contains(out.String(), `"host_port":8443`) || errOut.Len() != 0 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestRunDockerPortsRejectsCallerSelection(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runDockerPorts([]string{"container-id"}, &out, &errOut, func(context.Context, func() time.Time) (dockerports.Report, error) {
		called = true
		return dockerports.Report{}, nil
	})
	if code != 2 || called || !strings.Contains(errOut.String(), "no sockets") {
		t.Fatalf("code=%d called=%v err=%q", code, called, errOut.String())
	}
}

func TestRunDockerPortsReportsCollectionFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runDockerPorts(nil, &out, &errOut, func(context.Context, func() time.Time) (dockerports.Report, error) {
		return dockerports.Report{}, errors.New("permission denied")
	})
	if code != 1 || out.Len() != 0 || !strings.Contains(errOut.String(), "permission denied") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}
