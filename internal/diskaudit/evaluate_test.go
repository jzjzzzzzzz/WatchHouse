package diskaudit

import (
	"errors"
	"testing"
)

func TestEvaluateCapacityOutcomes(t *testing.T) {
	got, err := Evaluate([]string{"/full", "/ok", "/missing", "/inodes"}, func(path string) (Sample, error) {
		switch path {
		case "/ok":
			return Sample{"ext4", 100 << 30, 30 << 30, 1000, 500}, nil
		case "/full":
			return Sample{"ext4", 100 << 30, 512 << 20, 1000, 500}, nil
		case "/inodes":
			return Sample{"ext4", 100 << 30, 30 << 30, 1000, 50}, nil
		default:
			return Sample{}, errors.New("not found")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]Result{}
	for _, result := range got {
		byPath[result.Path] = result
	}
	if byPath["/ok"].Status != Pass || byPath["/ok"].AvailablePercent != 30 {
		t.Fatalf("%#v", byPath["/ok"])
	}
	if byPath["/full"].Status != Fail || byPath["/full"].Detail != "available bytes below threshold" {
		t.Fatalf("%#v", byPath["/full"])
	}
	if byPath["/inodes"].Status != Fail || byPath["/inodes"].Detail != "available inode percentage below threshold" {
		t.Fatalf("%#v", byPath["/inodes"])
	}
	if byPath["/missing"].Status != Error {
		t.Fatalf("%#v", byPath["/missing"])
	}
}

func TestEvaluateRejectsZeroAndInvalidCatalog(t *testing.T) {
	got, err := Evaluate([]string{"/zero"}, func(string) (Sample, error) { return Sample{}, nil })
	if err != nil || got[0].Status != Error {
		t.Fatalf("got=%#v err=%v", got, err)
	}
	for _, paths := range [][]string{{"relative"}, {"/x", "/x"}, {""}} {
		if _, err := Evaluate(paths, func(string) (Sample, error) { return Sample{}, nil }); err == nil {
			t.Fatalf("accepted %#v", paths)
		}
	}
}

func TestPseudoFilesystemWithoutInodesUsesBlockThreshold(t *testing.T) {
	got, err := Evaluate([]string{"/pseudo"}, func(string) (Sample, error) { return Sample{"tmpfs", 10 << 30, 5 << 30, 0, 0}, nil })
	if err != nil || got[0].Status != Pass {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}
