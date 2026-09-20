// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

var indexTmpl = view.Template("index")

// IndexHandler lists the published windows on a small index page.
func IndexHandler(windows []string) http.Handler {

	data := struct {
		view.Page
		Windows []string
	}{Page: view.NewPage(""), Windows: windows}

	body, err := view.Render(indexTmpl, data)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			view.Status(w, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(body)
	})
}
