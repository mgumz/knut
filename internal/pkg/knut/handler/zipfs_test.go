// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// zipItem is one entry of a zip written for a test.
type zipItem struct{ name, content string }

// zipBytes builds a zip carrying "entries" in memory. the timestamps are
// fixed: a listing ordered by date has exactly one right answer that way.
func zipBytes(t *testing.T, entries ...zipItem) []byte {
	t.Helper()

	buf := bytes.NewBuffer(nil)
	zw := zip.NewWriter(buf)

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
		t.Fatalf("closing the zip: %v", err)
	}

	return buf.Bytes()
}

// writeZip puts a zip at "name", replacing whatever is at that path.
func writeZip(t *testing.T, name string, entries ...zipItem) {
	t.Helper()

	if err := os.WriteFile(name, zipBytes(t, entries...), 0600); err != nil {
		t.Fatalf("writing %q: %v", name, err)
	}
}

// testZip writes a zip carrying a file, a folder with an entry of its own
// and an explicit folder entry.
func testZip(t *testing.T) string {
	t.Helper()

	name := filepath.Join(t.TempDir(), "tree.zip")
	writeZip(t, name,
		zipItem{"a.txt", "knut"},
		zipItem{"sub/b.txt", "knut knut"},
		zipItem{"empty/", ""})

	return name
}

// zipPace shortens how long the reader trusts a stat, the way livePace
// shortens the poll.
func zipPace(t *testing.T, reread time.Duration) {
	t.Helper()
	old := zipReread
	zipReread = reread
	t.Cleanup(func() { zipReread = old })
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

// a zip which is not there is the server's problem, not the client's
func TestZipFSListsMissingZip(t *testing.T) {

	rec := get(ZipFSHandler(filepath.Join(t.TempDir(), "gone.zip"), "", ""), "/")
	if got, want := rec.Code, http.StatusInternalServerError; got != want {
		t.Errorf("got status %d, want %d", got, want)
	}
}

// re-listing means parsing the whole central directory: within zipReread,
// a zip whose stat did not move is answered from the last read
func TestZipFolderReaderReadsOnlyWhenTheZipMoved(t *testing.T) {

	mod := modTime(15)
	fsys := fstest.MapFS{
		"tree.zip": {Data: zipBytes(t, zipItem{"a.txt", "knut"}), ModTime: mod},
	}
	read := zipFolderReader(fsys, "tree.zip", "")

	first, state, err := read()
	if err != nil {
		t.Fatalf("reading the zip: %v", err)
	}

	again, sameState, err := read()
	if err != nil {
		t.Fatalf("reading the zip again: %v", err)
	}
	if sameState != state {
		t.Errorf("an untouched zip changed state from %q to %q", state, sameState)
	}
	if &again[0] != &first[0] {
		t.Error("an untouched zip was parsed a second time")
	}

	// replaced by a zip of another size, under another timestamp: what a
	// stat is there to catch
	fsys["tree.zip"] = &fstest.MapFile{
		Data:    zipBytes(t, zipItem{"a.txt", "knut"}, zipItem{"c.txt", "new"}),
		ModTime: mod.Add(time.Minute),
	}

	moved, movedState, err := read()
	if err != nil {
		t.Fatalf("reading the replaced zip: %v", err)
	}
	if movedState == state {
		t.Error("a replaced zip was answered from the last read")
	}
	if got := len(moved); got != 2 {
		t.Errorf("got %d entries, want the 2 of the replaced zip", got)
	}
}

// a stat is no evidence: nfs answers one from its attribute cache, a
// coarse filesystem rounds the timestamp away. so the gate may postpone a
// read, never cancel it - every zipReread the zip is opened whatever the
// stat claims.
//
// a stat which lies is the reason this runs on a MapFS: no filesystem
// hands out unchanged metadata for changed bytes on request.
func TestZipFolderReaderReadsThroughAStatWhichLies(t *testing.T) {

	zipPace(t, 20*time.Millisecond)

	mod := modTime(15)
	one := zipBytes(t, zipItem{"a.txt", "knut"})
	two := zipBytes(t, zipItem{"a.txt", "KNUT"})
	if len(one) != len(two) {
		t.Fatalf("the two zips are %d and %d bytes: the stat would not be lying", len(one), len(two))
	}

	fsys := fstest.MapFS{"tree.zip": {Data: one, ModTime: mod}}
	read := zipFolderReader(fsys, "tree.zip", "")

	_, state, err := read()
	if err != nil {
		t.Fatalf("reading the zip: %v", err)
	}

	// other content, same size, same timestamp
	fsys["tree.zip"] = &fstest.MapFile{Data: two, ModTime: mod}

	if _, gated, _ := read(); gated != state {
		t.Error("the zip was parsed again although its stat had not moved")
	}

	time.Sleep(2 * zipReread)

	if _, fresh, _ := read(); fresh == state {
		t.Error("a zip replaced behind a lying stat was never picked up")
	}
}

// the crc folded into the state is what tells two zips apart whose entries
// kept their names, their sizes and their timestamps
func TestZipFSStateSeesReplacedContent(t *testing.T) {

	states := make([]string, 2)
	for i, content := range []string{"knut", "KNUT"} {

		data := zipBytes(t, zipItem{"a.txt", content})
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("reading the zip: %v", err)
		}

		states[i] = view.LiveState(listFolderEntries(z, ""))
	}

	if states[0] == states[1] {
		t.Error("a zip whose entry kept its name, size and timestamp is reported as unchanged")
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
