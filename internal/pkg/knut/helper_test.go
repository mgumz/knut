package knut

import (
	"net/url"
	"path/filepath"
	"testing"
)

func TestGetWindowAndTree(t *testing.T) {
	tests := []struct {
		in, window, tree string
		err              error
	}{
		{"", "", "", errMissingSep},
		{"w", "", "", errMissingSep},
		{":rest", "", "", errEmptyPairParts},
		{"w:t", "w", "t", nil},
		{"w:t:rest", "w", "t:rest", nil},
	}

	for i, test := range tests {
		win, tree, err := GetWindowAndTree(test.in)
		t.Logf("case %d: %q => %q,%q,%v", i, test.in, win, tree, err)
		if win != test.window || tree != test.tree || err != test.err {
			t.Errorf("case %d: %q,%q,%v (%q) does not match %v",
				i, win, tree, err, test.in, test)
		}
	}
}

func TestLocalFilename(t *testing.T) {
	tests := []struct{ in, out string }{
		{"s://./cwd.txt", "cwd.txt"},
		{"s:///absolute.txt", "/absolute.txt"},
		{"s://../relative.txt", "../relative.txt"},
	}

	for i, test := range tests {
		u, err := url.Parse(test.in)
		if err != nil {
			t.Errorf("case %d: parsing test.in %q: %v", i, test.in, err)
		}
		out := filepath.ToSlash(LocalFilename(u))
		t.Logf("case %d: localFilename(%q): %q", i, u.String(), out)
		if out != test.out {
			t.Errorf("case %d: localFilename(%q): expected %q, got %q",
				i, u.String(), test.out, out)
		}
	}
}
