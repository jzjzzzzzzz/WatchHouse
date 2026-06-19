package posture

import (
	"testing"
	"testing/fstest"
)

func TestEvaluateSysctlsReportsPassFailAndReadError(t *testing.T) {
	controls := []SysctlControl{
		{ID: "Z", Path: "net/ipv4/tcp_syncookies", Expected: []string{"1"}, Reason: "syncookies"},
		{ID: "A", Path: "kernel/kptr_restrict", Expected: []string{"1", "2"}, Reason: "pointers"},
		{ID: "M", Path: "fs/protected_symlinks", Expected: []string{"1"}, Reason: "symlinks"},
	}
	root := fstest.MapFS{
		"net/ipv4/tcp_syncookies": {Data: []byte("1\n")},
		"kernel/kptr_restrict":    {Data: []byte("0\n")},
	}
	got, err := EvaluateSysctls(root, controls)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ControlID != "A" || got[0].Status != Fail || got[0].Observed != "0" {
		t.Fatalf("first: %#v", got)
	}
	if got[1].Status != Error || got[2].Status != Pass {
		t.Fatalf("results: %#v", got)
	}
}

func TestEvaluateSysctlsRejectsInvalidCatalog(t *testing.T) {
	tests := [][]SysctlControl{
		{{ID: "", Path: "net/x", Expected: []string{"0"}}},
		{{ID: "A", Path: "/etc/shadow", Expected: []string{"0"}}},
		{{ID: "A", Path: "net/../etc/shadow", Expected: []string{"0"}}},
		{{ID: "A", Path: "net/x"}},
		{{ID: "A", Path: "net/x", Expected: []string{"0"}}, {ID: "A", Path: "net/y", Expected: []string{"0"}}},
	}
	for _, controls := range tests {
		if _, err := EvaluateSysctls(fstest.MapFS{}, controls); err == nil {
			t.Fatalf("accepted %#v", controls)
		}
	}
}

func TestServerSysctlCatalogIsValid(t *testing.T) {
	root := fstest.MapFS{}
	for _, control := range ServerSysctls {
		root[control.Path] = &fstest.MapFile{Data: []byte(control.Expected[0])}
	}
	results, err := EvaluateSysctls(root, ServerSysctls)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Status != Pass {
			t.Fatalf("%#v", result)
		}
	}
}
