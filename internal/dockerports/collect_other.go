//go:build !linux

package dockerports

import (
	"context"
	"fmt"
	"time"
)

func Collect(context.Context, func() time.Time) (Report, error) {
	return Report{}, fmt.Errorf("Docker port collection requires Linux")
}
