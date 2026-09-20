// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"html/template"
	"strings"
	"testing"
	"time"
)

// the blocks of assets/knut.html, each with the data its handler renders
// it with. a field renamed in go and not in the markup renders as nothing
// at all, which no compiler catches - this does.
func pageTemplates() []struct {
	name string
	tmpl *template.Template
	data any
	want []string
} {
	entries := []listEntry{newListEntry("a.txt", 12, modTime(15), false)}

	return []struct {
		name string
		tmpl *template.Template
		data any
		want []string
	}{
		{
			name: "listing",
			tmpl: listingTmpl,
			data: newListing("/", entries, listSort{Key: sortKeyName, Order: orderAsc}, true),
			want: []string{`<table class="listing">`, "a.txt", "12 B", "2026-09-19 15:04", "1 file"},
		}, {
			name: "index",
			tmpl: indexTmpl,
			data: struct {
				page
				Windows []string
			}{page: newPage(""), Windows: []string{"/tree"}},
			want: []string{`<ul class="windows">`, `<a href="./tree">/tree</a>`},
		}, {
			name: "status",
			tmpl: statusTmpl,
			data: struct {
				page
				Code int
				Text string
			}{page: newPage(""), Code: 404, Text: "Not Found"},
			want: []string{`<span class="code">404</span>`, "Not Found"},
		}, {
			name: "upload",
			tmpl: uploadTmpl,
			data: struct {
				page
				Action string
			}{page: newPage("file upload"), Action: "/upload"},
			want: []string{`<div id="upload">`, `type="file"`, `name="upload_file"`},
		}, {
			name: "upload-done",
			tmpl: uploadDoneTmpl,
			data: struct {
				page
				Size     string
				Duration time.Duration
			}{page: newPage("file upload"), Size: "4 B", Duration: time.Second},
			want: []string{"ok, received 4 B in 1s", "upload another file"},
		}, {
			name: "redirect",
			tmpl: redirectTmpl,
			data: struct {
				page
				Location string
			}{page: newPage(""), Location: "/elsewhere"},
			want: []string{`<a href="/elsewhere">/elsewhere</a>`},
		}, {
			name: "myip",
			tmpl: newPageTemplate("myip"),
			data: &myIP{page: newPage("myip"), IP: "127.0.0.1", Port: "8080", ASN: "AS1"},
			want: []string{`<span id="ip">127.0.0.1</span>`, `<span id="port">8080</span>`, `<span id="asn">AS1</span>`},
		},
	}
}

// every block renders, inside the layout, with what its handler hands it
func TestPageTemplatesRender(t *testing.T) {

	for _, page := range pageTemplates() {
		t.Run(page.name, func(t *testing.T) {

			body, err := renderPage(page.tmpl, page.data)
			if err != nil {
				t.Fatalf("rendering %q: %v", page.name, err)
			}

			rendered := string(body)
			for _, want := range append(page.want,
				"<!doctype html>", `<div class="brand">knut</div>`, "</html>\n") {
				if !strings.Contains(rendered, want) {
					t.Errorf("the %q page does not render %q", page.name, want)
				}
			}

			// a block which lost its data renders the layout and nothing
			// between the heading and the footer
			if _, body, _ := strings.Cut(rendered, `<div class="page">`); len(strings.TrimSpace(body)) < 60 {
				t.Errorf("the %q page renders an empty layout", page.name)
			}
		})
	}
}

// htmx gets the block alone: same markup, no layout around it
func TestPageTemplatesRenderAsFragment(t *testing.T) {

	liveMode(t)

	for _, page := range pageTemplates() {
		t.Run(page.name, func(t *testing.T) {

			content := page.tmpl.Lookup(contentTmpl)
			if content == nil {
				t.Fatalf("the %q page has no %q template", page.name, contentTmpl)
			}

			body, err := renderPage(content, page.data)
			if err != nil {
				t.Fatalf("rendering the %q fragment: %v", page.name, err)
			}

			fragment := string(body)
			if strings.Contains(fragment, "<!doctype html>") {
				t.Errorf("the %q fragment carries the layout", page.name)
			}
			for _, want := range page.want {
				if !strings.Contains(fragment, want) {
					t.Errorf("the %q fragment does not render %q", page.name, want)
				}
			}
		})
	}
}

// a page asking for a block which is not in assets/knut.html is a typo,
// and it is one at startup - not on the request which first renders it
func TestNewPageTemplateRejectsUnknownBlock(t *testing.T) {

	defer func() {
		if recover() == nil {
			t.Error("an unknown block was accepted")
		}
	}()

	newPageTemplate("no-such-block")
}
