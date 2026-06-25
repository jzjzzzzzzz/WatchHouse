package fileaudit

import (
	"fmt"
	"sort"
)

type Control struct {
	ID               string
	Path             string
	UID              uint32
	AllowedWriteMask uint32
	AllowedMode      uint32
}

type Metadata struct {
	UID     uint32
	GID     uint32
	Mode    uint32
	Regular bool
	Symlink bool
}

type Inspector func(string) (Metadata, error)

var ServerFiles = []Control{
	{"FILE-001", "/etc/passwd", 0, 0o022, 0o644},
	{"FILE-002", "/etc/group", 0, 0o022, 0o644},
	{"FILE-003", "/etc/shadow", 0, 0o077, 0o640},
	{"FILE-004", "/etc/gshadow", 0, 0o077, 0o640},
	{"FILE-005", "/etc/ssh/sshd_config", 0, 0o077, 0o600},
	{"FILE-006", "/etc/sudoers", 0, 0o077, 0o440},
}

func Evaluate(controls []Control, inspect Inspector) ([]Result, error) {
	seen := map[string]bool{}
	results := make([]Result, 0, len(controls))
	for _, control := range controls {
		if control.ID == "" || seen[control.ID] || control.Path == "" || control.Path[0] != '/' || control.AllowedMode > 0o7777 {
			return nil, fmt.Errorf("invalid or duplicate file control %q", control.ID)
		}
		seen[control.ID] = true
		result := Result{ControlID: control.ID, Path: control.Path}
		meta, err := inspect(control.Path)
		if err != nil {
			result.Status, result.Detail = Error, err.Error()
			results = append(results, result)
			continue
		}
		result.OwnerUID, result.OwnerGID, result.Mode = meta.UID, meta.GID, fmt.Sprintf("%04o", meta.Mode)
		switch {
		case meta.Symlink:
			result.Status, result.Detail = Fail, "final path is a symlink"
		case !meta.Regular:
			result.Status, result.Detail = Fail, "not a regular file"
		case meta.UID != control.UID:
			result.Status, result.Detail = Fail, fmt.Sprintf("owner UID must be %d", control.UID)
		case meta.Mode&control.AllowedWriteMask != 0:
			result.Status, result.Detail = Fail, fmt.Sprintf("forbidden permission mask %04o is set", control.AllowedWriteMask)
		case meta.Mode&0o777 & ^control.AllowedMode != 0:
			result.Status, result.Detail = Fail, fmt.Sprintf("mode grants permissions outside %04o", control.AllowedMode)
		default:
			result.Status = Pass
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].ControlID < results[j].ControlID })
	return results, nil
}
