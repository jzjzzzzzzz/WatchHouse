package transport

import (
	"testing"
	"time"

	"watchhouse/internal/extprobe"
)

func validProbeResult() extprobe.Result {
	return extprobe.Result{Type: "external_https_probe", URL: "https://203.0.113.10/health", ObservedAt: time.Unix(1770000000, 0).UTC(),
		ResolvedAddresses: []string{"203.0.113.10"}, ConnectedAddress: "203.0.113.10:443", DNSDuration: time.Millisecond,
		TCPDuration: time.Millisecond, TLSDuration: time.Millisecond, TotalDuration: 4 * time.Millisecond,
		TLSVersion: "TLS 1.3", CipherSuite: "TLS_AES_128_GCM_SHA256",
		PeerCertificateSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		HTTPStatus:            200, ExpectedStatus: 200, ContentType: "text/plain", BodyBytes: 2,
		BodySHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Expected: true}
}

func TestProbeObservationIdentityBindsPrincipalAndResult(t *testing.T) {
	result := validProbeResult()
	id, err := ProbeObservationID("outside-1", result)
	if err != nil {
		t.Fatal(err)
	}
	request := ProbeObservationRequest{SchemaVersion: 1, ObservationID: id, Result: result}
	if err := request.Validate("outside-1"); err != nil {
		t.Fatal(err)
	}
	if err := request.Validate("outside-2"); err == nil {
		t.Fatal("observation replayed under another probe")
	}
	request.Result.HTTPStatus = 503
	request.Result.Expected = false
	if err := request.Validate("outside-1"); err == nil {
		t.Fatal("changed result accepted under old identity")
	}
}
