// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package auth

import (
	"crypto/sha256"
	"slices"
	"strings"
	"sync"
)

// Logins are the users one window lets in, or the global ones.
type Logins struct {
	realm string
	users map[string]secret

	// a bcrypt check is slow on purpose, and basic auth sends the
	// password with every request: a live listing, every object of a
	// git clone. a login which passed once passes again from here. a
	// failed one is not remembered.
	passed sync.Map
}

func newLogins(realm string) *Logins {
	escape := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return &Logins{realm: escape.Replace(realm), users: map[string]secret{}}
}

// Realm is the realm of the "WWW-Authenticate" header, quoted-string safe.
// One realm per window: the browser keeps the logins apart and asks for
// each window on its own.
func (l *Logins) Realm() string {
	return l.realm
}

// Check reports whether user and password are one of the logins.
func (l *Logins) Check(user, password string) bool {

	s, ok := l.users[user]
	if !ok {
		return false
	}

	key := sha256.Sum256([]byte(user + "\x00" + password))
	if _, ok := l.passed.Load(key); ok {
		return true
	}
	if !s.matches(password) {
		return false
	}
	l.passed.Store(key, struct{}{})
	return true
}

// Guard knows the logins of every window. A window with logins of its own
// asks for those, every other one for the global logins - both cannot
// apply: a browser sends one login per request.
type Guard struct {
	global  *Logins
	windows map[string]*Logins
}

// NewGuard groups entries by their window. A user given twice for the same
// window is an error, the second password would be a guess.
func NewGuard(entries []Entry) (*Guard, error) {

	g := &Guard{windows: map[string]*Logins{}}

	for _, e := range entries {
		var l *Logins
		if e.Window == "" {
			if g.global == nil {
				g.global = newLogins("knut")
			}
			l = g.global
		} else {
			window := normalize(e.Window)
			if l = g.windows[window]; l == nil {
				l = newLogins("knut " + window)
				g.windows[window] = l
			}
		}
		if _, ok := l.users[e.User]; ok {
			return nil, e.errorf("given twice")
		}
		l.users[e.User] = e.secret
	}

	return g, nil
}

// For returns the logins a window asks for. nil: the window is open. The
// window is the pattern the muxer routed a request by, anything which is
// no mapping (the index, the 404 page) gets the global logins.
func (g *Guard) For(window string) *Logins {

	if g == nil {
		return nil
	}
	if l, ok := g.windows[normalize(window)]; ok {
		return l
	}
	return g.global
}

// Unknown returns the windows given with an entry which none of windows
// matches, sorted. Their entries are never asked for.
func (g *Guard) Unknown(windows []string) []string {

	if g == nil {
		return nil
	}

	published := map[string]bool{}
	for _, window := range windows {
		published[normalize(window)] = true
	}

	unknown := []string{}
	for window := range g.windows {
		if !published[window] {
			unknown = append(unknown, window)
		}
	}
	slices.Sort(unknown)
	return unknown
}

// Nested returns the windows which lie below a window with logins of its
// own, without logins of their own: the muxer routes by the longest
// pattern, so they do not ask for the logins of the window around them.
// Each element is a pair of the inner and the outer window.
func (g *Guard) Nested(windows []string) [][2]string {

	if g == nil {
		return nil
	}

	pairs := [][2]string{}
	for _, outer := range windows {
		if _, own := g.windows[normalize(outer)]; !own || !strings.HasSuffix(outer, "/") {
			continue
		}
		for _, inner := range windows {
			if _, own := g.windows[normalize(inner)]; own || inner == outer || !strings.HasPrefix(inner, outer) {
				continue
			}
			pairs = append(pairs, [2]string{inner, outer})
		}
	}
	return pairs
}

// normalize makes "/photos" and "/photos/" the same window: the user
// writes either, the muxer knows the one subtreeWindow made of it.
func normalize(window string) string {
	if window == "/" {
		return window
	}
	return strings.TrimSuffix(window, "/")
}
