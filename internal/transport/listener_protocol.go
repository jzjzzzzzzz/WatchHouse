package transport

import (
	"fmt"
	"time"

	"watchhouse/internal/hostview"
	"watchhouse/internal/telemetry"
)

type ListenerSnapshotRequest struct {
	SchemaVersion int                   `json:"schema_version"`
	SnapshotID    string                `json:"snapshot_id"`
	Snapshot      hostview.HostSnapshot `json:"snapshot"`
}

type ListenerSnapshotReceipt struct {
	SchemaVersion int    `json:"schema_version"`
	SnapshotID    string `json:"snapshot_id"`
}

func ListenerSnapshotID(host string, snapshot hostview.HostSnapshot) string {
	return telemetry.Identity(host, snapshot.BootID, snapshot.NetworkNamespace, snapshot.ObservedAt.UTC().Format(time.RFC3339Nano))
}

func (request ListenerSnapshotRequest) Validate(host string) error {
	if request.SchemaVersion != SchemaVersion || !telemetry.ValidHost(host) || request.SnapshotID != ListenerSnapshotID(host, request.Snapshot) {
		return fmt.Errorf("invalid listener snapshot envelope")
	}
	if err := request.Snapshot.Validate(); err != nil {
		return err
	}
	return nil
}
