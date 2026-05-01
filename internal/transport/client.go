package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"time"

	"watchhouse/internal/spool"
	"watchhouse/internal/strictjson"
)

const maxResponseBytes = 256 * 1024

type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(endpoint string, tlsConfig *tls.Config) (*Client, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("event endpoint must be an HTTPS origin without path, user info, query, or fragment")
	}
	if tlsConfig == nil || tlsConfig.RootCAs == nil || len(tlsConfig.Certificates) != 1 || tlsConfig.ServerName == "" || tlsConfig.InsecureSkipVerify {
		return nil, fmt.Errorf("event transport requires one client certificate, private roots, and an explicit verified server name")
	}
	configuration := tlsConfig.Clone()
	configuration.MinVersion = tls.VersionTLS13
	configuration.InsecureSkipVerify = false
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, ForceAttemptHTTP2: true,
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 10 * time.Second,
		TLSClientConfig: configuration, MaxIdleConns: 2, MaxIdleConnsPerHost: 2,
		IdleConnTimeout: 30 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("event transport redirects are forbidden")
		}}
	parsed.Path = ""
	return &Client{endpoint: parsed.String() + "/v1/events/batch", http: client}, nil
}

func (client *Client) Close() {
	if transport, ok := client.http.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

func (client *Client) Deliver(ctx context.Context, queued []spool.Item) ([]spool.Receipt, error) {
	items := make([]Item, len(queued))
	for index, item := range queued {
		items[index] = Item{Sequence: item.Sequence, EventID: item.EventID, Event: item.Event}
	}
	if len(items) < 1 || len(items) > MaxItems {
		return nil, fmt.Errorf("invalid outbound event batch size")
	}
	body, err := json.Marshal(BatchRequest{SchemaVersion: SchemaVersion, Items: items})
	if err != nil || len(body) > MaxBodyBytes {
		return nil, fmt.Errorf("encode bounded event batch")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create event request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send event batch: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("event receiver returned HTTP %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, fmt.Errorf("event receiver returned invalid content type")
	}
	accepted, err := strictjson.Decode[BatchResponse](response.Body, maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("decode event receipts: %w", err)
	}
	if err := accepted.ValidateExact(items); err != nil {
		return nil, err
	}
	receipts := make([]spool.Receipt, len(accepted.Receipts))
	for index, receipt := range accepted.Receipts {
		receipts[index] = spool.Receipt{Sequence: receipt.Sequence, EventID: receipt.EventID}
	}
	return receipts, nil
}
