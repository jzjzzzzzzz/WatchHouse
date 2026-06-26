package fileaudit

import (
	"errors"
	"testing"
)

func TestEvaluateFileControls(t *testing.T) {
	controls := []Control{
		{ID: "C", Path: "/missing", UID: 0, AllowedWriteMask: 0o022, AllowedMode: 0o644},
		{ID: "A", Path: "/good", UID: 0, AllowedWriteMask: 0o022, AllowedMode: 0o640},
		{ID: "B", Path: "/bad", UID: 0, AllowedWriteMask: 0o022, AllowedMode: 0o640},
	}
	got, err := Evaluate(controls, func(path string) (Metadata, error) {
		switch path {
		case "/good":
			return Metadata{UID: 0, GID: 42, Mode: 0o600, Regular: true}, nil
		case "/bad":
			return Metadata{UID: 1000, GID: 1000, Mode: 0o666, Regular: true}, nil
		default:
			return Metadata{}, errors.New("not found")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ControlID != "A" || got[0].Status != Pass {
		t.Fatalf("%#v", got)
	}
	if got[1].Status != Fail || got[1].Detail != "owner UID must be 0" {
		t.Fatalf("%#v", got[1])
	}
	if got[2].Status != Error {
		t.Fatalf("%#v", got[2])
	}
}

func TestEvaluateRejectsSymlinkBeforePermissions(t *testing.T) {
	got, err := Evaluate([]Control{{ID: "A", Path: "/link", UID: 0, AllowedMode: 0o777}}, func(string) (Metadata, error) {
		return Metadata{UID: 0, Mode: 0o777, Regular: true, Symlink: true}, nil
	})
	if err != nil || got[0].Status != Fail || got[0].Detail != "final path is a symlink" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestServerFileCatalogIsValid(t *testing.T) {
	got, err := Evaluate(ServerFiles, func(string) (Metadata, error) { return Metadata{UID: 0, GID: 0, Mode: 0, Regular: true}, nil })
	if err != nil || len(got) != len(ServerFiles) {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	for _, result := range got {
		if result.Status != Pass {
			t.Fatalf("%#v", result)
		}
	}
}

func TestEvaluateRejectsInvalidCatalog(t *testing.T) {
	for _, controls := range [][]Control{
		{{ID: "", Path: "/x"}},
		{{ID: "A", Path: "relative"}},
		{{ID: "A", Path: "/x"}, {ID: "A", Path: "/y"}},
		{{ID: "A", Path: "/x", AllowedMode: 0o10000}},
	} {
		if _, err := Evaluate(controls, func(string) (Metadata, error) { return Metadata{Regular: true}, nil }); err == nil {
			t.Fatalf("accepted %#v", controls)
		}
	}
}
