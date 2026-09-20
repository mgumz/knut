// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"
)

// the url a code points at is the one the client asked for, spelled out:
// whoever scans it is on another device and shares nothing with the
// browser but the network.
func TestRequestURL(t *testing.T) {

	tests := []struct {
		name      string
		host      string
		target    string
		forwarded string
		tls       bool
		want      string
	}{
		{name: "host and path", host: "192.168.1.5:8080", target: "/pics/",
			want: "http://192.168.1.5:8080/pics/"},
		{name: "the escaping of the path survives", host: "knut.box", target: "/a%20b/",
			want: "http://knut.box/a%20b/"},
		{name: "the query is no part of it", host: "knut.box", target: "/pics/?sort=size",
			want: "http://knut.box/pics/"},
		{name: "tls", host: "knut.box", target: "/", tls: true,
			want: "https://knut.box/"},
		{name: "a proxy which terminated tls", host: "knut.box", target: "/", forwarded: "https",
			want: "https://knut.box/"},
		{name: "a chain of proxies", host: "knut.box", target: "/", forwarded: "https, http",
			want: "https://knut.box/"},
		{name: "a scheme knut does not serve", host: "knut.box", target: "/", forwarded: "javascript:alert(1)",
			want: "http://knut.box/"},
		{name: "loopback by name", host: "localhost:8080", target: "/"},
		{name: "loopback by address", host: "127.0.0.1:8080", target: "/"},
		{name: "loopback without a port", host: "127.0.0.1", target: "/"},
		{name: "loopback over ipv6", host: "[::1]:8080", target: "/"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			r := httptest.NewRequest("GET", test.target, nil)
			r.Host = test.host
			if test.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if test.forwarded != "" {
				r.Header.Set("X-Forwarded-Proto", test.forwarded)
			}

			if got := requestURL(r); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// the code goes into the markup as a data uri, so what comes back has to
// be a png a browser can draw
func TestQRCode(t *testing.T) {

	const prefix = "data:image/png;base64,"

	code := string(qrCode("http://192.168.1.5:8080/pics/"))
	if !strings.HasPrefix(code, prefix) {
		t.Fatalf("the code is no data uri: %.40q", code)
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(code, prefix))
	if err != nil {
		t.Fatalf("decoding the data uri: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decoding the png: %v", err)
	}
	if width := img.Bounds().Dx(); width != qrPixels {
		t.Errorf("the code is %d pixels wide, want %d", width, qrPixels)
	}
}

// what cannot be drawn comes back empty, and the markup then writes no
// <img> at all: a browser resolves an empty "src" against the page it is
// on and fetches that, which is a request knut would have to answer and a
// broken image either way
func TestQRCodeNothingToDraw(t *testing.T) {

	tests := map[string]string{
		"a page which does not know its uri": "",
		"more than a code holds":             "http://knut.box/" + strings.Repeat("x", 4096),
	}

	for name, url := range tests {
		if code := qrCode(url); code != "" {
			t.Errorf("%s rendered %.40q", name, code)
		}
	}
}

// the header is where the code hangs, and only while there is one
func TestLayoutDrawsTheCode(t *testing.T) {

	r := httptest.NewRequest("GET", "/pics/", nil)
	r.Host = "knut.box"

	body, err := Render(Template("index"), struct {
		Page
		Windows []string
	}{Page: PageFor(r, "")})
	if err != nil {
		t.Fatalf("rendering the page: %v", err)
	}

	if page := string(body); !strings.Contains(page, `<img class="qr" src="data:image/png;base64,`) ||
		!strings.Contains(page, `alt="http://knut.box/pics/"`) {
		t.Error("the header of a page which knows its uri draws no code")
	}

	r.Host = "localhost:8080"
	body, err = Render(Template("index"), struct {
		Page
		Windows []string
	}{Page: PageFor(r, "")})
	if err != nil {
		t.Fatalf("rendering the page: %v", err)
	}

	if strings.Contains(string(body), "<img") {
		t.Error("a page served to the machine it runs on draws a code nobody can scan")
	}
}

// a page framed for a request knows the uri it was asked for, both for
// the code in the header and for the knock which follows a lost
// connection. one framed without a request knows neither.
func TestPageForCarriesItsURI(t *testing.T) {

	r := httptest.NewRequest("GET", "/pics/?sort=size", nil)
	r.Host = "knut.box"

	page := PageFor(r, "/pics/")
	if page.Path != "/pics/" {
		t.Errorf("the page knocks on %q, want %q", page.Path, "/pics/")
	}
	if page.URL != "http://knut.box/pics/" {
		t.Errorf("the page points at %q", page.URL)
	}

	if page := NewPage(""); page.Path != "" || page.URL != "" {
		t.Errorf("a page without a request carries %q and %q", page.Path, page.URL)
	}
}
