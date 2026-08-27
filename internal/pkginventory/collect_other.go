//go:build !linux

package pkginventory

import (
	"context"
	"fmt"
	"time"
)

func Collect(context.Context, func() time.Time) (Report, error) {
	return Report{}, fmt.Errorf("package inventory requires Linux with dpkg")
}
