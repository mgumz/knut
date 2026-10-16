// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mgumz/knut/internal/pkg/knut"
	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// the whole way a request takes: muxer, the window it lands in, the
// logins of that window
func TestAuthPerWindow(t *testing.T) {

	photos, docs := t.TempDir(), t.TempDir()
	muxer, windows := prepareTrees(http.NewServeMux(), []string{"/photos:" + photos, "/docs:" + docs})

	opts := &knut.Opts{Auth: []string{"admin:pw", "/photos=alice:pw2"}, DoLive: true}
	guard := buildGuard(opts, windows)
	serveIndex(muxer, windows, guard)
	t.Cleanup(func() { view.SetIndex(false) })
	serveNotFound(muxer, windows)
	h := buildHandlerChain(muxer, opts, guard)

	tests := []struct {
		target, user, password string
		want                   int
		realm                  string
	}{
		{"/photos/", "", "", http.StatusUnauthorized, "knut /photos"},
		{"/photos/", "alice", "pw2", http.StatusOK, ""},
		{"/photos/?zip", "alice", "pw2", http.StatusOK, ""},
		{"/photos/", "admin", "pw", http.StatusUnauthorized, "knut /photos"},
		{"/docs/", "", "", http.StatusUnauthorized, "knut"},
		{"/docs/", "admin", "pw", http.StatusOK, ""},
		{"/docs/", "alice", "pw2", http.StatusUnauthorized, "knut"},
		{"/", "", "", http.StatusUnauthorized, "knut"},
		{"/", "admin", "pw", http.StatusOK, ""},
		{"/unmapped", "admin", "pw", http.StatusNotFound, ""},
		{knut.LiveAssetURI, "", "", http.StatusOK, ""},
	}

	for _, tt := range tests {
		req := browse(tt.target)
		if tt.user != "" {
			req.SetBasicAuth(tt.user, tt.password)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != tt.want {
			t.Errorf("%s as %q: got status %d, want %d", tt.target, tt.user, rec.Code, tt.want)
		}
		got := rec.Header().Get("WWW-Authenticate")
		if tt.realm == "" && got != "" {
			t.Errorf("%s as %q: got %q on a %d", tt.target, tt.user, got, rec.Code)
		}
		if want := `realm="` + tt.realm + `"`; tt.realm != "" && !strings.Contains(got, want) {
			t.Errorf("%s as %q: got %q, want %s", tt.target, tt.user, got, want)
		}
	}
}

// the index lists every window and marks the ones asking for a login
func TestAuthIndexMarksWindows(t *testing.T) {

	muxer, windows := prepareTrees(http.NewServeMux(), []string{"/photos:" + t.TempDir(), "/docs:" + t.TempDir()})
	guard := buildGuard(&knut.Opts{Auth: []string{"/photos=alice:pw"}}, windows)
	serveIndex(muxer, windows, guard)
	t.Cleanup(func() { view.SetIndex(false) })

	rec := httptest.NewRecorder()
	req := browse("/?text")
	muxer.ServeHTTP(rec, req)

	if got, want := rec.Body.String(), "/photos/  (login)\n/docs/\n"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestAuthWithoutEntries(t *testing.T) {

	if buildGuard(&knut.Opts{}, []string{"/d/"}) != nil {
		t.Error("no entries make a guard")
	}
}
