// Package extprobe performs bounded HTTPS observations from the caller's
// network vantage point. It never follows redirects or trusts proxy settings.
package extprobe

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	URL            string
	ExpectedStatus int
	MaxBodyBytes   int64
	AllowPrivate   bool
	Roots          *x509.CertPool
	Timeout        time.Duration
}

type Result struct {
	Type                  string        `json:"type"`
	URL                   string        `json:"url"`
	ObservedAt            time.Time     `json:"observed_at"`
	ResolvedAddresses     []string      `json:"resolved_addresses"`
	ConnectedAddress      string        `json:"connected_address"`
	DNSDuration           time.Duration `json:"dns_duration"`
	TCPDuration           time.Duration `json:"tcp_duration"`
	TLSDuration           time.Duration `json:"tls_duration"`
	TotalDuration         time.Duration `json:"total_duration"`
	TLSVersion            string        `json:"tls_version"`
	CipherSuite           string        `json:"cipher_suite"`
	PeerCertificateSHA256 string        `json:"peer_certificate_sha256"`
	HTTPStatus            int           `json:"http_status"`
	ExpectedStatus        int           `json:"expected_status"`
	ContentType           string        `json:"content_type"`
	BodyBytes             int64         `json:"body_bytes"`
	BodySHA256            string        `json:"body_sha256"`
	Expected              bool          `json:"expected"`
	PrivateTargetsAllowed bool          `json:"private_targets_allowed"`
}

func Run(ctx context.Context, config Config) (Result, error) {
	var result Result
	parsed, host, port, err := validateConfig(config)
	if err != nil {
		return result, err
	}
	started := time.Now()
	result = Result{Type: "external_https_probe", URL: parsed.String(), ObservedAt: started.UTC(), ExpectedStatus: config.ExpectedStatus, PrivateTargetsAllowed: config.AllowPrivate}
	resolveStarted := time.Now()
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	result.DNSDuration = time.Since(resolveStarted)
	if err != nil {
		return result, fmt.Errorf("resolve target: %w", err)
	}
	unique := make(map[netip.Addr]struct{})
	for _, address := range addresses {
		address = address.Unmap()
		if !address.IsValid() || address.Zone() != "" || (!config.AllowPrivate && !publicAddress(address)) {
			continue
		}
		unique[address] = struct{}{}
	}
	addresses = addresses[:0]
	for address := range unique {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].Compare(addresses[j]) < 0 })
	for _, address := range addresses {
		result.ResolvedAddresses = append(result.ResolvedAddresses, address.String())
	}
	if len(addresses) == 0 {
		return result, fmt.Errorf("DNS returned no permitted target addresses")
	}
	selected := netip.AddrPortFrom(addresses[0], port)
	result.ConnectedAddress = selected.String()
	configuration := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host, RootCAs: config.Roots}
	dialer := &net.Dialer{Timeout: minDuration(5*time.Second, config.Timeout), KeepAlive: -1}
	transport := &http.Transport{Proxy: nil, DisableCompression: true, ForceAttemptHTTP2: true, TLSClientConfig: configuration,
		ResponseHeaderTimeout: minDuration(10*time.Second, config.Timeout), IdleConnTimeout: time.Second, MaxIdleConns: 1, MaxIdleConnsPerHost: 1}
	transport.DialTLSContext = func(dialCtx context.Context, _, _ string) (net.Conn, error) {
		tcpStarted := time.Now()
		connection, err := dialer.DialContext(dialCtx, "tcp", selected.String())
		result.TCPDuration = time.Since(tcpStarted)
		if err != nil {
			return nil, err
		}
		tlsConnection := tls.Client(connection, configuration.Clone())
		tlsStarted := time.Now()
		if err := tlsConnection.HandshakeContext(dialCtx); err != nil {
			connection.Close()
			return nil, err
		}
		result.TLSDuration = time.Since(tlsStarted)
		return tlsConnection, nil
	}
	client := &http.Client{Transport: transport, Timeout: config.Timeout, CheckRedirect: func(*http.Request, []*http.Request) error {
		return fmt.Errorf("redirects are forbidden")
	}}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return result, err
	}
	request.Header.Set("Accept", "*/*")
	request.Header.Set("User-Agent", "watchhouse-probe/1")
	response, err := client.Do(request)
	if err != nil {
		return result, fmt.Errorf("HTTPS probe: %w", err)
	}
	defer response.Body.Close()
	result.HTTPStatus = response.StatusCode
	result.ContentType = response.Header.Get("Content-Type")
	if response.TLS == nil || len(response.TLS.PeerCertificates) == 0 {
		return result, fmt.Errorf("verified TLS state missing")
	}
	result.TLSVersion = tls.VersionName(response.TLS.Version)
	result.CipherSuite = tls.CipherSuiteName(response.TLS.CipherSuite)
	certificateDigest := sha256.Sum256(response.TLS.PeerCertificates[0].Raw)
	result.PeerCertificateSHA256 = hex.EncodeToString(certificateDigest[:])
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(response.Body, config.MaxBodyBytes+1))
	if err != nil {
		return result, fmt.Errorf("read response body: %w", err)
	}
	if count > config.MaxBodyBytes {
		return result, fmt.Errorf("response body exceeds %d-byte bound", config.MaxBodyBytes)
	}
	result.BodyBytes = count
	result.BodySHA256 = hex.EncodeToString(hash.Sum(nil))
	result.TotalDuration = time.Since(started)
	result.Expected = response.StatusCode == config.ExpectedStatus
	return result, nil
}

