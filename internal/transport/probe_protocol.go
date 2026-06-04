package transport

import (
	"context"
	"encoding/json"
	"fmt"

	"watchhouse/internal/extprobe"
	"watchhouse/internal/telemetry"
)

type ProbeObservationRequest struct {
	SchemaVersion int             `json:"schema_version"`
	ObservationID string          `json:"observation_id"`
	Result        extprobe.Result `json:"result"`
}

type ProbeObservationReceipt struct {
	SchemaVersion int    `json:"schema_version"`
	ObservationID string `json:"observation_id"`
}

type ProbeObservationStore interface {
	CommitProbeObservation(context.Context, string, ProbeObservationRequest) error
}

func ProbeObservationID(probe string, result extprobe.Result) (string, error) {
	if !telemetry.ValidHost(probe) || result.Validate() != nil {
		return "", fmt.Errorf("invalid probe observation identity input")
	}
	body, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return telemetry.Identity(probe, string(body)), nil
}

func (request ProbeObservationRequest) Validate(probe string) error {
	if request.SchemaVersion != SchemaVersion || request.Result.Validate() != nil {
		return fmt.Errorf("invalid probe observation envelope")
	}
	expected, err := ProbeObservationID(probe, request.Result)
	if err != nil || request.ObservationID != expected {
		return fmt.Errorf("probe observation identity mismatch")
	}
	return nil
}
