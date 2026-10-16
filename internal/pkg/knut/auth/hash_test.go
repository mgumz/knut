// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package auth

import (
	"strings"
	"testing"
)

func TestHash(t *testing.T) {

	entry, err := Hash("alice", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(entry, "alice:$2a$10$") {
		t.Errorf("got %q", entry)
	}

	e, err := ParseEntry(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !e.secret.matches("pw") {
		t.Error("the entry -hash prints does not let its user in")
	}

	for _, tt := range []struct{ user, password, want string }{
		{"", "pw", "empty"},
		{"a:b", "pw", `":"`},
		{"/a", "pw", `"/"`},
		{"alice", "", "empty"},
		{"alice", strings.Repeat("x", 73), "72 bytes"},
	} {
		if _, err := Hash(tt.user, tt.password); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Hash(%q, …): got %v, want %q in it", tt.user, err, tt.want)
		}
	}
}
