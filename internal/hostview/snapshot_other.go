//go:build !linux

package hostview

import "fmt"

func Snapshot() (HostSnapshot, error) {
	return HostSnapshot{}, fmt.Errorf("TCP listener snapshot requires Linux procfs")
}
