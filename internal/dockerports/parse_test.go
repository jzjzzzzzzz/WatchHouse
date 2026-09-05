package dockerports

import (
	"strings"
	"testing"
	"time"
)

const containerID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestParsePublishedBindings(t *testing.T) {
	raw := []byte(`{"id":"` + containerID + `","name":"/edge","network_mode":"bridge","ports":{"443/tcp":[{"host_ip":"127.0.0.1","host_port":"8443"},{"host_ip":"::1","host_port":"8443"}],"80/tcp":null}}` + "\n")
	got, err := Parse(raw, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got.ContainerCount != 1 || got.PublishedBindingCount != 2 {
		t.Fatalf("%#v", got)
	}
	if got.Bindings[0].ContainerName != "edge" || got.Bindings[0].ContainerPort != 443 || got.Bindings[0].HostPort != 8443 || got.Bindings[0].Protocol != "tcp" {
		t.Fatalf("%#v", got.Bindings)
	}
}

func TestParseAllowsNoRunningContainers(t *testing.T) {
	got, err := Parse(nil, time.Now())
	if err != nil || got.ContainerCount != 0 || got.Bindings == nil {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestParseRejectsHostileRows(t *testing.T) {
	valid := `{"id":"` + containerID + `","name":"/edge","network_mode":"bridge","ports":{}}`
	for _, raw := range []string{
		`{"id":"bad","name":"/edge","network_mode":"bridge","ports":{}}`,
		`{"id":"` + containerID + `","name":"/edge","network_mode":"bridge","ports":{},"extra":1}`,
		`{"id":"` + containerID + `","id":"` + containerID + `","name":"/edge","network_mode":"bridge","ports":{}}`,
		`{"id":"` + containerID + `","name":"/edge","network_mode":"bridge","ports":{"80/sctp":[]}}`,
		`{"id":"` + containerID + `","name":"/edge","network_mode":"bridge","ports":{"80/tcp":[{"host_ip":"not-ip","host_port":"8080"}]}}`,
		valid + "\n" + valid + "\n",
	} {
		if _, err := Parse([]byte(raw), time.Now()); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if _, err := Parse([]byte(strings.Repeat("x", MaxInspectBytes+1)), time.Now()); err == nil {
		t.Fatal("accepted oversize")
	}
}

func FuzzParseNeverPanics(f *testing.F) {
	f.Add([]byte(`{"id":"` + containerID + `","name":"edge","network_mode":"bridge","ports":{}}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxInspectBytes+1 {
			t.Skip()
		}
		_, _ = Parse(raw, time.Unix(0, 0))
	})
}
