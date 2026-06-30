package sshaudit

import "time"

type Status string

const (
	Pass  Status = "pass"
	Fail  Status = "fail"
	Error Status = "error"
)

type Result struct {
	ControlID string `json:"control_id"`
	Directive string `json:"directive"`
	Status    Status `json:"status"`
	Observed  string `json:"observed,omitempty"`
	Expected  string `json:"expected"`
	Detail    string `json:"detail,omitempty"`
}

type Report struct {
	SchemaVersion int       `json:"schema_version"`
	ObservedAt    time.Time `json:"observed_at"`
	Context       string    `json:"context"`
	Results       []Result  `json:"results"`
	Passed        int       `json:"passed"`
	Failed        int       `json:"failed"`
	Errors        int       `json:"errors"`
}

func NewReport(at time.Time, context string, results []Result) Report {
	r := Report{SchemaVersion: 1, ObservedAt: at.UTC(), Context: context, Results: results}
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
