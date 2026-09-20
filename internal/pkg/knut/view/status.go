// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"net/http"
	"net/url"
	"strings"
)

var statusTmpl = Template("status")

// Status renders the given status code and a text associated with that
// code.
func Status(w http.ResponseWriter, code int) {

	status := struct {
		Page
		Code int
		Text string
	}{
		Page: NewPage(""),
		Code: code,
		Text: http.StatusText(code),
	}

	body, err := Render(statusTmpl, status)
	if err != nil {
		// the status page is what Write falls back to, it cannot
		// fall back onto itself
		http.Error(w, http.StatusText(code), code)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	w.Write(body)
}

// RequestPath returns the path as the client asked for it, unescaped for
// a reader: by the time a listing is rendered, r.URL.Path has lost the
// prefix of its mapping.
func RequestPath(r *http.Request) string {

	uri := requestURI(r)
	if unescaped, err := url.PathUnescape(uri); err == nil {
		return unescaped
	}

	return uri
}

// requestURI is the same path still escaped, the shape it can be asked
// for again in - a link, an "hx-get", the content of a qr code.
func requestURI(r *http.Request) string {

	uri := r.RequestURI
	if uri == "" {
		return r.URL.EscapedPath()
	}
	if i := strings.IndexByte(uri, '?'); i > -1 {
		uri = uri[:i]
	}

	return uri
}
