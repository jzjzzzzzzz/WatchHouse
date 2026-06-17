package posture

import "time"

type Status string

const (
	Pass  Status = "pass"
	Fail  Status = "fail"
	Error Status = "error"
)

type Result struct {
	ControlID string `json:"control_id"`
	Status    Status `json:"status"`
	Observed  string `json:"observed,omitempty"`
	Expected  string `json:"expected"`
	Source    string `json:"source"`
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
	report := Report{SchemaVersion: 1, ObservedAt: at.UTC(), Results: results}
	for _, result := range results {
		switch result.Status {
		case Pass:
			report.Passed++
		case Fail:
			report.Failed++
		case Error:
			report.Errors++
		}
	}
	return report
}
