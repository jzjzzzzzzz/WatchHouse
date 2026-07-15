//go:build linux

package unitaudit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"watchhouse/internal/boundedio"
)

const propertyArgument = "LoadState,ActiveState,NoNewPrivileges,ProtectSystem,ProtectHome,PrivateTmp,CapabilityBoundingSet"

var systemctlPaths = []string{"/usr/bin/systemctl", "/bin/systemctl"}

func Collect(ctx context.Context, now func() time.Time) (Report, error) {
	path := ""
	for _, candidate := range systemctlPaths {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o022 == 0 {
			path = candidate
			break
		}
	}
	if path == "" {
		return Report{}, fmt.Errorf("trusted systemctl executable not found")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	reports := make([]UnitReport, 0, len(Units))
	for _, unit := range Units {
		stdout, stderr := boundedio.NewWriter(MaxOutputBytes), boundedio.NewWriter(16<<10)
		cmd := exec.CommandContext(ctx, path, "show", "--no-pager", "--property="+propertyArgument, unit)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(nil), stdout, stderr
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return Report{}, fmt.Errorf("systemctl timeout: %w", ctx.Err())
			}
			if errors.Is(err, boundedio.ErrLimit) {
				return Report{}, boundedio.ErrLimit
			}
			return Report{}, fmt.Errorf("systemctl show %s: %w: %s", unit, err, bytes.TrimSpace(stderr.Bytes()))
		}
		report, err := Parse(unit, stdout.Bytes())
		if err != nil {
			return Report{}, err
		}
		reports = append(reports, report)
	}
	return NewReport(now(), reports), nil
}
