// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

var listedName = regexp.MustCompile(`<td class="name"><a href="[^"]*">([^<]*)</a>`)

// listedNames returns the entries of a rendered listing, in the order they
// are rendered in.
func listedNames(body string) []string {
	names := []string{}
	for _, match := range listedName.FindAllStringSubmatch(body, -1) {
		names = append(names, match[1])
	}
	return names
}

// modTime dates an entry of the in-memory tree. fixed timestamps give a
// listing ordered by date exactly one right answer.
func modTime(hour int) time.Time {
	return time.Date(2026, time.September, 19, hour, 4, 0, 0, time.Local)
}

// testFS lays out a tree to list: 3 files of different size and age plus a
// folder. it is in memory - what those tests are about is the order and the
// rendering of a listing, not the filesystem below it.
func testFS() http.FileSystem {
	return http.FS(fstest.MapFS{
		"a.txt":        {Data: make([]byte, 100), ModTime: modTime(15)},
		"b.txt":        {Data: make([]byte, 300), ModTime: modTime(13)},
		"c.txt":        {Data: make([]byte, 200), ModTime: modTime(14)},
		"sub":          {Mode: fs.ModeDir, ModTime: modTime(16)},
		"sub/deep.txt": {ModTime: modTime(16)},
	})
}

// testTree lays out the same shape on disk. the tests using it are about
// how the handler behaves against a real filesystem - the redirects, the
// delegation to http.FileServer, the errors http.Dir returns.
func testTree(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0600); err != nil {
			t.Fatalf("writing %q: %v", name, err)
		}
	}

	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.txt"), nil, 0600); err != nil {
		t.Fatalf("writing deep.txt: %v", err)
	}

	return root
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestDirListSort(t *testing.T) {

	handler := DirListHandler(testFS())

	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"a.txt", "b.txt", "c.txt", "sub/"}},
		{"?sort=name&order=asc", []string{"a.txt", "b.txt", "c.txt", "sub/"}},
		{"?sort=name&order=desc", []string{"sub/", "c.txt", "b.txt", "a.txt"}},
		{"?sort=size&order=asc", []string{"sub/", "a.txt", "c.txt", "b.txt"}},
		{"?sort=size&order=desc", []string{"b.txt", "c.txt", "a.txt", "sub/"}},
		{"?sort=date&order=asc", []string{"b.txt", "c.txt", "a.txt", "sub/"}},
		{"?sort=date&order=desc", []string{"sub/", "a.txt", "c.txt", "b.txt"}},
		// unknown keys fall back to name, ascending
		{"?sort=owner&order=sideways", []string{"a.txt", "b.txt", "c.txt", "sub/"}},
	}

	for _, test := range tests {
		rec := get(handler, "/"+test.query)
		if rec.Code != http.StatusOK {
			t.Errorf("%q: got status %d, want %d", test.query, rec.Code, http.StatusOK)
			continue
		}
		got := listedNames(rec.Body.String())
		if strings.Join(got, " ") != strings.Join(test.want, " ") {
			t.Errorf("%q: got %v, want %v", test.query, got, test.want)
		}
	}
}

// by type the folders come first, the files follow, both by name
func TestDirListSortByType(t *testing.T) {

	handler := DirListHandler(http.FS(fstest.MapFS{
		"z.txt": {},
		"a.bin": {},
		"m":     {},
		"beta":  {Mode: fs.ModeDir},
		"alpha": {Mode: fs.ModeDir},
	}))

	rec := get(handler, "/?sort=type&order=asc")
	got := strings.Join(listedNames(rec.Body.String()), " ")
	if want := "alpha/ beta/ a.bin m z.txt"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	rec = get(handler, "/?sort=type&order=desc")
	got = strings.Join(listedNames(rec.Body.String()), " ")
	if want := "a.bin m z.txt alpha/ beta/"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	body := rec.Body.String()
	for _, want := range []string{`<td class="type">dir</td>`, `<td class="type">file</td>`} {
		if !strings.Contains(body, want) {
			t.Errorf("listing does not render %q", want)
		}
	}
}

func TestDirListShowsSizeAndDate(t *testing.T) {

	body := get(DirListHandler(testFS()), "/").Body.String()

	// a.txt is the 100 byte one, dated by modTime(15)
	for _, want := range []string{"100 B", "2026-09-19 15:04", modTime(15).Format(time.RFC3339)} {
		if !strings.Contains(body, want) {
			t.Errorf("listing does not mention %q", want)
		}
	}
}

// the header of the active column links to the opposite order, all others
// start ascending.
func TestDirListSortLinks(t *testing.T) {

	handler := DirListHandler(testFS())

	body := get(handler, "/?sort=size&order=asc").Body.String()
	for _, want := range []string{"?sort=size&amp;order=desc", "?sort=name&amp;order=asc", "↑"} {
		if !strings.Contains(body, want) {
			t.Errorf("listing sorted by size asc does not link %q", want)
		}
	}

	body = get(handler, "/?sort=size&order=desc").Body.String()
	for _, want := range []string{"?sort=size&amp;order=asc", "↓"} {
		if !strings.Contains(body, want) {
			t.Errorf("listing sorted by size desc does not link %q", want)
		}
	}
}

func TestDirListParentLink(t *testing.T) {

	handler := DirListHandler(testFS())

	if body := get(handler, "/").Body.String(); strings.Contains(body, `>../<`) {
		t.Error("the root of a mapping must not link to a parent")
	}
	if body := get(handler, "/sub/").Body.String(); !strings.Contains(body, `>../<`) {
		t.Error("a subfolder must link to its parent")
	}
}

// an empty folder has no table to show, it gets a tree instead
func TestDirListEmptyFolder(t *testing.T) {

	fsys := http.FS(fstest.MapFS{"bare": {Mode: fs.ModeDir}})

	body := get(DirListHandler(fsys), "/bare/").Body.String()

	if strings.Contains(body, "<table") {
		t.Error("an empty folder must not render a table")
	}
	for _, want := range []string{`<pre class="tree">`, "this tree is bare", `>../<`} {
		if !strings.Contains(body, want) {
			t.Errorf("empty listing does not contain %q", want)
		}
	}
}

// a folder addressed without a trailing "/" has to redirect, otherwise the
// relative links of the listing resolve against the wrong folder.
func TestDirListRedirectsToFolder(t *testing.T) {

	handler := DirListHandler(http.Dir(testTree(t)))

	rec := get(handler, "/sub?sort=date")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if got, want := rec.Header().Get("Location"), "sub/?sort=date"; got != want {
		t.Errorf("got location %q, want %q", got, want)
	}
}

// files are none of the listings business, they belong to http.FileServer
func TestDirListServesFiles(t *testing.T) {

	root := testTree(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("knut"), 0600); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}

	rec := get(DirListHandler(http.Dir(root)), "/a.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "knut" {
		t.Errorf("got body %q, want %q", got, "knut")
	}
}

// an index.html shadows the generated listing, like in http.FileServer
func TestDirListIndexPage(t *testing.T) {

	root := testTree(t)
	if err := os.WriteFile(filepath.Join(root, indexPage), []byte("hand written"), 0600); err != nil {
		t.Fatalf("writing %q: %v", indexPage, err)
	}

	if got := get(DirListHandler(http.Dir(root)), "/").Body.String(); got != "hand written" {
		t.Errorf("got body %q, want %q", got, "hand written")
	}
}

func TestDirListMissing(t *testing.T) {

	rec := get(DirListHandler(http.Dir(testTree(t))), "/nope/")
	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

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
