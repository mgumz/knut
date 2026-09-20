// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut"
	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// live mode is a process wide switch, so a test turning it on turns it off
// again - otherwise the next test renders pages it never asked for.
func liveMode(t *testing.T) {
	t.Helper()
	view.SetLive(true)
	t.Cleanup(func() { view.SetLive(false) })
}

// livePace shortens the long poll: the tests wait for the same loop the
// server runs, just not for half a minute.
func livePace(t *testing.T, timeout time.Duration) {
	t.Helper()
	old := view.LiveTimeout
	view.LiveTimeout = timeout
	t.Cleanup(func() { view.LiveTimeout = old })
}

// getHX asks the way htmx does: it announces itself and gets a fragment.
func getHX(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("HX-Request", "true")
	h.ServeHTTP(rec, req)
	return rec
}

var liveURL = regexp.MustCompile(`hx-get="([^"]*)"`)

// armedURL is the url the rendered listing polls, with the entities of an
// html attribute resolved - that url is meant to be requested. a listing
// polls itself, so the url is relative and resolved against "folder".
func armedURL(t *testing.T, folder, body string) string {
	t.Helper()

	match := liveURL.FindStringSubmatch(body)
	if match == nil {
		t.Fatal("the listing is not armed for live updates")
	}

	return folder + strings.ReplaceAll(match[1], "&amp;", "&")
}

// without -live knut renders what it always rendered: no script, no
// attributes, nothing to reserve a uri for.
func TestLiveOffRendersNoJavaScript(t *testing.T) {

	bodies := map[string]string{
		"listing": get(DirListHandler(testFS()), "/").Body.String(),
		"upload":  get(UploadHandler(t.TempDir()), "/upload").Body.String(),
	}

	// the word "htmx" itself is in the stylesheet, which is inlined into
	// every page: what must not be there is a script, an attribute for one
	// or the uri it would be served from
	for page, body := range bodies {
		for _, unwanted := range []string{"hx-get", "hx-post", "hx-on", knut.LiveAssetURI, "<script"} {
			if strings.Contains(body, unwanted) {
				t.Errorf("the %s page mentions %q without -live", page, unwanted)
			}
		}
	}
}

