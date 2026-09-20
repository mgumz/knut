// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"encoding/base64"
	"html/template"
	"net"
	"net/http"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// qrPixels is how wide the png is drawn. it is scaled down by the markup,
// which keeps the code sharp on a display which packs more than one pixel
// per css pixel - and costs well under a kilobyte either way.
const qrPixels = 256

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

// requestURL spells out what "r" asked for: a code is scanned by a device
// which is not the one showing the page, so the host has to be in it -
// and a host only that one device knows is no url to hand anybody.
func requestURL(r *http.Request) string {

	if r.Host == "" || isLoopback(r.Host) {
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

// isLoopback reports whether "host" names the machine the browser itself
// runs on. the port is none of its business, and an address in brackets
// is one net.SplitHostPort hands over without them.
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
