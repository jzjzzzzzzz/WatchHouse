package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"

	"watchhouse/internal/hostview"
	"watchhouse/internal/strictjson"
)

func (client *Client) PublishListenerSnapshot(ctx context.Context, host string, snapshot hostview.HostSnapshot) (ListenerSnapshotReceipt, error) {
	requestBody := ListenerSnapshotRequest{SchemaVersion: SchemaVersion, Snapshot: snapshot, SnapshotID: ListenerSnapshotID(host, snapshot)}
	var receipt ListenerSnapshotReceipt
	if err := requestBody.Validate(host); err != nil {
		return receipt, err
	}
	body, err := json.Marshal(requestBody)
	if err != nil || len(body) > MaxBodyBytes {
		return receipt, fmt.Errorf("encode bounded listener snapshot")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.origin+"/v1/listener-snapshots", bytes.NewReader(body))
	if err != nil {
		return receipt, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return receipt, fmt.Errorf("publish listener snapshot: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return receipt, fmt.Errorf("listener receiver returned HTTP %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return receipt, fmt.Errorf("listener receiver returned invalid content type")
	}
	receipt, err = strictjson.Decode[ListenerSnapshotReceipt](response.Body, maxResponseBytes)
	if err != nil || receipt.SchemaVersion != SchemaVersion || receipt.SnapshotID != requestBody.SnapshotID {
		return ListenerSnapshotReceipt{}, fmt.Errorf("listener receiver returned invalid receipt")
	}
	return receipt, nil
}
