package pkginventory

import "time"

const (
	MaxPackages    = 100_000
	MaxOutputBytes = 16 << 20
)

type Package struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Architecture string `json:"architecture"`
}

type Report struct {
	SchemaVersion int       `json:"schema_version"`
	ObservedAt    time.Time `json:"observed_at"`
	Manager       string    `json:"manager"`
	PackageCount  int       `json:"package_count"`
	SHA256        string    `json:"sha256"`
	Packages      []Package `json:"packages"`
}
