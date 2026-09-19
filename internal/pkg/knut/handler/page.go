// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	_ "embed"
	"html/template"
	"net/http"
	"strconv"

	"github.com/mgumz/knut/internal/pkg/knut"
)

// knutCSS is the stylesheet shared by all pages knut renders. it is inlined
// into every page instead of being served as its own resource: that way no
// uri has to be reserved for it and the styling survives whatever prefix a
// mapping is published under.
//
//go:embed assets/knut.css
var knutCSS string

// layoutHTML is the frame around every page. the page specific markup is
// pulled in as the "content" template.
const layoutHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{ .Title }}</title>
<style>{{ .CSS }}</style>
</head>
<body>
<main>
<div class="brand">knut</div>
<div class="page">
{{- with .Heading }}
<h1>{{ . }}</h1>
{{- end }}
{{ template "content" . }}
</div>
</main>
<footer><div>knut {{ .Version }}</div></footer>
</body>
</html>
`

// page carries what layoutHTML needs. page specific data embeds it, so a
// "content" template reaches both its own fields and the ones below.
type page struct {
	Title   string
	Heading string
	Version string
	Tree    string
	CSS     template.CSS
}

// newPage frames "heading" - an empty one renders the bare knut title.
func newPage(heading string) page {
	title := "knut"
	if heading != "" {
		title += " - " + heading
	}
	return page{
		Title:   title,
		Heading: heading,
		Version: knut.Version,
		Tree:    knut.Tree,
		CSS:     template.CSS(knutCSS),
	}
}

// newPageTemplate renders "content" inside the shared knut layout.
func newPageTemplate(name, content string) *template.Template {
	tmpl := template.Must(template.New(name).Parse(layoutHTML))
	template.Must(tmpl.New("content").Parse(content))
	return tmpl
}

// renderPage renders "data" into a buffer. rendering upfront keeps a
// template error from ending up as a half written response.
func renderPage(tmpl *template.Template, data any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writePage renders "data" and sends it as an html response.
func writePage(w http.ResponseWriter, tmpl *template.Template, data any) {
	writePageStatus(w, http.StatusOK, tmpl, data)
}

// writePageStatus renders "data" as an html response under "code".
func writePageStatus(w http.ResponseWriter, code int, tmpl *template.Template, data any) {
	body, err := renderPage(tmpl, data)
	if err != nil {
		writeStatus(w, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	w.Write(body)
}

// humanSize renders "n" bytes in the largest unit which keeps the
// number below 1024.
func humanSize(n int64) string {
	const unit = 1024

	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}

	div, exp := int64(unit), 0
	for m := n / unit; m >= unit && exp < 5; m /= unit {
		div *= unit
		exp++
	}

	units := [...]string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	return strconv.FormatFloat(float64(n)/float64(div), 'f', 1, 64) + " " + units[exp]
}
