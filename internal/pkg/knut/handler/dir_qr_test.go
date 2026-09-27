// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// getFrom asks "target" the way a browser pointed at "host" would - the
// code of an entry is the uri under that host, and a listing offers one
// only where there is a host to hand another device.
func getFrom(h http.Handler, target, host string) *httptest.ResponseRecorder {

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = host

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// every row offers its code, the file as well as the folder
func TestDirListQRLinks(t *testing.T) {

	body := get(DirListHandler(testFS()), "/").Body.String()

	for _, want := range []string{
		// the link is the entry, the code of it is drawn from that uri
		`<a class="qr-link" href="a.txt"`,
		`<a class="qr-link" href="sub/"`,
		`<dialog id="qr-modal"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the listing does not offer %q", want)
		}
	}
}

// the folder on screen is offered where the column names would be: the
// zip of it and the code of it, for the reader who is already in it
func TestDirListHeaderOffersTheFolder(t *testing.T) {

	body := get(DirListHandler(testFS()), "/sub/").Body.String()

	for _, want := range []string{
		`<th class="actions">`,
		`<a class="zip" href="?zip"`,
		`<a class="qr-link" href="./"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the header of the listing does not offer %q", want)
		}
	}
}

// a listing of a zip offers the codes, the file as well as the folder
func TestZipFSListingOffersQR(t *testing.T) {

	body := get(ZipFSHandler(testZip(t), "", ""), "/").Body.String()

	for _, want := range []string{
		`<a class="qr-link" href="./"`,
		`<a class="qr-link" href="a.txt"`,
		`<a class="qr-link" href="sub/"`,
		`<dialog id="qr-modal"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("a listing of a zip does not offer %q", want)
		}
	}
}

// the code of an entry inside a zip is a png, same as on disk - with and
// without a prefix narrowing what is published
func TestZipFSQRImage(t *testing.T) {

	for _, tc := range []struct {
		prefix string
		target string
	}{
		{"", "/?qr"},
		{"", "/a.txt?qr"},
		{"", "/sub/?qr"},
		{"", "/sub/b.txt?qr"},
		{"", "/empty/?qr"},
		{"sub", "/?qr"},
		{"sub", "/b.txt?qr"},
	} {
		rec := get(ZipFSHandler(testZip(t), tc.prefix, ""), tc.target)

		if rec.Code != http.StatusOK {
			t.Errorf("%q%q: got status %d, want %d", tc.prefix, tc.target, rec.Code, http.StatusOK)
			continue
		}
		if got, want := rec.Header().Get("Content-Type"), "image/png"; got != want {
			t.Errorf("%q%q: got content type %q, want %q", tc.prefix, tc.target, got, want)
		}
		if _, err := png.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil {
			t.Errorf("%q%q: decoding the code: %v", tc.prefix, tc.target, err)
		}
	}
}

// an entry the zip does not carry has no code - nor one outside the
// prefix, or a name only sharing the start of an entry
func TestZipFSQRImageMissing(t *testing.T) {

	for _, tc := range []struct {
		prefix string
		target string
	}{
		{"", "/nope.txt?qr"},
		{"", "/su?qr"},
		{"sub", "/a.txt?qr"},
	} {
		rec := get(ZipFSHandler(testZip(t), tc.prefix, ""), tc.target)

		if rec.Code != http.StatusNotFound {
			t.Errorf("%q%q: got status %d, want %d", tc.prefix, tc.target, rec.Code, http.StatusNotFound)
		}
	}
}

// a zip read on the machine knut runs on has no code to hand anybody
func TestZipFSQRImageOnLoopback(t *testing.T) {

	rec := getFrom(ZipFSHandler(testZip(t), "", ""), "/a.txt?qr", "localhost:8080")

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// a listing read on the machine knut runs on has no code to hand anybody:
// the url of it names a host no other device can reach
func TestDirListNoQRLinksOnLoopback(t *testing.T) {

	body := getFrom(DirListHandler(testFS()), "/", "127.0.0.1:8080").Body.String()

	for _, unwanted := range []string{`class="qr-link"`, `<dialog id="qr-modal"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("a listing on a loopback host renders %q", unwanted)
		}
	}
	// the column stays, the listing is the same table either way
	if !strings.Contains(body, `<td class="actions">`) {
		t.Error("the listing lost its column of links")
	}
}

