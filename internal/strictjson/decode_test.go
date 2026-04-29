package strictjson

import (
	"strings"
	"testing"
)

type fixture struct {
	Name   string `json:"name"`
	Nested struct {
		Value int `json:"value"`
	} `json:"nested"`
}

func TestDecodeStrictObject(t *testing.T) {
	got, err := Decode[fixture](strings.NewReader(`{"name":"ok","nested":{"value":2}}`), 1024)
	if err != nil || got.Name != "ok" || got.Nested.Value != 2 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestRejectDuplicateUnknownTrailingAndOversize(t *testing.T) {
	for _, body := range []string{
		`{"name":"one","name":"two","nested":{"value":1}}`,
		`{"name":"one","nested":{"value":1,"value":2}}`,
		`{"name":"one","nested":{"unknown":1}}`,
		`{"name":"one","nested":{"value":1}} {}`,
		``,
	} {
		if _, err := Decode[fixture](strings.NewReader(body), 1024); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
	if _, err := Decode[fixture](strings.NewReader(`{"name":"long"}`), 4); err == nil {
		t.Fatal("size bound not enforced")
	}
	if _, err := Decode[fixture](strings.NewReader(`{}`), 0); err == nil {
		t.Fatal("invalid bound accepted")
	}
}
