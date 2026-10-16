// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestParseEntry(t *testing.T) {

	t.Setenv("KNUT_TEST_PW", "from-env")
	hash := mustHash(t, "alice", "pw")[len("alice:"):]

	tests := []struct {
		value    string
		window   string
		user     string
		password string
	}{
		{"admin:pw", "", "admin", "pw"},
		{"/photos=alice:pw", "/photos", "alice", "pw"},
		{"/photos/=alice:pw", "/photos/", "alice", "pw"},
		{"/git=ci:token:with:colons", "/git", "ci", "token:with:colons"},
		{"/docs=carol:env:KNUT_TEST_PW", "/docs", "carol", "from-env"},
		{"/p=alice:" + hash, "/p", "alice", "pw"},
		{"/a=b=alice:pw", "/a", "b=alice", "pw"},
	}

	for _, tt := range tests {
		e, err := ParseEntry(tt.value)
		if err != nil {
			t.Errorf("%q: %v", tt.value, err)
			continue
		}
		if e.Window != tt.window || e.User != tt.user {
			t.Errorf("%q: got window %q user %q, want %q %q", tt.value, e.Window, e.User, tt.window, tt.user)
		}
		if !e.secret.matches(tt.password) {
			t.Errorf("%q: the password %q does not match", tt.value, tt.password)
		}
		if e.secret.matches(tt.password + "x") {
			t.Errorf("%q: a wrong password matches", tt.value)
		}
	}
}

func TestParseEntryRefuses(t *testing.T) {

	tests := map[string]string{
		"admin":                         `missing ":"`,
		":pw":                           "the name is empty",
		"admin:":                        "the password is empty",
		"/photos:pw":                    `missing "="`,
		"/photos=alice":                 `missing ":"`,
		"/d=alice:env:":                 "needs the name",
		"/d=alice:env:KNUT_UNSET_XYZ":   "unset or empty",
		"/d=alice:$apr1$salt$hash":      "only bcrypt",
		"/d=alice:{SHA}W6ph5Mm5Pz8GgiU": "only bcrypt",
		"/d=alice:$2y$10$tooshort":      "not a valid bcrypt hash",
	}

	for value, want := range tests {
		_, err := ParseEntry(value)
		if err == nil {
			t.Errorf("%q: no error, want one with %q", value, want)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %q, want %q in it", value, err, want)
		}
	}
}

// an error names the window and the user, never the password
func TestParseEntryKeepsThePasswordOut(t *testing.T) {

	_, err := ParseEntry("/d=alice:$apr1$secret-part")
	if err == nil {
		t.Fatal("no error")
	}
	if strings.Contains(err.Error(), "secret-part") {
		t.Errorf("the error quotes the password: %q", err)
	}
	if !strings.Contains(err.Error(), "/d=alice") {
		t.Errorf("the error does not say where: %q", err)
	}
}

func TestReadFile(t *testing.T) {

	name := writeFile(t, 0o600, `# comment

admin:pw
  /photos=alice:pw2
`)

	entries, err := ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[1].Window != "/photos" || entries[1].User != "alice" {
		t.Errorf("got %+v", entries[1])
	}
}

func TestReadFileNamesTheLine(t *testing.T) {

	name := writeFile(t, 0o600, "admin:pw\n\nbroken\n")

	_, err := ReadFile(name)
	if err == nil || !strings.Contains(err.Error(), name+":3:") {
		t.Errorf("got %v, want the error at line 3", err)
	}
}

func TestReadFileRefusesOpenFiles(t *testing.T) {

	name := writeFile(t, 0o644, "admin:pw\n")

	_, err := ReadFile(name)
	if err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Errorf("got %v, want a refusal", err)
	}
}

func writeFile(t *testing.T, mode os.FileMode, content string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "knut.auth")
	if err := os.WriteFile(name, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	// the umask may have taken bits away
	if err := os.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
	return name
}

// mustHash is an entry "user:<bcrypt hash>", at the cheapest cost: the
// tests check what is done with a hash, not how strong it is.
func mustHash(t *testing.T, user, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return user + ":" + string(hash)
}
