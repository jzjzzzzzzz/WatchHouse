package controlstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"watchhouse/internal/transport"
)

func (store *Store) CommitListenerSnapshot(ctx context.Context, host string, request transport.ListenerSnapshotRequest) error {
	if err := request.Validate(host); err != nil {
		return err
	}
	body, err := json.Marshal(request.Snapshot)
	if err != nil {
		return fmt.Errorf("encode listener snapshot: %w", err)
	}
	digestBytes := sha256.Sum256(body)
	digest := hex.EncodeToString(digestBytes[:])
	tag, err := store.pool.Exec(ctx, `INSERT INTO control_listener_snapshots(snapshot_id,host_id,boot_id,network_namespace,observed_at,listener_count,content_digest,payload)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(snapshot_id) DO NOTHING`, request.SnapshotID, host, request.Snapshot.BootID,
		request.Snapshot.NetworkNamespace, request.Snapshot.ObservedAt, len(request.Snapshot.Listeners), digest, body)
	if err != nil {
		return fmt.Errorf("insert listener snapshot: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var storedHost, storedDigest string
	if err := store.pool.QueryRow(ctx, `SELECT host_id,content_digest FROM control_listener_snapshots WHERE snapshot_id=$1`, request.SnapshotID).Scan(&storedHost, &storedDigest); err != nil {
		return fmt.Errorf("read existing listener snapshot: %w", err)
	}
	if storedHost != host || storedDigest != digest {
		return transport.ErrEventConflict
	}
	return nil
}
