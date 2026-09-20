// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/mgumz/knut/internal/pkg/knut"
)

// get asks for "target" the way a browser would.
func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// ask sends what the asset handler branches on: a method and the two
// headers which decide what comes back.
func ask(h http.Handler, method, target string, header http.Header) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, nil)
	for name, values := range header {
		req.Header[name] = values
	}
	h.ServeHTTP(rec, req)
	return rec
}

// assetHandler is the handler with a sentinel behind it: whatever reaches
// "next" is a request the handler decided was not its own.
func assetHandler() (http.Handler, *bool) {

	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusTeapot)
	})

	return AssetHandler(next), &reached
}

// knut maps arbitrary uris, so the handler takes exactly the one it
// reserved and not a byte more - the rest belongs to the mapping behind
// it.
func TestAssetHandlerPassesEverythingElseOn(t *testing.T) {

	for _, target := range []string{
		"/",
		"/index.html",
		"/.knut/",
		"/.knut/htmx.js/still-not-it",
		"/.knut/htmx.json",
		"/HTMX.js",
	} {
		h, reached := assetHandler()
		rec := get(h, target)

		if !*reached {
			t.Errorf("%q: the handler answered a uri it does not own", target)
		}
		if rec.Code != http.StatusTeapot {
			t.Errorf("%q: got %d, want the mapping behind the handler to answer", target, rec.Code)
		}
	}
}

// the asset is embedded gzipped because that is the shape it goes out in:
// a client taking gzip gets the stored bytes, untouched.
func TestAssetHandlerServesTheStoredBytes(t *testing.T) {

	h, reached := assetHandler()
	rec := ask(h, http.MethodGet, knut.LiveAssetURI, http.Header{
		"Accept-Encoding": {"gzip"},
	})

	if *reached {
		t.Fatal("the reserved uri was handed to the mapping behind it")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	body := rec.Body.Bytes()
	if len(body) != htmxSizeGz {
		t.Errorf("got %d bytes, want the stored %d", len(body), htmxSizeGz)
	}
	if string(body) != string(htmxGz) {
		t.Error("the body is not the embedded asset - it was re-encoded on the way out")
	}

	header := rec.Result().Header
	for name, want := range map[string]string{
		"Content-Encoding": "gzip",
		"Content-Type":     "text/javascript; charset=utf-8",
		"Cache-Control":    "public, max-age=86400",
		"Vary":             "Accept-Encoding",
		"Content-Length":   strconv.Itoa(htmxSizeGz),
	} {
		if got := header.Get(name); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

// the rare client which does not take gzip gets the asset unpacked, and
// what it gets is the release which was pinned: the sha256 is the one
// vendor_htmx.go wrote down when it fetched it.
func TestAssetHandlerUnpacksForClientsWithoutGzip(t *testing.T) {

	for _, accept := range []string{"", "deflate", "br, zstd"} {

		h, _ := assetHandler()
		rec := ask(h, http.MethodGet, knut.LiveAssetURI, http.Header{
			"Accept-Encoding": {accept},
		})

		if rec.Code != http.StatusOK {
			t.Fatalf("accept-encoding %q: got %d, want 200", accept, rec.Code)
		}

		body := rec.Body.Bytes()
		if len(body) != htmxSizeRaw {
			t.Errorf("accept-encoding %q: got %d bytes, want the unpacked %d",
				accept, len(body), htmxSizeRaw)
		}

		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); got != htmxSHA256 {
			t.Errorf("accept-encoding %q: the unpacked asset is not the pinned release:\n got %s\nwant %s",
				accept, got, htmxSHA256)
		}

		header := rec.Result().Header
		if got := header.Get("Content-Encoding"); got != "" {
			t.Errorf("accept-encoding %q: the body went out as %q to a client which does not take it",
				accept, got)
		}
		if got := header.Get("Content-Length"); got != strconv.Itoa(htmxSizeRaw) {
			t.Errorf("accept-encoding %q: content-length %q does not describe the %d bytes sent",
				accept, got, len(body))
		}
	}
}

// the asset only changes with the binary, so a client which already has it
// is told so instead of being sent 16 kb again.
func TestAssetHandlerAnswers304ForTheEtagItHandedOut(t *testing.T) {

	h, _ := assetHandler()
	etag := get(h, knut.LiveAssetURI).Result().Header.Get("Etag")

	if etag == "" {
		t.Fatal("no etag was handed out, so no client can ever revalidate")
	}
	if etag[0] != '"' || etag[len(etag)-1] != '"' {
		t.Errorf("etag %s is not quoted, which no cache is required to honour", etag)
	}

	rec := ask(h, http.MethodGet, knut.LiveAssetURI, http.Header{
		"If-None-Match": {etag},
	})
	if rec.Code != http.StatusNotModified {
		t.Errorf("got %d for the etag we handed out, want 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a 304 carried %d bytes of body", rec.Body.Len())
	}
	if got := rec.Result().Header.Get("Etag"); got != etag {
		t.Errorf("the 304 dropped the etag: got %q, want %q", got, etag)
	}

	// an etag from another build must not be taken for this one
	rec = ask(h, http.MethodGet, knut.LiveAssetURI, http.Header{
		"If-None-Match": {`"htmx-0.0.1"`},
	})
	if rec.Code != http.StatusOK {
		t.Errorf("got %d for a stale etag, want the asset itself", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("a stale etag was answered without the asset")
	}
}

// the asset is a file: it is read, and nothing else.
func TestAssetHandlerTakesOnlyGetAndHead(t *testing.T) {

	// net/http drops the body of a HEAD on the way out, the recorder does
	// not - so the headers are what this asserts on
	h, _ := assetHandler()
	rec := ask(h, http.MethodHead, knut.LiveAssetURI, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("HEAD: got %d, want 200", rec.Code)
	}
	if got := rec.Result().Header.Get("Etag"); got == "" {
		t.Error("HEAD: no etag, so a conditional GET cannot follow")
	}

	for _, method := range []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	} {
		h, reached := assetHandler()
		rec := ask(h, method, knut.LiveAssetURI, nil)

		if *reached {
			t.Errorf("%s: fell through to the mapping behind the reserved uri", method)
		}
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: got %d, want 405", method, rec.Code)
		}
	}
}

// accept-encoding is a list with parameters, not a word: "gzip" hides in
// it and "gzipped" only looks like it does.
func TestAcceptsGzip(t *testing.T) {

	tests := []struct {
		header string
		want   bool
	}{
		{"gzip", true},
		{"gzip, deflate", true},
		{"deflate, gzip", true},
		{"  gzip  ", true},
		{"deflate, gzip;q=0.5", true},
		{"gzip;q=1.0, identity;q=0.5", true},
		{"", false},
		{"identity", false},
		{"deflate", false},
		{"br, zstd", false},
		{"gzipped", false},
		{"x-gzip", false},
		{"notgzip, deflate", false},
	}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, knut.LiveAssetURI, nil)
		if test.header != "" {
			req.Header.Set("Accept-Encoding", test.header)
		}
		if got := acceptsGzip(req); got != test.want {
			t.Errorf("acceptsGzip(%q): got %v, want %v", test.header, got, test.want)
		}
	}
}
