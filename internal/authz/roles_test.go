package authz

import (
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
