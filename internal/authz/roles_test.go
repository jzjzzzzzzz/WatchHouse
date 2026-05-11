package authz

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictRoleMapAndQueryPermission(t *testing.T) {
	roles, err := Decode(strings.NewReader(`{"schema_version":1,"principals":{"alice":"viewer","operator-1":"operator","root-admin":"admin"}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, principal := range []string{"alice", "operator-1", "root-admin"} {
		if _, allowed := roles.CanQuery(principal); !allowed {
			t.Fatalf("%s denied", principal)
		}
	}
	if _, allowed := roles.CanQuery("unknown"); allowed {
		t.Fatal("unknown principal allowed")
	}
}

func TestLoadRoleMapRejectsMutableOrSymlinkFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "roles.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"principals":{"alice":"viewer"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("group/world writable role map accepted")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "roles-link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("symlink role map accepted")
	}
	if _, err := Load("roles.json"); err == nil {
		t.Fatal("relative role map accepted")
	}
}

func TestInvalidRoleMapsFailClosed(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"schema_version":1,"principals":{}}`,
		`{"schema_version":1,"principals":{"alice":"viewer","alice":"admin"}}`,
		`{"schema_version":1,"principals":{"alice":"owner"}}`,
		`{"schema_version":1,"principals":{"../alice":"viewer"}}`,
		`{"schema_version":1,"principals":{"alice":"viewer"},"extra":true}`,
	} {
		if _, err := Decode(strings.NewReader(body)); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	if _, allowed := (*Map)(nil).CanQuery("alice"); allowed {
		t.Fatal("nil role map allowed query")
	}
}
