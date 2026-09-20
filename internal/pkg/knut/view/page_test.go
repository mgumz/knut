// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import "testing"

func TestHumanSize(t *testing.T) {

	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{3 * 1024 * 1024 * 1024, "3.0 GiB"},
	}

	for _, test := range tests {
		if got := humanSize(test.size); got != test.want {
			t.Errorf("humanSize(%d): got %q, want %q", test.size, got, test.want)
		}
	}
}
