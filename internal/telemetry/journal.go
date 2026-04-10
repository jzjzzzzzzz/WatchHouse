package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var authPattern = regexp.MustCompile(`^(Failed|Accepted) (password|publickey) for (invalid user )?(\S{1,256}) from (\S+) port ([0-9]{1,5})(?: ssh2)?(?:: [^\r\n]*)?(?: \[preauth\])?$`)

// ParseJournal returns matched=false for irrelevant or unsupported records.
// Malformed input returns an error. No raw MESSAGE is included in the event.
func ParseJournal(line []byte, host string, received time.Time) (Event, bool, error) {
	var zero Event
	if !ValidHost(host) || received.IsZero() {
		return zero, false, fmt.Errorf("invalid host or receive time")
	}
	if len(line) > MaxRecordBytes {
		return zero, false, fmt.Errorf("journal record exceeds limit")
	}
	if !utf8.Valid(line) {
		return zero, false, fmt.Errorf("journal record is not valid UTF-8")
	}
	fields, err := journalFields(line)
	if err != nil {
		return zero, false, err
	}
	// Kernel-provided comm is not enough by itself: a local user can rename a
	// process to sshd. Require a root-owned emitting process as well.
	comm, err := stringField(fields, "_COMM", false)
	if err != nil {
		return zero, false, err
	}
	if comm != "sshd" && comm != "sshd-session" {
		return zero, false, nil
	}
	uid, err := stringField(fields, "_UID", true)
	if err != nil {
		return zero, false, err
	}
	if uid != "0" {
		return zero, false, nil
	}
	message, err := stringField(fields, "MESSAGE", true)
	if err != nil {
		return zero, false, err
	}
	if !clean(message, MaxRecordBytes) {
		return zero, false, fmt.Errorf("invalid authentication message")
	}
	m := authPattern.FindStringSubmatch(message)
	if m == nil {
		return zero, false, nil
	}
	boot, err := stringField(fields, "_BOOT_ID", true)
	if err != nil {
		return zero, false, err
	}
	cursor, err := stringField(fields, "__CURSOR", true)
	if err != nil {
		return zero, false, err
	}
	ts, err := stringField(fields, "__REALTIME_TIMESTAMP", true)
	if err != nil {
		return zero, false, err
	}
	micros, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || micros < 0 || micros > 253402300799999999 {
		return zero, false, fmt.Errorf("invalid journal microsecond timestamp")
	}
	ip, err := netip.ParseAddr(m[5])
	if err != nil || ip.Zone() != "" {
		return zero, false, fmt.Errorf("invalid source IP")
	}
	port, err := strconv.ParseUint(m[6], 10, 16)
	if err != nil || port == 0 {
		return zero, false, fmt.Errorf("invalid source port")
	}
	e := Event{
		SchemaVersion: 1, HostID: host, BootID: boot, Source: "journald", SourceCursor: cursor,
		EventID: Identity(host, boot, cursor), Kind: "ssh.authentication",
		ObservedAt: time.UnixMicro(micros).UTC(), ReceivedAt: received.UTC(),
		Authentication: Authentication{Outcome: strings.ToLower(m[1]), Method: m[2], User: m[4],
			InvalidUser: m[3] != "", SourceIP: ip.Unmap().String(), SourcePort: uint16(port)},
	}
	// OpenSSH's success word is Accepted, not a generic inferred status.
	if err := e.Validate(); err != nil {
		return zero, false, err
	}
	return e, true, nil
}

func stringField(fields map[string]json.RawMessage, key string, required bool) (string, error) {
	raw, ok := fields[key]
	if !ok {
		if required {
			return "", fmt.Errorf("journal field %s missing", key)
		}
		return "", nil
	}
	var value string
	if len(raw) == 0 || raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
		return "", fmt.Errorf("journal field %s must be a string", key)
	}
	if required && value == "" {
		return "", fmt.Errorf("journal field %s empty", key)
	}
	return value, nil
}

// Reject duplicate top-level keys instead of accepting JSON's last-key-wins.
// Journal may contain repeated-value arrays; required fields must be scalar.
func journalFields(line []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(line))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("journal record must be a JSON object")
	}
	fields := make(map[string]json.RawMessage)
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("invalid journal key")
		}
		name, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("invalid journal key type")
		}
		if _, exists := fields[name]; exists {
			return nil, fmt.Errorf("duplicate journal field")
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("invalid journal value")
		}
		fields[name] = value
	}
	if token, err = dec.Token(); err != nil || token != json.Delim('}') {
		return nil, fmt.Errorf("unterminated journal object")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing journal data")
	}
	return fields, nil
}
