// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut"
)

// live mode is a process wide switch, so nothing below runs in parallel:
// a t.Parallel() here would have one test render the pages of another.

// statusPage is the model the "status" block renders - the smallest page
// there is, and one every handler falls back on.
func statusPage(page Page) any {
	return struct {
		Page
		Code int
		Text string
	}{Page: page, Code: http.StatusNotFound, Text: http.StatusText(http.StatusNotFound)}
}

// listOne hands out a folder of a single file, along with the state that
// folder fingerprints to. the entries are copied per read: a listing sorts
// what it is given in place.
func listOne() (ReadListing, string) {

	entries := []ListEntry{NewListEntry("a.txt", 12, modTime(15), false)}
	state := LiveState(entries)

	return func() ([]ListEntry, string, error) {
		return append([]ListEntry(nil), entries...), state, nil
	}, state
}

// the layout pulls htmx in only while live mode is on. without "-live"
// knut renders the html it rendered before: no script tag, no reserved
// uri on the page.
func TestPagePullsHtmxInOnlyInLiveMode(t *testing.T) {

	if page := NewPage(""); page.Live {
		t.Error("live mode is off, a fresh page is marked live anyway")
	}

	body, err := Render(Template("status"), statusPage(NewPage("")))
	if err != nil {
		t.Fatalf("rendering with live mode off: %v", err)
	}
	if strings.Contains(string(body), "<script") {
		t.Errorf("live mode is off, the page still carries a script tag:\n%s", body)
	}

	liveMode(t)

	page := NewPage("")
	if !page.Live {
		t.Error("live mode is on, a fresh page is not marked live")
	}
	if page.LiveURI != knut.LiveAssetURI {
		t.Errorf("the page points at %q, the asset is served at %q", page.LiveURI, knut.LiveAssetURI)
	}

	body, err = Render(Template("status"), statusPage(page))
	if err != nil {
		t.Fatalf("rendering with live mode on: %v", err)
	}
	if want := `<script src="` + knut.LiveAssetURI + `" defer></script>`; !strings.Contains(string(body), want) {
		t.Errorf("live mode is on, the page does not pull htmx in:\n%s", body)
	}
}

// a listing arms its poll only when there is both a live mode to poll in
// and a folder to poll: an armed listing without a watch sends the client
// back for an answer which can never come.
func TestListingArmsThePollOnlyWhenItCanWatch(t *testing.T) {

	read, state := listOne()

	for _, test := range []struct {
		name  string
		live  bool
		dir   string
		armed bool
	}{
		{"live off", false, t.TempDir(), false},
		{"live on, nothing to watch", true, "", false},
		{"live on, a folder to watch", true, t.TempDir(), true},
	} {
		t.Run(test.name, func(t *testing.T) {

			if test.live {
				liveMode(t)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept", "text/html")
			if err := Listing(rec, req, read, ListOpts{Dir: test.dir}); err != nil {
				t.Fatalf("listing: %v", err)
			}

			// the poll is spelled out in the markup: hx-get hands the sort
			// and the state of what is on screen back. "&" is an attribute
			// value here, so the template escapes it.
			body := rec.Body.String()
			want := `hx-get="?sort=name&amp;order=asc&amp;` + liveParam + `=` + state + `"`

			if armed := strings.Contains(body, want); armed != test.armed {
				t.Errorf("armed = %v, want %v - looked for %s in:\n%s", armed, test.armed, want, body)
			}
			if test.armed && !strings.Contains(body, `hx-trigger="load"`) {
				t.Errorf("the listing carries a poll nothing triggers:\n%s", body)
			}
		})
	}
}

// htmx replaces a piece of a page which is already on screen, so it gets
// the content block alone. the same header without live mode is no reason
// to answer half a page - nothing asked for it.
func TestHtmxGetsTheFragmentOnlyInLiveMode(t *testing.T) {

	for _, test := range []struct {
		name     string
		live     bool
		fragment bool
	}{
		{"live off", false, false},
		{"live on", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {

			if test.live {
				liveMode(t)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("HX-Request", "true")

			WriteFor(rec, req, Template("status"), statusPage(NewPage("")))

			body := rec.Body.String()
			if !strings.Contains(body, `<span class="code">404</span>`) {
				t.Fatalf("the content is missing either way:\n%s", body)
			}

			if got := !strings.Contains(body, "<!doctype html>"); got != test.fragment {
				t.Errorf("fragment = %v, want %v:\n%s", got, test.fragment, body)
			}
		})
	}
}

// a request carrying the state it already shows is a poll. with live mode
// off there is nothing to poll with, so the request is answered at once
// rather than held for the length of a timeout.
func TestListingHoldsAPollOnlyInLiveMode(t *testing.T) {

	read, state := listOne()
	livePace(t, time.Second)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?"+liveParam+"="+state, nil)

	start := time.Now()
	if err := Listing(rec, req, read, ListOpts{Dir: t.TempDir()}); err != nil {
		t.Fatalf("listing: %v", err)
	}

	if held := time.Since(start); held >= LiveTimeout {
		t.Errorf("live mode is off, the poll was held for %v anyway", held)
	}
}
