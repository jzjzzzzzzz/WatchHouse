//go:build !linux

package sshaudit

import (
	"context"
	"fmt"
	"time"
)

func Collect(context.Context, func() time.Time) (Report, error) {
	return Report{}, fmt.Errorf("effective SSH audit requires Linux")
}
