package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"

	"watchhouse/internal/extprobe"
	"watchhouse/internal/strictjson"
)

func (client *Client) PublishProbeObservation(ctx context.Context, probe string, result extprobe.Result) (ProbeObservationReceipt, error) {
	var receipt ProbeObservationReceipt
	id, err := ProbeObservationID(probe, result)
	if err != nil {
		return receipt, err
	}
	payload := ProbeObservationRequest{SchemaVersion: SchemaVersion, ObservationID: id, Result: result}
	body, err := json.Marshal(payload)
	if err != nil || len(body) > MaxBodyBytes {
		return receipt, fmt.Errorf("encode bounded probe observation")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.origin+"/v1/probe-observations", bytes.NewReader(body))
	if err != nil {
		return receipt, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return receipt, fmt.Errorf("publish probe observation: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return receipt, fmt.Errorf("probe receiver returned HTTP %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return receipt, fmt.Errorf("probe receiver returned invalid content type")
	}
	receipt, err = strictjson.Decode[ProbeObservationReceipt](response.Body, maxResponseBytes)
	if err != nil || receipt.SchemaVersion != SchemaVersion || receipt.ObservationID != id {
		return ProbeObservationReceipt{}, fmt.Errorf("probe receiver returned invalid receipt")
	}
	return receipt, nil
}
