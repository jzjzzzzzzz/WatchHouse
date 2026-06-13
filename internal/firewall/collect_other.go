//go:build !linux

package firewall

import (
	"context"
	"fmt"
	"time"
)

func Collect(context.Context, func() time.Time) (Snapshot, error) {
	return Snapshot{}, fmt.Errorf("nftables collection requires Linux")
}
