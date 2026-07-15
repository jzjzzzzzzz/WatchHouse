//go:build !linux

package unitaudit

import (
	"context"
	"fmt"
	"time"
)

func Collect(context.Context, func() time.Time) (Report, error) {
	return Report{}, fmt.Errorf("systemd unit audit requires Linux")
}
