package diskaudit

import "time"

type Status string

const (
	Pass  Status = "pass"
	Fail  Status = "fail"
	Error Status = "error"
)

type Result struct {
	Path             string  `json:"path"`
	Status           Status  `json:"status"`
	FilesystemType   string  `json:"filesystem_type,omitempty"`
	TotalBytes       uint64  `json:"total_bytes,omitempty"`
	AvailableBytes   uint64  `json:"available_bytes,omitempty"`
	AvailablePercent float64 `json:"available_percent,omitempty"`
	Inodes           uint64  `json:"inodes,omitempty"`
	AvailableInodes  uint64  `json:"available_inodes,omitempty"`
	Expected         string  `json:"expected"`
	Detail           string  `json:"detail,omitempty"`
}

type Report struct {
	SchemaVersion int       `json:"schema_version"`
	ObservedAt    time.Time `json:"observed_at"`
	Results       []Result  `json:"results"`
	Passed        int       `json:"passed"`
	Failed        int       `json:"failed"`
	Errors        int       `json:"errors"`
}

func NewReport(at time.Time, results []Result) Report {
	r := Report{SchemaVersion: 1, ObservedAt: at.UTC(), Results: results}
	for _, item := range results {
		switch item.Status {
		case Pass:
			r.Passed++
		case Fail:
			r.Failed++
		case Error:
			r.Errors++
		}
	}
	return r
}