func (result Result) Validate() error {
	parsed, host, port, err := validateConfig(Config{URL: result.URL, ExpectedStatus: result.ExpectedStatus, MaxBodyBytes: result.BodyBytes,
		AllowPrivate: result.PrivateTargetsAllowed, Timeout: maxDuration(result.TotalDuration, time.Second)})
	if err != nil || parsed.String() != result.URL || host == "" || port == 0 || result.Type != "external_https_probe" ||
		result.ObservedAt.IsZero() || result.ObservedAt.Year() < 1970 || result.ObservedAt.Year() > 9999 ||
		len(result.ResolvedAddresses) < 1 || len(result.ResolvedAddresses) > 64 || result.ConnectedAddress == "" ||
		result.DNSDuration < 0 || result.TCPDuration < 0 || result.TLSDuration < 0 || result.TotalDuration < 0 || result.TotalDuration > time.Minute ||
		result.TLSVersion == "" || len(result.TLSVersion) > 32 || result.CipherSuite == "" || len(result.CipherSuite) > 128 ||
		!validHexDigest(result.PeerCertificateSHA256) || result.HTTPStatus < 100 || result.HTTPStatus > 599 ||
		len(result.ContentType) > 256 || result.BodyBytes < 0 || result.BodyBytes > 1024*1024 || !validHexDigest(result.BodySHA256) ||
		result.Expected != (result.HTTPStatus == result.ExpectedStatus) {
		return fmt.Errorf("invalid external probe result")
	}
	seen, connected := make(map[netip.Addr]struct{}), false
	previous := netip.Addr{}
	for index, value := range result.ResolvedAddresses {
		address, parseErr := netip.ParseAddr(value)
		if parseErr != nil || address.Zone() != "" || address.String() != value || (!result.PrivateTargetsAllowed && !publicAddress(address)) ||
			(index > 0 && previous.Compare(address) >= 0) {
			return fmt.Errorf("invalid external probe address set")
		}
		seen[address] = struct{}{}
		previous = address
	}
	connectedAddress, parseErr := netip.ParseAddrPort(result.ConnectedAddress)
	if parseErr == nil {
		_, connected = seen[connectedAddress.Addr().Unmap()]
	}
	if !connected || connectedAddress.Port() != port {
		return fmt.Errorf("connected address is outside resolved target set")
	}
	return nil
}

func validHexDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func validateConfig(config Config) (*url.URL, string, uint16, error) {
	if config.ExpectedStatus < 100 || config.ExpectedStatus > 599 || config.MaxBodyBytes < 0 || config.MaxBodyBytes > 1024*1024 || config.Timeout < time.Second || config.Timeout > time.Minute {
		return nil, "", 0, fmt.Errorf("invalid probe bounds")
	}
	parsed, err := url.Parse(config.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || parsed.Opaque != "" {
		return nil, "", 0, fmt.Errorf("probe target must be an HTTPS URL without user info, query, or fragment")
	}
	host := parsed.Hostname()
	if host == "" || strings.HasSuffix(host, ".") {
		return nil, "", 0, fmt.Errorf("probe target hostname is invalid")
	}
	port := uint64(443)
	if value := parsed.Port(); value != "" {
		port, err = strconv.ParseUint(value, 10, 16)
		if err != nil || port == 0 {
			return nil, "", 0, fmt.Errorf("probe target port is invalid")
		}
	}
	return parsed, host, uint16(port), nil
}

func publicAddress(address netip.Addr) bool {
	return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() && !address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast()
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
