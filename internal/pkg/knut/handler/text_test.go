// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// wantText checks "rec" is the text form, "want" line by line
func wantText(t *testing.T, rec *httptest.ResponseRecorder, want ...string) {
	t.Helper()

	if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf("got content type %q, want text/plain", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Accept") {
		t.Errorf("got Vary %q, want it to carry Accept", got)
	}
	if got, want := rec.Body.String(), strings.Join(want, "\n")+"\n"; got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// a listing is one line per entry, the columns lined up, the name last
func TestTextListing(t *testing.T) {

	rec := getText(DirListHandler(testFS()), "/?sort=size&order=desc")

	wantText(t, rec,
		"300 B  2026-09-19 13:04  b.txt",
		"200 B  2026-09-19 14:04  c.txt",
		"100 B  2026-09-19 15:04  a.txt",
		"-      2026-09-19 16:04  sub/")
}

func TestTextZipFSListing(t *testing.T) {

	rec := getText(ZipFSHandler(testZip(t), "", ""), "/")

	body := rec.Body.String()
	for _, want := range []string{"a.txt\n", "sub/\n", "empty/\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("the text listing does not carry %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<") {
		t.Errorf("the text listing carries markup:\n%s", body)
	}
}

// the query overrides what the Accept header asks for, either way
func TestTextOverrides(t *testing.T) {

	handler := DirListHandler(testFS())

	if body := getText(handler, "/?html").Body.String(); !strings.HasPrefix(body, "<!doctype html>") {
		t.Errorf("?html does not give the page:\n%.60s", body)
	}
	if body := get(handler, "/?text").Body.String(); strings.HasPrefix(body, "<") {
		t.Errorf("?text does not give the text form:\n%.60s", body)
	}
	if rec := get(handler, "/"); !strings.Contains(rec.Header().Get("Vary"), "Accept") {
		t.Error("the page of a listing does not vary by Accept")
	}
}

// the git page: how to clone it, the refs, the log
func TestTextGit(t *testing.T) {

	rec := getText(GitHandler(gitBin, testRepos(t), "/"), "/one/")
	body := rec.Body.String()

	for _, want := range []string{
		"git clone http://example.com/one/\n",
		"main  HEAD",
		"knut  second <b>bold</b>\n",
		"knut  first\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the text form does not carry %q:\n%s", want, body)
		}
	}
}

// the log goes on where the page goes on, spelled out in full
func TestTextGitPages(t *testing.T) {

	root := testRepos(t)
	head := gitLong(t, root)
	handler := GitHandler(gitBin, root, "/")

	first := getText(handler, "/one/").Body.String()
	if want := "older: http://example.com/one/?from=" + head + "&skip=50\n"; !strings.HasSuffix(first, want) {
		t.Errorf("the log does not end in %q:\n%s", want, first)
	}
	if got := strings.Count(first, "  knut  c"); got != 50 {
		t.Errorf("got %d commits, want 50", got)
	}

	second := getText(handler, "/one/?from="+head+"&skip=50").Body.String()
	if want := "newer: http://example.com/one/\n"; !strings.Contains(second, want) {
		t.Errorf("the second page does not carry %q:\n%s", want, second)
	}
}

func TestTextGitList(t *testing.T) {

	rec := getText(GitHandler(gitBin, testRepos(t), "/"), "/")

	body := rec.Body.String()
	for _, want := range []string{"main  2026-09-19 15:04  bare.git/\n", "-     -                 empty/\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("the repo list does not carry %q:\n%s", want, body)
		}
	}
}

func TestTextStatus(t *testing.T) {

	rec := getText(GitHandler(gitBin, testRepos(t), "/"), "/plain/")

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
	wantText(t, rec, "404 Not Found")
}

func TestTextIndex(t *testing.T) {

	rec := getText(IndexHandler([]string{"/d/", "/f.txt"}), "/")

	wantText(t, rec, "/d/", "/f.txt")
}

func TestTextMyIP(t *testing.T) {

	rec := getText(MyIPHandler("", false), "/myip")

	// httptest asks from 192.0.2.1
	wantText(t, rec, "192.0.2.1")
}
