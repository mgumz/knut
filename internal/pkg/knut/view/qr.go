// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"encoding/base64"
	"html/template"
	"net"
	"net/http"
	"strings"

	_ "embed"

	qrcode "github.com/skip2/go-qrcode"
)

// qrJS opens a code in a dialog on top of the page: the one in the header,
// and the one of a row of a listing. it is inlined like the stylesheet, and
// it is an addition: without it the code in the header stays small, and
// the link of a row is followed and shows the code on a page of its own.
//
//go:embed assets/qr.js
var qrJS string

// qrScript is qrJS as the markup takes it.
func qrScript() template.JS {
	return template.JS(qrJS)
}

// qrPixels is how wide the png is drawn. it is scaled down by the markup,
// which keeps the code sharp on a display which packs more than one pixel
// per css pixel - and costs well under a kilobyte either way.
const qrPixels = 256

// qrLargePixels is how wide a code meant to be scanned off the page
// itself is drawn - the one a listing shows for a single entry, not the
// thumbnail in the header.
const qrLargePixels = 512

// QRImage answers "r" with a code pointing at the uri it was asked for,
// the png alone.
//
// it is one request per code shown instead of one code per row travelling
// with every listing: a folder of a thousand entries would carry a
// thousand images nobody asked to see.
//
// a page which has no url worth scanning has no code either - see
// requestURL. that is a 404 rather than an empty answer: the entry is
// there, the code of it is not.
func QRImage(w http.ResponseWriter, r *http.Request) {

	url := requestURL(r)
	if url == "" {
		Status(w, http.StatusNotFound)
		return
	}

	png, err := qrcode.Encode(url, qrcode.Medium, qrLargePixels)
	if err != nil {
		Status(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

// qrCode draws "url" as a code to scan and hands it back as a data uri -
// no second request, no uri to reserve, same as the stylesheet.
//
// the markup calls it, like it calls humansize: the go side hands over
// the url a page was asked for, what is made of it is the block's
// decision. it is also what keeps the encoding off the fragments htmx
// asks for - those carry no header, and a code nobody draws is not worth
// the cpu.
//
// a template.URL because html/template refuses a "data:" src otherwise.
// nothing to draw comes back as the empty string, which the markup has to
// ask about before it writes an <img>: an empty "src" is not "no image",
// the browser resolves it against the page it is on and fetches that.
func qrCode(url string) template.URL {

	if url == "" {
		return ""
	}

	png, err := qrcode.Encode(url, qrcode.Medium, qrPixels)
	if err != nil {
		return ""
	}

	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
}

// requestURL spells out what "r" asked for: a code is read somewhere else
// than where it is drawn, so the host has to be in it.
//
// a loopback host is no such url. it is resolved wherever it is read, so a
// device scanning it would ask itself and never reach knut. which machine
// the browser showing the page runs on has nothing to do with it - what
// counts is whether the host it asked for leads back here from elsewhere.
func requestURL(r *http.Request) string {

	if isLoopback(r.Host) {
		return ""
	}

	return AbsoluteURL(r)
}

// AbsoluteURL spells out what "r" asked for, scheme and host in front -
// whatever the host is, loopback included. it is the url to paste on the
// machine the page is read on, requestURL the one to hand elsewhere.
func AbsoluteURL(r *http.Request) string {

	if r.Host == "" {
		return ""
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// knut behind a proxy terminating tls is asked over plain http and
	// would hand out a url which downgrades whoever scans it
	if proto := forwardedProto(r); proto != "" {
		scheme = proto
	}

	return scheme + "://" + r.Host + requestURI(r)
}

// isLoopback reports whether "host" is one which leads back to whoever
// resolves it, wherever that is. the port is none of its business, and an
// address in brackets is one net.SplitHostPort hands over without them.
func isLoopback(host string) bool {

	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}

	if strings.EqualFold(name, "localhost") {
		return true
	}

	ip := net.ParseIP(strings.Trim(name, "[]"))

	return ip != nil && ip.IsLoopback()
}

// forwardedProto reads the scheme a proxy in front of knut was asked
// over. a chain of proxies appends to the header, the first entry is the
// one the client spoke; anything but the two schemes knut serves is not
// taken - it ends up in the markup and in the code.
func forwardedProto(r *http.Request) string {

	proto, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	switch proto = strings.TrimSpace(proto); proto {
	case "http", "https":
		return proto
	}

	return ""
}
