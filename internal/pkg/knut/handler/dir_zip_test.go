// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// zipOf reads the response as an archive and hands back what is in it.
func zipOf(t *testing.T, body []byte) *zip.Reader {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("reading the response as a zip: %v", err)
	}

	return reader
}

// zipNames are the entries of an archive, ordered by name.
func zipNames(reader *zip.Reader) []string {
	names := []string{}
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	sort.Strings(names)
	return names
}

// a listed folder carries a link to its own download, a file does not
func TestDirListZipLinks(t *testing.T) {

	body := get(DirListHandler(testFS()), "/").Body.String()

	if want := `<a class="zip" href="sub/?zip"`; !strings.Contains(body, want) {
		t.Errorf("the listing does not offer %q", want)
	}
	if strings.Contains(body, `href="a.txt?zip"`) {
		t.Error("a file must not be offered as a zip")
	}

	// the download sits in the last column, the size column says
	// what it always said about a folder
	if !strings.Contains(body, `<td class="actions">`) {
		t.Error("the listing has no column of links")
	}
	if !strings.Contains(body, `<td class="size">-</td>`) {
		t.Error("a folder renders no size")
	}
}

func TestDirZipStreamsFolder(t *testing.T) {

	rec := get(DirListHandler(testFS()), "/sub/?zip")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/zip"; got != want {
		t.Errorf("got content type %q, want %q", got, want)
	}
	// the archive is written while it is sent, so its length is unknown
	// when the headers go out
	if got := rec.Header().Get("Content-Length"); got != "" {
		t.Errorf("got content length %q, want none", got)
	}

	reader := zipOf(t, rec.Body.Bytes())
	if got, want := strings.Join(zipNames(reader), " "), "sub/deep.txt"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}
}

// the download is named after the folder and the moment it was taken
func TestDirZipFilename(t *testing.T) {

	stamp := time.Now().Format(zipStamp)

	tests := []struct {
		target string
		want   string
	}{
		{"/sub/?zip", "sub-" + stamp + ".zip"},
		// the root of a mapping published at "/" has no folder name in
		// its uri to be named after
		{"/?zip", zipRootName + "-" + stamp + ".zip"},
	}

	for _, test := range tests {
		rec := get(DirListHandler(testFS()), test.target)

		disposition := rec.Header().Get("Content-Disposition")
		kind, params, err := mime.ParseMediaType(disposition)
		if err != nil {
			t.Errorf("%q: parsing %q: %v", test.target, disposition, err)
			continue
		}
		if kind != "attachment" {
			t.Errorf("%q: got %q, want an attachment", test.target, kind)
		}
		if got := params["filename"]; got != test.want {
			t.Errorf("%q: got filename %q, want %q", test.target, got, test.want)
		}
	}
}

// the uri a mapping is published under names the zip of its root
func TestDirZipFilenameBehindPrefix(t *testing.T) {

	handler := http.StripPrefix("/pub/", DirListHandler(testFS()))

	rec := get(handler, "/pub/?zip")
	_, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
	if err != nil {
		t.Fatalf("parsing the disposition: %v", err)
	}

	if want := "pub-" + time.Now().Format(zipStamp) + ".zip"; params["filename"] != want {
		t.Errorf("got filename %q, want %q", params["filename"], want)
	}
}

// the whole tree below the folder travels, empty folders included, the
// files deflated and the folders stored
func TestDirZipWholeTree(t *testing.T) {

	fsys := http.FS(fstest.MapFS{
		"a.txt":            {Data: []byte("knut")},
		"deep":             {Mode: fs.ModeDir},
		"deep/b.txt":       {Data: []byte("tree")},
		"deep/deeper":      {Mode: fs.ModeDir},
		"deep/deeper/c.md": {Data: []byte("#")},
		"empty":            {Mode: fs.ModeDir},
	})

	rec := get(DirListHandler(fsys), "/?zip")
	reader := zipOf(t, rec.Body.Bytes())

	want := "knut/a.txt knut/deep/ knut/deep/b.txt knut/deep/deeper/ knut/deep/deeper/c.md knut/empty/"
	if got := strings.Join(zipNames(reader), " "); got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}

	for _, file := range reader.File {
		want := zip.Deflate
		if strings.HasSuffix(file.Name, "/") {
			want = zip.Store
		}
		if file.Method != want {
			t.Errorf("%q is written with method %d, want %d", file.Name, file.Method, want)
		}
	}

	content, err := reader.Open("knut/deep/b.txt")
	if err != nil {
		t.Fatalf("opening an entry of the archive: %v", err)
	}
	defer content.Close()

	if got, _ := io.ReadAll(content); string(got) != "tree" {
		t.Errorf("got content %q, want %q", got, "tree")
	}
}

