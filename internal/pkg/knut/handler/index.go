// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"
)

var indexTmpl = newPageTemplate("index", `<pre class="tree">{{ .Tree }}</pre>
<ul class="windows">
{{- range .Windows }}
<li><a href=".{{ . }}">{{ . }}</a></li>
{{- end }}
</ul>
`)

// IndexHandler lists the published windows on a small index page.
func IndexHandler(windows []string) http.Handler {

	data := struct {
		page
		Windows []string
	}{page: newPage(""), Windows: windows}

	body, err := renderPage(indexTmpl, data)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			writeStatus(w, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(body)
	})
}
