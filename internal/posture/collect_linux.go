//go:build linux

package posture

import (
	"os"
	"time"
)

func CollectSysctls(now func() time.Time) (Report, error) {
	results, err := EvaluateSysctls(os.DirFS("/proc/sys"), ServerSysctls)
	if err != nil {
		return Report{}, err
	}
	return NewReport(now(), results), nil
}
