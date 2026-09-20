// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"
)

var statusTmpl = newPageTemplate("status")

// writeStatus renders the given status code and
// a text associated with that code
func writeStatus(w http.ResponseWriter, code int) {

	status := struct {
		page
		Code int
		Text string
	}{
		page: newPage(""),
		Code: code,
		Text: http.StatusText(code),
	}

	body, err := renderPage(statusTmpl, status)
	if err != nil {
		// the status page is what writePage falls back to, it cannot
		// fall back onto itself
		http.Error(w, http.StatusText(code), code)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	w.Write(body)
}
