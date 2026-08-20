package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		cur, lat string
		want     int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"v1.2.3", "1.2.4", -1},
	}
	for _, tc := range cases {
		if got := Compare(tc.cur, tc.lat); got != tc.want {
			t.Fatalf("%s vs %s = %d, want %d", tc.cur, tc.lat, got, tc.want)
		}
	}
}

func TestValidateWindowsExe(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.exe")
	if err := os.WriteFile(p, []byte("<html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateWindowsExe(p); err == nil {
		t.Fatal("expected error for html file")
	}
	good := filepath.Join(dir, "good.exe")
	buf := make([]byte, minExeSize+2)
	buf[0] = 'M'
	buf[1] = 'Z'
	if err := os.WriteFile(good, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateWindowsExe(good); err != nil {
		t.Fatal(err)
	}
}
