//go:build !linux

package fileaudit

import (
	"fmt"
	"time"
)

func Collect(func() time.Time) (Report, error) {
	return Report{}, fmt.Errorf("file audit requires Linux")
}
