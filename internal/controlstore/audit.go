package controlstore

import (
	"context"
	"fmt"

	"watchhouse/internal/authz"
	"watchhouse/internal/telemetry"
)

func (store *Store) RecordQueryDecision(ctx context.Context, principal string, role authz.Role, resource, host, decision string) error {
	if !telemetry.ValidHost(principal) || !telemetry.ValidHost(host) ||
		(resource != "events" && resource != "findings") || (decision != "allowed" && decision != "denied") ||
		(decision == "allowed" && role != authz.Viewer && role != authz.Operator && role != authz.Admin) ||
		(decision == "denied" && role != "" && role != authz.Viewer && role != authz.Operator && role != authz.Admin) {
		return fmt.Errorf("invalid query audit decision")
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO control_query_audit(principal_id,role,resource,host_id,decision) VALUES($1,NULLIF($2,''),$3,$4,$5)`,
		principal, role, resource, host, decision); err != nil {
		return fmt.Errorf("record query authorization decision: %w", err)
	}
	return nil
}
