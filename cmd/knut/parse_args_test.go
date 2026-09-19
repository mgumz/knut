// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// a folder mapped without a trailing "/" has to end up as a subtree
// pattern, otherwise http.ServeMux matches the window exactly and every
// link of the rendered listing misses the handler.
func TestFolderMappingServesItsChildren(t *testing.T) {

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# readme"), 0600); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}

	muxer, windows := prepareTrees(http.NewServeMux(), []string{"/files:" + root})
	if len(windows) != 1 {
		t.Fatalf("got windows %v, want one", windows)
	}
	if want := "/files/"; windows[0] != want {
		t.Errorf("got window %q, want %q", windows[0], want)
	}

	// the mux redirects the bare window to the subtree on its own
	rec := httptest.NewRecorder()
	muxer.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files", nil))
	if rec.Code/100 != 3 {
		t.Errorf("got status %d for /files, want a redirect", rec.Code)
	}
	if got, want := rec.Header().Get("Location"), "/files/"; got != want {
		t.Errorf("got location %q, want %q", got, want)
	}

	// and a file below it is served, not swallowed by a catch-all
	rec = httptest.NewRecorder()
	muxer.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/README.md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d for /files/README.md, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "# readme" {
		t.Errorf("got body %q, want %q", got, "# readme")
	}
}

// a mapped file stays an exact match, it has no subtree
func TestFileMappingStaysExact(t *testing.T) {

	root := t.TempDir()
	name := filepath.Join(root, "ding.txt")
	if err := os.WriteFile(name, []byte("dong"), 0600); err != nil {
		t.Fatalf("writing ding.txt: %v", err)
	}

	muxer, windows := prepareTrees(http.NewServeMux(), []string{"/ding.txt:" + name})
	if want := "/ding.txt"; windows[0] != want {
		t.Errorf("got window %q, want %q", windows[0], want)
	}

	rec := httptest.NewRecorder()
	muxer.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ding.txt", nil))
	if got := rec.Body.String(); got != "dong" {
		t.Errorf("got body %q, want %q", got, "dong")
	}
}

func TestSubtreeWindow(t *testing.T) {

	root := t.TempDir()
	file := filepath.Join(root, "a.txt")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}

	tests := []struct {
		window, tree, want string
	}{
		{"/uri", root, "/uri/"},
		{"/uri/", root, "/uri/"},
		{"/uri", "file://" + root, "/uri/"},
		{"/uri", file, "/uri"},
		{"/uri", "/does/not/exist", "/uri"},
		// these answer below their window
		{"/uri", "http://1.2.3.4/", "/uri/"},
		{"/uri", "https://1.2.3.4/", "/uri/"},
		{"/uri", "git://" + root, "/uri/"},
		{"/uri", "cgit://" + root, "/uri/"},
		{"/uri", "zipfs://" + file, "/uri/"},
		// these serve a single resource at their window
		{"/c.tgz", "tar+gz://" + root, "/c.tgz"},
		{"/c.tar", "tar://" + root, "/c.tar"},
		{"/z.zip", "zip://" + root, "/z.zip"},
		{"/qr.png", "qr://text", "/qr.png"},
		{"/ip", "myip://", "/ip"},
	}

	for _, test := range tests {
		if got := subtreeWindow(test.window, test.tree); got != test.want {
			t.Errorf("subtreeWindow(%q, %q): got %q, want %q",
				test.window, test.tree, got, test.want)
		}
	}
}

// a zip mapped below "/" has to find its entries: the handler looks them up
// by request path, so the window must be stripped off it. this is the
// mapping the README advertises.
func TestZipFSMappingBelowRoot(t *testing.T) {

	name := filepath.Join(t.TempDir(), "a.zip")
	file, err := os.Create(name)
	if err != nil {
		t.Fatalf("creating %q: %v", name, err)
	}
	zw := zip.NewWriter(file)
	for _, entry := range []string{"example.txt", "sub/deep.txt"} {
		w, err := zw.Create(entry)
		if err != nil {
			t.Fatalf("adding %q: %v", entry, err)
		}
		if _, err := w.Write([]byte("in zip")); err != nil {
			t.Fatalf("writing %q: %v", entry, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip: %v", err)
	}
	file.Close()

	muxer, windows := prepareTrees(http.NewServeMux(), []string{"/z.zip:zipfs://" + name})
	if want := "/z.zip/"; windows[0] != want {
		t.Errorf("got window %q, want %q", windows[0], want)
	}

	tests := []struct {
		path, want string
	}{
		{"/z.zip/example.txt", "in zip"},
		{"/z.zip/sub/deep.txt", "in zip"},
	}

	for _, test := range tests {
		rec := httptest.NewRecorder()
		muxer.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, test.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: got status %d, want %d", test.path, rec.Code, http.StatusOK)
			continue
		}
		if got := rec.Body.String(); got != test.want {
			t.Errorf("%s: got body %q, want %q", test.path, got, test.want)
		}
	}

	// the folder listing has to find the entries as well
	rec := httptest.NewRecorder()
	muxer.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/z.zip/", nil))
	body := rec.Body.String()
	for _, want := range []string{`<a href="example.txt">`, `<a href="sub/">`} {
		if !strings.Contains(body, want) {
			t.Errorf("listing of the zip does not link %q", want)
		}
	}
}

// the listing of a folder links its entries relative, so the page has to be
// served under a path ending in "/" for those links to resolve below it
func TestListingLinksResolveBelowTheWindow(t *testing.T) {

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0600); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}

	muxer, _ := prepareTrees(http.NewServeMux(), []string{"/files:" + root})

	rec := httptest.NewRecorder()
	muxer.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/", nil))
	if !strings.Contains(rec.Body.String(), `<a href="README.md">`) {
		t.Fatal("listing does not link README.md relative")
	}

	rec = httptest.NewRecorder()
	muxer.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/README.md", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d for the linked file, want %d", rec.Code, http.StatusOK)
	}
}