// what packs, packs: an entry travels smaller than it sits on disk and
// still arrives byte for byte
func TestDirZipDeflatesContent(t *testing.T) {

	text := bytes.Repeat([]byte("knut serves a folder. "), 512)
	fsys := http.FS(fstest.MapFS{"a.txt": {Data: text}})

	rec := get(DirListHandler(fsys), "/?zip")
	reader := zipOf(t, rec.Body.Bytes())

	entry := reader.File[0]
	if entry.CompressedSize64 >= entry.UncompressedSize64 {
		t.Errorf("%q travels %d bytes of %d, want fewer",
			entry.Name, entry.CompressedSize64, entry.UncompressedSize64)
	}

	content, err := reader.Open(entry.Name)
	if err != nil {
		t.Fatalf("opening an entry of the archive: %v", err)
	}
	defer content.Close()

	if got, _ := io.ReadAll(content); !bytes.Equal(got, text) {
		t.Errorf("got %d bytes back, want the %d which went in", len(got), len(text))
	}
}

// a symlink has no content to store and one pointing at its own parent
// would make the walk endless
func TestDirZipSkipsSymlinks(t *testing.T) {

	root := testTree(t)
	if err := os.Symlink(root, filepath.Join(root, "sub", "loop")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}

	rec := get(DirListHandler(http.Dir(root)), "/sub/?zip")

	if got := strings.Join(zipNames(zipOf(t, rec.Body.Bytes())), " "); got != "sub/deep.txt" {
		t.Errorf("got entries %q, want %q", got, "sub/deep.txt")
	}
}

// a folder asked for without its "/" redirects, "?zip" and all
func TestDirZipRedirectsToFolder(t *testing.T) {

	rec := get(DirListHandler(http.Dir(testTree(t))), "/sub?zip")

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusMovedPermanently)
	}
	if got, want := rec.Header().Get("Location"), "sub/?zip"; got != want {
		t.Errorf("got location %q, want %q", got, want)
	}
}

// an index.html shadows the listing of a folder, not the download of it
func TestDirZipPastIndexPage(t *testing.T) {

	root := testTree(t)
	if err := os.WriteFile(filepath.Join(root, indexPage), []byte("hand written"), 0600); err != nil {
		t.Fatalf("writing %q: %v", indexPage, err)
	}

	rec := get(DirListHandler(http.Dir(root)), "/?zip")

	if got := zipNames(zipOf(t, rec.Body.Bytes())); len(got) == 0 {
		t.Error("the folder was answered with the index page, not with a zip")
	}
}

// a folder inside a zip is listed like one on disk
func TestZipFSListingFolderSize(t *testing.T) {

	body := get(ZipFSHandler(testZip(t), "", ""), "/").Body.String()

	if !strings.Contains(body, `<td class="size">-</td>`) {
		t.Error("a folder inside a zip renders no size")
	}
}

var zipEntryHref = regexp.MustCompile(`href="([^"]*\?zip)"`)

// a listing offers the folder it shows and every folder in it, both
// relative to the page so they survive the prefix of a mapping
func TestDirZipLinks(t *testing.T) {

	body := get(DirListHandler(testFS()), "/?sort=name").Body.String()

	links := []string{}
	for _, match := range zipEntryHref.FindAllStringSubmatch(body, -1) {
		links = append(links, match[1])
	}

	// the folder on screen first, it hangs in the header of the column
	if got, want := strings.Join(links, " "), "?zip sub/?zip"; got != want {
		t.Errorf("got zip links %q, want %q", got, want)
	}
}
