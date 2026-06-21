//go:build !linux

package posture

import (
	"fmt"
	"time"
)

func CollectSysctls(func() time.Time) (Report, error) {
	return Report{}, fmt.Errorf("sysctl posture collection requires Linux")
}
