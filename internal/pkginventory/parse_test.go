package pkginventory

import (
	"strings"
	"testing"
	"time"
)

func TestParseDPKGSortsAndIdentifiesInventory(t *testing.T) {
	raw := []byte("zlib1g\t1:1.3.dfsg-3.1\tarm64\nbase-files\t13.8+deb13u2\tarm64\n")
	got, err := ParseDPKG(raw, time.Date(2026, 10, 5, 20, 0, 0, 0, time.FixedZone("EDT", -4*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 1 || got.Manager != "dpkg" || got.PackageCount != 2 || got.Packages[0].Name != "base-files" || len(got.SHA256) != 64 {
		t.Fatalf("%#v", got)
	}
	if got.ObservedAt.Location() != time.UTC {
		t.Fatalf("timestamp=%v", got.ObservedAt)
	}
}

func TestParseDPKGRejectsMalformedOrAmbiguousRows(t *testing.T) {
	for _, raw := range [][]byte{
		nil,
		[]byte("only-name\n"),
		[]byte("name\tversion\tarch\textra\n"),
		[]byte("name\tversion\tarch\nname\tversion\tarch\n"),
		[]byte("name\tbad\x01version\tarch\n"),
		[]byte("name\t" + strings.Repeat("v", 1025) + "\tarch\n"),
		make([]byte, MaxOutputBytes+1),
	} {
		if _, err := ParseDPKG(raw, time.Now()); err == nil {
			t.Fatalf("accepted %d bytes", len(raw))
		}
	}
}

func FuzzParseDPKGNeverPanics(f *testing.F) {
	f.Add([]byte("base-files\t1.0\tamd64\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > MaxOutputBytes+1 {
			t.Skip()
		}
		_, _ = ParseDPKG(raw, time.Unix(0, 0))
	})
}
