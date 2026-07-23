//go:build !linux

package diskaudit

import (
	"fmt"
	"time"
)

func Collect(func() time.Time) (Report, error) {
	return Report{}, fmt.Errorf("filesystem capacity audit requires Linux")
}
