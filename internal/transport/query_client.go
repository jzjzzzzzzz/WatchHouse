package transport

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"watchhouse/internal/strictjson"
	"watchhouse/internal/telemetry"
)

const MaxQueryResponseBytes = 16 * 1024 * 1024

func (client *Client) QueryEvents(ctx context.Context, host string, limit int, before int64) (EventPage, error) {
	var page EventPage
	if !telemetry.ValidHost(host) || limit < 1 || limit > 200 || before < 0 {
		return page, fmt.Errorf("invalid event query")
	}
	values := url.Values{"host": []string{host}, "limit": []string{strconv.Itoa(limit)}}
	if before > 0 {
		values.Set("before", strconv.FormatInt(before, 10))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.origin+"/v1/events?"+values.Encode(), nil)
	if err != nil {
		return page, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return page, fmt.Errorf("query events: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return page, fmt.Errorf("event query returned HTTP %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return page, fmt.Errorf("event query returned invalid content type")
	}
	page, err = strictjson.Decode[EventPage](response.Body, MaxQueryResponseBytes)
	if err != nil {
		return EventPage{}, fmt.Errorf("decode event query: %w", err)
	}
	if page.SchemaVersion != SchemaVersion || len(page.Records) > limit {
		return EventPage{}, fmt.Errorf("invalid event query envelope")
	}
	previous := int64(^uint64(0) >> 1)
	if before > 0 {
		previous = before
	}
	for index, record := range page.Records {
		if record.IngestSequence < 1 || record.IngestSequence >= previous || record.HostID != host || record.EventID != record.Event.EventID || record.Event.HostID != host || record.IngestedAt.IsZero() || record.Event.Validate() != nil {
			return EventPage{}, fmt.Errorf("invalid event query record %d", index)
		}
		previous = record.IngestSequence
	}
	return page, nil
}
