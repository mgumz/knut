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

// contentTmpl is the name of the page specific part of the layout.
const contentTmpl = "content"

// layoutTmpl is the frame around every page, the block the other ones are
// rendered inside of.
const layoutTmpl = "layout"

// knutHTML is every page knut renders: the layout and one block per page.
// it sits next to the stylesheet instead of in a string literal here -
// markup is easier to read, to diff and to edit as markup.
//
//go:embed assets/knut.html
var knutHTML string

// pages is knutHTML parsed, once. a page template is this set cloned with
// "content" pointed at one of its blocks, see newPageTemplate.
var pages = template.Must(template.New("knut").Parse(knutHTML))

// page carries what the "layout" block needs. page specific data embeds
// it, so a "content" template reaches both its own fields and the ones
// below.
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

// newPageTemplate frames the block "name" of assets/knut.html in the
// shared knut layout.
//
// the block is reached through a "content" of its own rather than by
// renaming it: the layout asks for "content".
func newPageTemplate(name string) *template.Template {

	tmpl := template.Must(pages.Clone())
	if tmpl.Lookup(name) == nil {
		panic("handler: no template " + name + " in assets/knut.html")
	}

	template.Must(tmpl.New(contentTmpl).Parse(`{{ template "` + name + `" . }}`))

	return tmpl.Lookup(layoutTmpl)
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