// the code of an entry is a png of its own, asked for when it is shown
func TestDirQRImage(t *testing.T) {

	handler := DirListHandler(testFS())

	for _, target := range []string{"/a.txt?qr", "/sub/?qr", "/?qr"} {

		rec := get(handler, target)

		if rec.Code != http.StatusOK {
			t.Errorf("%q: got status %d, want %d", target, rec.Code, http.StatusOK)
			continue
		}
		if got, want := rec.Header().Get("Content-Type"), "image/png"; got != want {
			t.Errorf("%q: got content type %q, want %q", target, got, want)
		}

		image, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Errorf("%q: decoding the code: %v", target, err)
			continue
		}
		if bounds := image.Bounds(); bounds.Dx() != bounds.Dy() || bounds.Dx() < 256 {
			t.Errorf("%q: got a %dx%d code, want a square one to scan",
				target, bounds.Dx(), bounds.Dy())
		}
	}
}

// the code is of the entry it hangs on, so two entries carry two codes
func TestDirQRImageIsOfTheEntry(t *testing.T) {

	handler := DirListHandler(testFS())

	one := get(handler, "/a.txt?qr").Body.Bytes()
	two := get(handler, "/b.txt?qr").Body.Bytes()
	again := get(handler, "/a.txt?qr").Body.Bytes()

	if bytes.Equal(one, two) {
		t.Error("two entries are handed the same code")
	}
	if !bytes.Equal(one, again) {
		t.Error("one entry is handed two codes")
	}
}

// nothing to point a device at, nothing to draw
func TestDirQRImageOnLoopback(t *testing.T) {

	rec := getFrom(DirListHandler(testFS()), "/a.txt?qr", "localhost:8080")

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// an entry which is not there has no code either
func TestDirQRImageMissing(t *testing.T) {

	rec := get(DirListHandler(testFS()), "/nope.txt?qr")

	if rec.Code != http.StatusNotFound {
		t.Errorf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// the zip of a folder is reached through knut, so it needs no code of its
// own: the code of the folder is the one worth scanning
func TestDirQRNotOfferedForTheZip(t *testing.T) {

	body := get(DirListHandler(testFS()), "/").Body.String()

	if strings.Contains(body, "?zip&amp;qr") || strings.Contains(body, "?zip&qr") {
		t.Error("the zip of a folder is offered as a code")
	}
}

// the code in the header opens larger on every page which has one, not
// only on a listing - and a listing carries the dialog once, not twice
func TestHeaderQRDialogOnEveryPage(t *testing.T) {

	gitSetup(t)

	pages := map[string]string{
		"listing": get(DirListHandler(testFS()), "/").Body.String(),
		"zip":     get(ZipFSHandler(testZip(t), "", ""), "/").Body.String(),
		"upload":  get(UploadHandler(t.TempDir()), "/upload").Body.String(),
		"git":     get(GitHandler(gitBin, testRepos(t), "/"), "/one/").Body.String(),
		"repos":   get(GitHandler(gitBin, testRepos(t), "/"), "/").Body.String(),
	}

	for page, body := range pages {
		if got := strings.Count(body, `<dialog id="qr-modal"`); got != 1 {
			t.Errorf("the %s page carries %d code dialogs, want 1", page, got)
		}
		if !strings.Contains(body, `querySelector("header img.qr")`) {
			t.Errorf("the %s page does not carry the script opening its code", page)
		}
	}
}

// a page with no code in its header has nothing to open
func TestHeaderQRDialogNotOnLoopback(t *testing.T) {

	body := getFrom(DirListHandler(testFS()), "/", "localhost:8080").Body.String()

	if strings.Contains(body, `id="qr-modal"`) || strings.Contains(body, `header img.qr`) {
		t.Error("a page without a code carries its dialog")
	}
}
