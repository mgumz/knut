// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut/view"
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
	return []struct {
		name string
		tmpl *template.Template
		data any
		want []string
	}{
		{
			name: "index",
			tmpl: indexTmpl,
			data: struct {
				view.Page
				Windows []string
			}{Page: view.NewPage(""), Windows: []string{"/tree"}},
			want: []string{`<table class="listing windows">`, `<td class="name"><a href="./tree">/tree</a></td>`},
		}, {
			name: "status",
			tmpl: view.Template("status"),
			data: struct {
				view.Page
				Code int
				Text string
			}{Page: view.NewPage(""), Code: 404, Text: "Not Found"},
			want: []string{`<span class="code">404</span>`, "Not Found"},
		}, {
			name: "upload",
			tmpl: uploadTmpl,
			data: struct {
				view.Page
				Action string
			}{Page: view.NewPage("file upload"), Action: "/upload"},
			want: []string{`<div id="upload">`, `type="file"`, `name="upload_file"`},
		}, {
			name: "upload-done",
			tmpl: uploadDoneTmpl,
			data: struct {
				view.Page
				Size     int64
				Duration time.Duration
			}{Page: view.NewPage("file upload"), Size: 4, Duration: time.Second},
			want: []string{"ok, received 4 B in 1s", "upload another file"},
		}, {
			name: "redirect",
			tmpl: redirectTmpl,
			data: struct {
				view.Page
				Location string
			}{Page: view.NewPage(""), Location: "/elsewhere"},
			want: []string{`<a href="/elsewhere">/elsewhere</a>`},
		}, {
			name: "myip",
			tmpl: view.Template("myip"),
			data: &myIP{Page: view.NewPage("myip"), IP: "127.0.0.1", Port: "8080", ASN: "AS1"},
			want: []string{`<span id="ip">127.0.0.1</span>`, `<span id="port">8080</span>`, `<span id="asn">AS1</span>`},
		},
	}
}

// every block renders, inside the layout, with what its handler hands it
func TestPageTemplatesRender(t *testing.T) {

	for _, page := range pageTemplates() {
		t.Run(page.name, func(t *testing.T) {

			body, err := view.Render(page.tmpl, page.data)
			if err != nil {
				t.Fatalf("rendering %q: %v", page.name, err)
			}

			rendered := string(body)
			for _, want := range append(page.want,
				"<!doctype html>", `<span class="wordmark">knut</span>`, "</html>\n") {
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

			content := page.tmpl.Lookup("content")
			if content == nil {
				t.Fatalf("the %q page has no %q template", page.name, "content")
			}

			body, err := view.Render(content, page.data)
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

	view.Template("no-such-block")
}
