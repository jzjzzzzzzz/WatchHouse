package fileaudit

import "time"

type Status string

const (
	Pass  Status = "pass"
	Fail  Status = "fail"
	Error Status = "error"
)

type Result struct {
	ControlID string `json:"control_id"`
	Path      string `json:"path"`
	Status    Status `json:"status"`
	OwnerUID  uint32 `json:"owner_uid,omitempty"`
	OwnerGID  uint32 `json:"owner_gid,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Detail    string `json:"detail,omitempty"`
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
