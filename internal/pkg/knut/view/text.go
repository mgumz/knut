// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"bytes"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	texttemplate "text/template"

	_ "embed"
)

// knutTXT is the text form of the pages which have one: what curl, wget
// and scripts get instead of the markup, see wantsText.
//
//go:embed assets/knut.txt
var knutTXT string

// the query overrides what the Accept header asks for, either way.
const (
	queryText = "text"
	queryHTML = "html"
)

var texts = texttemplate.Must(texttemplate.New("knut").
	Funcs(texttemplate.FuncMap{"humansize": humanSize, "row": textRow}).
	Parse(knutTXT))

// textForms holds the text form of a page template, keyed by what
// Template handed out: the handlers pass that around, and keep doing so.
var textForms sync.Map

func registerText(page *template.Template, name string) {
	if text := texts.Lookup(name); text != nil {
		textForms.Store(page, text)
	}
}

func textFor(page *template.Template) *texttemplate.Template {
	text, _ := textForms.Load(page)
	form, _ := text.(*texttemplate.Template)
	return form
}

// wantsText reports whether "r" gets the text form of a page: "?text"
// and "?html" decide outright, else anything which does not ask for
// html. browsers navigating list text/html, curl and wget send "*/*".
//
// htmx sends "*/*" as well, and it sits in a page which wants markup -
// even one left open while knut restarted without -live.
func wantsText(r *http.Request) bool {

	query := r.URL.Query()
	switch {
	case r.Header.Get("HX-Request") == "true", query.Has(queryHTML):
		return false
	case query.Has(queryText):
		return true
	}

	return !acceptsHTML(r.Header.Values("Accept"))
}

// acceptsHTML reports whether an Accept header names html itself, with
// a weight above 0. "*/*" is no such wish.
func acceptsHTML(accept []string) bool {

	for _, value := range accept {
		for _, part := range strings.Split(value, ",") {
			media, params, err := mime.ParseMediaType(part)
			if err != nil || (media != "text/html" && media != "application/xhtml+xml") {
				continue
			}
			if q, err := strconv.ParseFloat(params["q"], 64); err == nil && q <= 0 {
				continue
			}
			return true
		}
	}

	return false
}

// textRow is one line of a text form: the cells tab separated, lined up
// by writeText.
func textRow(cells ...any) string {

	line := make([]string, len(cells))
	for i := range cells {
		line[i] = fmt.Sprint(cells[i])
	}

	return strings.Join(line, "\t") + "\n"
}

// writeTextFor answers "r" with the text form of the page "tmpl", if it
// has one and "r" wants it. a page with a text form varies by Accept,
// whichever form it went out in.
func writeTextFor(w http.ResponseWriter, r *http.Request, code int, tmpl *template.Template, data any) bool {

	text := textFor(tmpl)
	if text == nil {
		return false
	}

	w.Header().Add("Vary", "Accept")
	if !wantsText(r) {
		return false
	}

	writeText(w, code, text, data)

	return true
}

// writeText renders "data" as plain text under "code", the columns of
// its rows lined up.
func writeText(w http.ResponseWriter, code int, tmpl *texttemplate.Template, data any) {

	buf := bytes.NewBuffer(nil)
	columns := tabwriter.NewWriter(buf, 0, 8, 2, ' ', 0)
	if err := tmpl.Execute(columns, data); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	columns.Flush()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	w.Write(buf.Bytes())
}
