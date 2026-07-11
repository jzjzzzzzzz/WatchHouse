package unitaudit

import "time"

type Status string

const (
	Pass  Status = "pass"
	Fail  Status = "fail"
	Error Status = "error"
)

type Result struct {
	Unit     string `json:"unit"`
	Property string `json:"property"`
	Status   Status `json:"status"`
	Observed string `json:"observed,omitempty"`
	Expected string `json:"expected"`
	Detail   string `json:"detail,omitempty"`
}

type UnitReport struct {
	Name        string   `json:"name"`
	LoadState   string   `json:"load_state"`
	ActiveState string   `json:"active_state"`
	Results     []Result `json:"results"`
}

type Report struct {
	SchemaVersion int          `json:"schema_version"`
	ObservedAt    time.Time    `json:"observed_at"`
	Units         []UnitReport `json:"units"`
	Passed        int          `json:"passed"`
	Failed        int          `json:"failed"`
	Errors        int          `json:"errors"`
}

func NewReport(at time.Time, units []UnitReport) Report {
	r := Report{SchemaVersion: 1, ObservedAt: at.UTC(), Units: units}
	for _, unit := range units {
		for _, item := range unit.Results {
			switch item.Status {
			case Pass:
				r.Passed++
			case Fail:
				r.Failed++
			case Error:
				r.Errors++
			}
		}
	}
	return r
}
