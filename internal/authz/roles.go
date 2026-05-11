// Package authz maps certificate-bound human principals to explicit roles.
package authz

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"watchhouse/internal/strictjson"
	"watchhouse/internal/telemetry"
)

const maxRoleMapBytes = 64 * 1024

type Role string

const (
	Viewer   Role = "viewer"
	Operator Role = "operator"
	Admin    Role = "admin"
)

type Map struct {
	Principals map[string]Role `json:"principals"`
}

func Load(path string) (*Map, error) {
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("role map path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("role map must be a non-writable regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Decode(file)
}

type document struct {
	SchemaVersion int             `json:"schema_version"`
	Principals    map[string]Role `json:"principals"`
}

func Decode(input io.Reader) (*Map, error) {
	value, err := strictjson.Decode[document](input, maxRoleMapBytes)
	if err != nil {
		return nil, fmt.Errorf("decode role map: %w", err)
	}
	if value.SchemaVersion != 1 || len(value.Principals) == 0 || len(value.Principals) > 256 {
		return nil, fmt.Errorf("invalid role map schema or principal count")
	}
	for principal, role := range value.Principals {
		if !telemetry.ValidHost(principal) || (role != Viewer && role != Operator && role != Admin) {
			return nil, fmt.Errorf("invalid role-map principal or role")
		}
	}
	return &Map{Principals: value.Principals}, nil
}

func (roles *Map) CanQuery(principal string) (Role, bool) {
	if roles == nil {
		return "", false
	}
	role, found := roles.Principals[principal]
	return role, found && (role == Viewer || role == Operator || role == Admin)
}
