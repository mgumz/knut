// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/tar"
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut"
)

// postFiles sends "files" - name to content - as one multipart POST, the
// way the form without a script does
func postFiles(t *testing.T, h http.Handler, accept string, files ...[2]string) *httptest.ResponseRecorder {
	t.Helper()

	body := bytes.NewBuffer(nil)
	mw := multipart.NewWriter(body)
	for _, file := range files {
		part, err := mw.CreateFormFile("upload_file", file[0])
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte(file[1]))
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", accept)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// every file of one POST is stored, and the done page lists each
func TestUploadStoresEveryFile(t *testing.T) {

	dir := t.TempDir()
	rec := postFiles(t, UploadHandler(dir), "text/html", [2]string{"a.txt", "aaaa"}, [2]string{"IMG_0001.jpeg", "jj"})

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	entries, _ := os.ReadDir(dir)
	var stored []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
		stored = append(stored, string(content))
	}
	sort.Strings(stored)
	if strings.Join(stored, ",") != "aaaa,jj" {
		t.Errorf("stored %q, want both files", stored)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"ok, received 6 B in",
		`<span class="name">a.txt</span> <span class="meta">4 B</span>`,
		`<span class="name">IMG_0001.jpeg</span> <span class="meta">2 B</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the done page does not render %q", want)
		}
	}
}

// a file which could not be stored is named, and fails the answer
func TestUploadReportsWhatWasNotStored(t *testing.T) {

	dir := filepath.Join(t.TempDir(), "gone")
	h := UploadHandler(dir)
	os.Remove(dir)

	rec := postFiles(t, h, "*/*", [2]string{"a.txt", "aaaa"})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"1 of 1 not stored", "failed  a.txt"} {
		if !strings.Contains(body, want) {
			t.Errorf("the text answer %q does not carry %q", body, want)
		}
	}
	if strings.Contains(body, dir) {
		t.Error("the answer carries the path of the folder")
	}
}

// a file is written below the hidden folder and only shows up next to it
// once it is complete: nothing is left below it afterwards
func TestUploadMovesCompleteFilesOutOfPartial(t *testing.T) {

	dir := t.TempDir()
	if rec := postFiles(t, UploadHandler(dir), "*/*", [2]string{"a.txt", "aaaa"}); rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	left, err := os.ReadDir(filepath.Join(dir, knut.UploadPartialDir))
	if err != nil || len(left) != 0 {
		t.Errorf("below the partial folder: %v, %v - want it there and empty", left, err)
	}

	stored, _ := filepath.Glob(filepath.Join(dir, "*_a.txt_*"))
	if len(stored) != 1 {
		t.Errorf("stored %q, want one a.txt next to the partial folder", stored)
	}
}

// a body which breaks off leaves nothing behind, neither the file it was
// in the middle of nor a part of it
func TestUploadBrokenBodyLeavesNothing(t *testing.T) {

	dir := t.TempDir()
	h := UploadHandler(dir)

	body := bytes.NewBuffer(nil)
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("upload_file", "a.txt")
	part.Write(bytes.Repeat([]byte("a"), 1<<16))
	// no mw.Close(): the closing boundary never comes

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Errorf("a broken body got %d", rec.Code)
	}

	var found []string
	filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			found = append(found, path)
		}
		return nil
	})
	if len(found) != 0 {
		t.Errorf("a broken upload left %q", found)
	}
}

// the partial folder of an upload folder is never published: not by its
// listing, not as files, not in the zip of the folder, not in a tar or zip
func TestPartialIsNotPublished(t *testing.T) {

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "done.txt"), []byte("done"), 0644)
	os.Mkdir(filepath.Join(dir, knut.UploadPartialDir), 0700)
	os.WriteFile(filepath.Join(dir, knut.UploadPartialDir, "half.txt"), []byte("ha"), 0644)

	listing := FileOrDirHandler(dir, "/")

	if body := get(listing, "/").Body.String(); strings.Contains(body, knut.UploadPartialDir) || !strings.Contains(body, "done.txt") {
		t.Error("the listing shows the partial folder, or not the files next to it")
	}
	for _, uri := range []string{"/" + knut.UploadPartialDir + "/", "/" + knut.UploadPartialDir + "/half.txt"} {
		if code := get(listing, uri).Code; code != http.StatusNotFound {
			t.Errorf("%s got %d, want 404", uri, code)
		}
	}
	if names := zipNames(zipOf(t, get(listing, "/?zip").Body.Bytes())); strings.Contains(strings.Join(names, ","), "half.txt") {
		t.Errorf("the zip of the folder carries %q", names)
	}

	if names := zipNames(zipOf(t, get(ZipHandler(dir, "", false), "/").Body.Bytes())); strings.Contains(strings.Join(names, ","), "half.txt") {
		t.Errorf("zip:// carries %q", names)
	}

	tr := tar.NewReader(get(TarHandler(dir, ""), "/").Body)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(header.Name, knut.UploadPartialDir) {
			t.Errorf("tar:// carries %q", header.Name)
		}
	}
}

// what a stopped knut left below the partial folder is swept on start,
// what is still being written to is not
func TestUploadSweepsStalePartials(t *testing.T) {

	dir := t.TempDir()
	partial := filepath.Join(dir, knut.UploadPartialDir)
	os.Mkdir(partial, 0700)
	stale, fresh := filepath.Join(partial, "stale"), filepath.Join(partial, "fresh")
	os.WriteFile(stale, nil, 0600)
	os.WriteFile(fresh, nil, 0600)
	old := time.Now().Add(-2 * partialMaxAge)
	os.Chtimes(stale, old, old)

	UploadHandler(dir)

	if _, err := os.Stat(stale); err == nil {
		t.Error("a stale partial survived the start")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("a fresh partial was swept")
	}
}
