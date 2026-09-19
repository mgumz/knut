// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testZip writes a zip carrying a file, a folder with an entry of its own
// and an explicit folder entry.
func testZip(t *testing.T) string {
	t.Helper()

	name := filepath.Join(t.TempDir(), "tree.zip")
	file, err := os.Create(name)
	if err != nil {
		t.Fatalf("creating %q: %v", name, err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	entries := []struct {
		name    string
		content string
	}{
		{"a.txt", "knut"},
		{"sub/b.txt", "knut knut"},
		{"empty/", ""},
	}

	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.Modified = time.Date(2026, time.September, 19, 15, 4, 0, 0, time.Local)
		w, err := zw.CreateHeader(header)
		if err != nil {
			t.Fatalf("adding %q: %v", entry.name, err)
		}
		if _, err := w.Write([]byte(entry.content)); err != nil {
			t.Fatalf("writing %q: %v", entry.name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("closing %q: %v", name, err)
	}

	return name
}

func TestZipFSListsFolder(t *testing.T) {

	handler := ZipFSHandler(testZip(t), "", "")

	rec := get(handler, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	got := strings.Join(listedNames(body), " ")
	if want := "a.txt empty/ sub/"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}
	for _, want := range []string{"4 B", "2026-09-19 15:04"} {
		if !strings.Contains(body, want) {
			t.Errorf("listing does not mention %q", want)
		}
	}
}

// folders which have no entry of their own are picked up from the names of
// the entries below them
func TestZipFSListsSubFolder(t *testing.T) {

	rec := get(ZipFSHandler(testZip(t), "", ""), "/sub/")
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	// the parent link is the first row of a listing below the top
	if got, want := strings.Join(listedNames(body), " "), "../ b.txt"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}
}

func TestZipFSSortsFolder(t *testing.T) {

	handler := ZipFSHandler(testZip(t), "", "")

	rec := get(handler, "/?sort=name&order=desc")
	if got, want := strings.Join(listedNames(rec.Body.String()), " "), "sub/ empty/ a.txt"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}
}

func TestZipFSServesEntry(t *testing.T) {

	rec := get(ZipFSHandler(testZip(t), "", ""), "/sub/b.txt")
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), "knut knut"; got != want {
		t.Errorf("got body %q, want %q", got, want)
	}
}
