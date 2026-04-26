package hostview

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureProc(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "sys/kernel/random/boot_id"), "12345678-1234-1234-1234-123456789abc\n")
	if err := os.MkdirAll(filepath.Join(root, "self/ns"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("net:[4026531840]", filepath.Join(root, "self/ns/net")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(root, "net/tcp"), header+"0: 0100007F:01BB 0:0 0A 0:0 00:0 0 1000 0 4242\n")
	writeFixture(t, filepath.Join(root, "net/tcp6"), header)
	pid := filepath.Join(root, "42")
	writeFixture(t, filepath.Join(pid, "stat"), syntheticStat(42, "nginx worker", "9876"))
	writeFixture(t, filepath.Join(pid, "status"), "Name:\tnginx\nUid:\t1000\t1001\t1002\t1003\n")
	writeFixture(t, filepath.Join(pid, "cgroup"), "0::/system.slice/nginx.service\n")
	if err := os.MkdirAll(filepath.Join(pid, "fd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("socket:[4242]", filepath.Join(pid, "fd/3")); err != nil {
		t.Fatal(err)
	}
	// A duplicate descriptor must not duplicate the process owner.
	if err := os.Symlink("socket:[4242]", filepath.Join(pid, "fd/4")); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestScanProcAttributesStableSocketOwner(t *testing.T) {
	root := fixtureProc(t)
	at := time.Date(2026, 10, 5, 20, 0, 0, 0, time.FixedZone("EDT", -4*60*60))
	got, err := scanProc(root, at)
	if err != nil {
		t.Fatal(err)
	}
	if got.ObservedAt != at.UTC() || got.NetworkNamespace != "net:[4026531840]" || got.BootID == "" {
		t.Fatalf("identity metadata: %+v", got)
	}
	if len(got.Listeners) != 1 || got.Listeners[0].Ownership != "attributed" || len(got.Listeners[0].Owners) != 1 {
		t.Fatalf("listeners: %+v", got.Listeners)
	}
	owner := got.Listeners[0].Owners[0]
	if owner.PID != 42 || owner.StartTimeTicks != 9876 || owner.EffectiveUID != 1001 || owner.SystemdUnit != "nginx.service" {
		t.Fatalf("owner: %+v", owner)
	}
	if got.Quality.ProcessesScanned != 1 || got.Quality.PermissionDenied != 0 || got.Quality.Malformed != 0 {
		t.Fatalf("quality: %+v", got.Quality)
	}
}

func TestIncompleteProcNetFailsSnapshot(t *testing.T) {
	root := fixtureProc(t)
	if err := os.Remove(filepath.Join(root, "net/tcp6")); err != nil {
		t.Fatal(err)
	}
	if _, err := scanProc(root, time.Now()); err == nil {
		t.Fatal("missing IPv6 table accepted as complete snapshot")
	}
}

func TestMalformedProcessDoesNotInventOwner(t *testing.T) {
	root := fixtureProc(t)
	writeFixture(t, filepath.Join(root, "42/stat"), "malformed")
	got, err := scanProc(root, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Listeners) != 1 || got.Listeners[0].Ownership != "unknown_partial" || len(got.Listeners[0].Owners) != 0 || got.Quality.Malformed != 1 {
		t.Fatalf("malformed process attribution: %+v", got)
	}
}
