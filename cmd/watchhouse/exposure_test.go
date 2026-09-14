package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"watchhouse/internal/dockerports"
	"watchhouse/internal/hostview"
)

func TestRunExposureCorrelatesTwoCollectors(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runExposure(nil, &out, &errOut,
		func() (hostview.HostSnapshot, error) {
			return hostview.HostSnapshot{NetworkNamespace: "net:[1]", Listeners: []hostview.Listener{{Socket: hostview.Socket{Family: "ipv4", LocalAddress: "127.0.0.1", LocalPort: 8080, Inode: 1}}}}, nil
		},
		func(context.Context, func() time.Time) (dockerports.Report, error) {
			return dockerports.Report{Bindings: []dockerports.Binding{{Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80}}}, nil
		})
	if code != 0 || !strings.Contains(out.String(), `"listener_observed":true`) || errOut.Len() != 0 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestRunExposureStopsOnCollectorFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	dockerCalled := false
	code := runExposure(nil, &out, &errOut, func() (hostview.HostSnapshot, error) { return hostview.HostSnapshot{}, errors.New("proc denied") }, func(context.Context, func() time.Time) (dockerports.Report, error) {
		dockerCalled = true
		return dockerports.Report{}, nil
	})
	if code != 1 || dockerCalled || !strings.Contains(errOut.String(), "proc denied") {
		t.Fatalf("code=%d called=%v err=%q", code, dockerCalled, errOut.String())
	}
}

func TestRunExposureRejectsSelectionArguments(t *testing.T) {
	var out, errOut bytes.Buffer
	called := false
	code := runExposure([]string{"--namespace", "x"}, &out, &errOut, func() (hostview.HostSnapshot, error) { called = true; return hostview.HostSnapshot{}, nil }, func(context.Context, func() time.Time) (dockerports.Report, error) { return dockerports.Report{}, nil })
	if code != 2 || called {
		t.Fatalf("code=%d called=%v err=%q", code, called, errOut.String())
	}
}
