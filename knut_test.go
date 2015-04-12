// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"net/url"
	"path/filepath"
	"testing"
)

func TestAtoiInRange(t *testing.T) {

	tests := []struct {
		n                          string
		min, max, fallback, result int
	}{
		// ok
		{"", 0, 5, 10, 10}, // empty input
		{"0", 0, 5, 10, 0},
		{"5", 0, 5, 10, 5},
		// failures
		{"9", 0, 5, 10, 10}, // out of range
		{"-1", 0, 5, 10, 10},
		{"a", 0, 5, 10, 10}, // invalid input
	}

	for i, test := range tests {
		n, err := atoiInRange(test.n, test.min, test.max, test.fallback)
		t.Logf("case %d: %q => %d, (%v)", i, test.n, n, err)
		if n != test.result {
			t.Errorf("case %d: %d != %d for %q, %v",
				i, n, test.result, test.n, err)
		}
	}
}

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
		win, tree, err := getWindowAndTree(test.in)
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
		out := filepath.ToSlash(localFileName(u))
		t.Logf("case %d: localFilename(%q): %q", i, u.String(), out)
		if out != test.out {
			t.Errorf("case %d: localFilename(%q): expected %q, got %q",
				i, u.String(), test.out, out)
		}
	}
}