// with -live the listing carries the script tag and polls itself
func TestLiveListingIsArmed(t *testing.T) {

	liveMode(t)

	body := get(DirListHandler(http.Dir(testTree(t))), "/?sort=size&order=desc").Body.String()

	for _, want := range []string{
		`<script src="` + knut.LiveAssetURI + `" defer></script>`,
		`<div id="listing" hx-get=`,
		`hx-trigger="load"`,
		`hx-select="#listing"`,
		`hx-swap="outerHTML"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("a live listing does not render %q", want)
		}
	}

	// the sort has to survive the refresh, and the state of what is on
	// screen has to travel with the poll
	if got := armedURL(t, "/", body); !strings.HasPrefix(got, "/?sort=size&order=desc&live=") {
		t.Errorf("the listing polls %q", got)
	}
}

// an empty folder renders no table - the block around it is what carries
// the poll, so a folder which fell empty keeps updating
func TestLiveEmptyFolderIsArmed(t *testing.T) {

	liveMode(t)

	body := get(DirListHandler(http.Dir(t.TempDir())), "/").Body.String()

	if strings.Contains(body, "<table") {
		t.Fatal("an empty folder must not render a table")
	}
	if !strings.Contains(body, `<div id="listing" hx-get=`) {
		t.Error("an empty live listing does not poll")
	}
}

// htmx replaces a piece of a page which is already on screen: it gets that
// piece, not another layout around it
func TestLiveFragment(t *testing.T) {

	liveMode(t)

	body := getHX(DirListHandler(testFS()), "/").Body.String()

	if strings.Contains(body, "<!doctype html>") {
		t.Error("htmx got a whole page instead of the listing")
	}
	if !strings.HasPrefix(body, `<div id="listing"`) {
		t.Errorf("the fragment starts with %.40q", body)
	}
	if !strings.Contains(body, "a.txt") {
		t.Error("the fragment does not list the folder")
	}
}

// the same request without -live is an ordinary one, whoever sends it
func TestLiveFragmentNeedsLiveMode(t *testing.T) {

	if body := getHX(DirListHandler(testFS()), "/").Body.String(); !strings.Contains(body, "<!doctype html>") {
		t.Error("without -live an HX-Request must be answered with a page")
	}
}

// a poll is held until the folder moves
func TestLiveWatchHoldsUntilChange(t *testing.T) {

	liveMode(t)
	livePace(t, 5*time.Second)

	root := testTree(t)
	handler := DirListHandler(http.Dir(root))

	poll := armedURL(t, "/", get(handler, "/").Body.String())

	answered := make(chan string, 1)
	go func() { answered <- getHX(handler, poll).Body.String() }()

	select {
	case <-answered:
		t.Fatal("the poll was answered before anything happened")
	case <-time.After(50 * time.Millisecond):
	}

	if err := os.WriteFile(filepath.Join(root, "d.txt"), nil, 0600); err != nil {
		t.Fatalf("writing d.txt: %v", err)
	}

	select {
	case body := <-answered:
		if !strings.Contains(body, "d.txt") {
			t.Error("the answer does not carry the new entry")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the poll was not answered after the folder changed")
	}
}

// a folder which changed between the render and the poll is answered right
// away: the state the client hands over is not the one on disk anymore
func TestLiveWatchAnswersStaleState(t *testing.T) {

	liveMode(t)
	livePace(t, time.Minute)

	handler := DirListHandler(http.Dir(testTree(t)))

	done := make(chan bool, 1)
	go func() { done <- get(handler, "/?sort=name&order=asc&live=notwhatisthere").Code == http.StatusOK }()

	select {
	case ok := <-done:
		if !ok {
			t.Error("a stale poll was not answered with a listing")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a stale poll was held instead of being answered")
	}
}

// nothing happens for a while: the client is sent home to come back
// instead of being held forever
func TestLiveWatchTimesOut(t *testing.T) {

	liveMode(t)
	livePace(t, 100*time.Millisecond)

	handler := DirListHandler(http.Dir(testTree(t)))
	poll := armedURL(t, "/", get(handler, "/").Body.String())

	start := time.Now()
	rec := getHX(handler, poll)

	if held := time.Since(start); held < 100*time.Millisecond {
		t.Errorf("the poll was answered after %v, before the deadline", held)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "a.txt") {
		t.Error("a timed out poll must render the folder again, so the client re-arms")
	}
}

// only a poll is held: an ordinary request for the same folder answers
func TestLiveWatchOnlyHoldsPolls(t *testing.T) {

	liveMode(t)
	livePace(t, time.Minute)

	done := make(chan bool, 1)
	go func() { done <- get(DirListHandler(http.Dir(testTree(t))), "/").Code == http.StatusOK }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a request without the live parameter was held")
	}
}

// a zip is served from a path, and that path is meant to be replaced
// underneath a running knut: its listing polls like any other
func TestLiveZipListingIsArmed(t *testing.T) {

	liveMode(t)

	body := get(ZipFSHandler(testZip(t), "", ""), "/?sort=size&order=desc").Body.String()

	if !strings.Contains(body, `<div id="listing" hx-get=`) {
		t.Error("a live zip listing does not poll")
	}
	if got := armedURL(t, "/", body); !strings.HasPrefix(got, "/?sort=size&order=desc&live=") {
		t.Errorf("the listing polls %q", got)
	}
}

// the end to end move this is all about: knut keeps running, the zip
// below it is replaced, the listing follows. on a real file - a replaced
// path is what no in-memory tree models.
func TestLiveZipWatchHoldsUntilReplaced(t *testing.T) {

	liveMode(t)
	livePace(t, 5*time.Second)
	zipPace(t, 10*time.Millisecond)

	name := testZip(t)
	handler := ZipFSHandler(name, "", "")

	poll := armedURL(t, "/", get(handler, "/").Body.String())

	answered := make(chan string, 1)
	go func() { answered <- getHX(handler, poll).Body.String() }()

	select {
	case <-answered:
		t.Fatal("the poll was answered before the zip was replaced")
	case <-time.After(50 * time.Millisecond):
	}

	writeZip(t, name, zipItem{"a.txt", "knut"}, zipItem{"d.txt", "brand new"})

	select {
	case body := <-answered:
		if !strings.Contains(body, "d.txt") {
			t.Error("the answer does not carry the entry of the replaced zip")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the poll was not answered after the zip was replaced")
	}
}

// the upload form posts through htmx and reports progress
func TestLiveUploadForm(t *testing.T) {

	liveMode(t)

	body := get(UploadHandler(t.TempDir()), "/upload").Body.String()

	for _, want := range []string{
		`hx-post="/upload"`,
		`hx-encoding="multipart/form-data"`,
		`hx-target="#upload"`,
		"hx-on::xhr:progress",
		`<progress id="upload-progress"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the live upload form does not render %q", want)
		}
	}
}

// an in-memory tree has no path to hand a watcher. it is listed like any
// other, it just never asks the client to come back for an answer which
// could not arrive.
func TestLiveListingWithoutAPathIsNotArmed(t *testing.T) {

	liveMode(t)

	body := get(DirListHandler(testFS()), "/").Body.String()

	if strings.Contains(body, "hx-get") {
		t.Error("a listing which cannot be watched armed a poll")
	}
	if !strings.Contains(body, "a.txt") {
		t.Error("the listing did not render its entries")
	}
}

// the same listing, asked as a poll: answered, not held
func TestLivePollWithoutAPathIsNotHeld(t *testing.T) {

	liveMode(t)
	livePace(t, time.Minute)

	done := make(chan int, 1)
	go func() { done <- get(DirListHandler(testFS()), "/?live=whatever").Code }()

	select {
	case code := <-done:
		if code != 200 {
			t.Errorf("got status %d, want 200", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a poll for an unwatchable tree was held")
	}
}
