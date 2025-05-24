// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import "testing"

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
