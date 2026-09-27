// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

var indexTmpl = view.Template("index")

// IndexHandler lists the published windows on a small index page.
//
// the list never changes, the page around it does: it is the uri it was
// asked for which decides what the header shows, so the index is rendered
// per request like every other page.
func IndexHandler(windows []string) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		view.WriteFor(w, r, indexTmpl, struct {
			view.Page
			Windows []string
		}{Page: view.PageFor(r, ""), Windows: windows})
	})
}
