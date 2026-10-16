// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package auth

import (
	"slices"
	"strings"
	"testing"
)

func newGuard(t *testing.T, values ...string) *Guard {
	t.Helper()
	entries := []Entry{}
	for _, value := range values {
		e, err := ParseEntry(value)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	g, err := NewGuard(entries)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// a window with logins of its own asks for those alone, every other
// window for the global ones
func TestGuardFor(t *testing.T) {

	g := newGuard(t, "admin:pw", "/photos=alice:pw2")

	photos := g.For("/photos/")
	if photos == nil || !photos.Check("alice", "pw2") {
		t.Error("/photos/ does not let alice in")
	}
	if photos.Check("admin", "pw") {
		t.Error("/photos/ lets the global admin in")
	}
	if g.For("/photos") != photos {
		t.Error(`"/photos" and "/photos/" are not the same window`)
	}

	for _, window := range []string{"/docs/", "/{$}", "/", ""} {
		if l := g.For(window); l == nil || !l.Check("admin", "pw") {
			t.Errorf("%q does not ask for the global login", window)
		}
	}
}

func TestGuardWithoutGlobal(t *testing.T) {

	g := newGuard(t, "/photos=alice:pw")

	if g.For("/docs/") != nil {
		t.Error("a window without logins asks for some")
	}

	var none *Guard
	if none.For("/photos/") != nil {
		t.Error("no guard asks for a login")
	}
}

func TestGuardRealm(t *testing.T) {

	g := newGuard(t, "admin:pw", `/a"b/=alice:pw`)

	if got := g.For("/x").Realm(); got != "knut" {
		t.Errorf("got global realm %q", got)
	}
	if got := g.For(`/a"b/`).Realm(); got != `knut /a\"b` {
		t.Errorf("got realm %q, want the quote escaped", got)
	}
}

func TestGuardRefusesTwice(t *testing.T) {

	a, _ := ParseEntry("/p=alice:one")
	b, _ := ParseEntry("/p/=alice:two")

	if _, err := NewGuard([]Entry{a, b}); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("got %v, want a refusal", err)
	}
}

func TestGuardUnknown(t *testing.T) {

	g := newGuard(t, "/photos=alice:pw", "/photo=bob:pw")

	got := g.Unknown([]string{"/photos/", "/docs/"})
	if !slices.Equal(got, []string{"/photo"}) {
		t.Errorf("got %v, want [/photo]", got)
	}
}

func TestGuardNested(t *testing.T) {

	g := newGuard(t, "/photos=alice:pw", "/photos/own=bob:pw")

	got := g.Nested([]string{"/photos/", "/photos/pub/", "/photos/own/", "/docs/"})
	want := [][2]string{{"/photos/pub/", "/photos/"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// a bcrypt login is checked once, then remembered; a failed one never is
func TestLoginsRemember(t *testing.T) {

	g := newGuard(t, mustHash(t, "alice", "pw"))
	l := g.For("/")

	if l.Check("alice", "wrong") {
		t.Fatal("a wrong password passes")
	}
	if !l.Check("alice", "pw") {
		t.Fatal("the right password fails")
	}

	remembered := 0
	l.passed.Range(func(_, _ any) bool { remembered++; return true })
	if remembered != 1 {
		t.Errorf("got %d remembered logins, want 1", remembered)
	}
	if !l.Check("alice", "pw") || l.Check("alice", "wrong") {
		t.Error("checking from memory gets it wrong")
	}
}
