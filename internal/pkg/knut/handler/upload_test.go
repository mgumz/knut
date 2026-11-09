// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
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
