//go:build linux

package hostview

import "time"

func Snapshot() (HostSnapshot, error) { return scanProc("/proc", time.Now()) }
