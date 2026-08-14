package evidencebundle

import "time"

const (
	MaxFiles      = 128
	MaxFileBytes  = 4 << 20
	MaxTotalBytes = 32 << 20
)

type Entry struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	Entries       []Entry   `json:"entries"`
	TotalBytes    int64     `json:"total_bytes"`
}
