// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"bytes"
	"html/template"
	"net/http"
	"strconv"

	_ "embed"

	"github.com/mgumz/knut/internal/pkg/knut"
)

// knutCSS is the stylesheet shared by all pages knut renders. it is inlined
// into every page instead of being served as its own resource: that way no
// uri has to be reserved for it and the styling survives whatever prefix a
// mapping is published under.
//
//go:embed assets/knut.css
var knutCSS string

// contentTmpl is the name of the page specific part of the layout. htmx
// asks for it alone, see writePageFor.
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
var pages = template.Must(template.New("knut").Funcs(funcs).Parse(knutHTML))

// funcs is what the markup can call. formatting a byte count is a
// presentation decision, so the block which shows one makes it - the go
// side hands over the number.
var funcs = template.FuncMap{"humansize": humanSize}

// Page carries what the "layout" block needs. Page specific data embeds
// it, so a "content" template reaches both its own fields and the ones
// below.
type Page struct {
	Title    string
	Heading  string
	Version  string
	Tree     string
	TreeGone string
	CSS      template.CSS
	Live     bool   // live mode: the page may lean on htmx
	LiveURI  string // where the layout pulls htmx from
	Path     string // the uri this page was asked for, empty without a request
}

// NewPage frames "heading" - an empty one renders the bare knut title.
func NewPage(heading string) Page {
	title := "knut"
	if heading != "" {
		title += " - " + heading
	}
	return Page{
		Title:    title,
		Heading:  heading,
		Version:  knut.Version,
		Tree:     knut.Tree,
		TreeGone: knut.TreeGone,
		CSS:      template.CSS(knutCSS),
		Live:     liveEnabled(),
		LiveURI:  knut.LiveAssetURI,
	}
}

// PageFor frames "heading" for the request which asked for it. a page
// which knows its own uri can knock on it when the connection breaks;
// the ones knut renders without a request in hand - the status pages -
// cannot, and say so without offering to come back.
func PageFor(r *http.Request, heading string) Page {

	page := NewPage(heading)
	page.Path = requestURI(r)

	return page
}

// Template frames the block "name" of assets/knut.html in the
// shared knut layout.
//
// the block is reached through a "content" of its own rather than by
// renaming it: the layout asks for "content", and so does writePageFor
// when htmx wants the block alone.
func Template(name string) *template.Template {

	tmpl := template.Must(pages.Clone())
	if tmpl.Lookup(name) == nil {
		panic("view: no template " + name + " in assets/knut.html")
	}

	template.Must(tmpl.New(contentTmpl).Parse(`{{ template "` + name + `" . }}`))

	return tmpl.Lookup(layoutTmpl)
}

// Render renders "data" into a buffer. rendering upfront keeps a
// template error from ending up as a half written response.
func Render(tmpl *template.Template, data any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := tmpl.Execute(buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write renders "data" and sends it as an html response.
func Write(w http.ResponseWriter, tmpl *template.Template, data any) {
	WriteStatus(w, http.StatusOK, tmpl, data)
}

// isFragment reports whether "r" comes from htmx. those requests get the
// content of a page alone - the layout around it is already on screen.
func isFragment(r *http.Request) bool {
	return liveEnabled() && r.Header.Get("HX-Request") == "true"
}

// WriteFor answers "r" with "data": htmx replaces a piece of a page
// which is already on screen, so it gets the content template alone.
// every other client - a browser navigating, curl, wget - gets the page.
func WriteFor(w http.ResponseWriter, r *http.Request, tmpl *template.Template, data any) {

	if isFragment(r) {
		if content := tmpl.Lookup(contentTmpl); content != nil {
			tmpl = content
		}
	}

	Write(w, tmpl, data)
}

// WriteStatus renders "data" as an html response under "code".
func WriteStatus(w http.ResponseWriter, code int, tmpl *template.Template, data any) {
	body, err := Render(tmpl, data)
	if err != nil {
		Status(w, http.StatusInternalServerError)
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
