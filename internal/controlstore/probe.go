package controlstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"watchhouse/internal/transport"
)

func (store *Store) CommitProbeObservation(ctx context.Context, probe string, request transport.ProbeObservationRequest) error {
	if err := request.Validate(probe); err != nil {
		return err
	}
	body, err := json.Marshal(request.Result)
	if err != nil {
		return fmt.Errorf("encode probe observation: %w", err)
	}
	digestBytes := sha256.Sum256(body)
	digest := hex.EncodeToString(digestBytes[:])
	tag, err := store.pool.Exec(ctx, `INSERT INTO control_probe_observations(observation_id,probe_id,observed_at,target_url,connected_address,http_status,expected,content_digest,payload)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(observation_id) DO NOTHING`, request.ObservationID, probe,
		request.Result.ObservedAt, request.Result.URL, request.Result.ConnectedAddress, request.Result.HTTPStatus, request.Result.Expected, digest, body)
	if err != nil {
		return fmt.Errorf("insert probe observation: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var storedProbe, storedDigest string
	if err := store.pool.QueryRow(ctx, `SELECT probe_id,content_digest FROM control_probe_observations WHERE observation_id=$1`, request.ObservationID).Scan(&storedProbe, &storedDigest); err != nil {
		return fmt.Errorf("read existing probe observation: %w", err)
	}
	if storedProbe != probe || storedDigest != digest {
		return transport.ErrEventConflict
	}
	return nil
}
