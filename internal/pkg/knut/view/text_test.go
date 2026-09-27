// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWantsText(t *testing.T) {

	const (
		chrome  = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"
		firefox = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	)

	for _, test := range []struct {
		name   string
		target string
		accept string
		htmx   bool
		want   bool
	}{
		{"chrome", "/", chrome, false, false},
		{"firefox", "/", firefox, false, false},
		{"curl", "/", "*/*", false, true},
		{"no accept", "/", "", false, true},
		{"plain text", "/", "text/plain", false, true},
		{"xhtml alone", "/", "application/xhtml+xml", false, false},
		{"html refused", "/", "text/html;q=0, */*", false, true},
		{"html weighed low", "/", "text/plain, text/html;q=0.1", false, false},
		{"?text in a browser", "/?text", firefox, false, true},
		{"?html from curl", "/?html", "*/*", false, false},
		{"htmx", "/", "*/*", true, false},
		{"htmx and ?text", "/?text", "*/*", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.target, nil)
			if test.accept != "" {
				req.Header.Set("Accept", test.accept)
			}
			if test.htmx {
				req.Header.Set("HX-Request", "true")
			}
			if got := wantsText(req); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

// a page without a text form is the page, whoever asks, and does not vary
func TestNoTextFormIsThePage(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	WriteFor(rec, req, Template("upload"), struct {
		Page
		Action string
	}{Page: NewPage("")})

	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("got content type %q, want the page", got)
	}
	if got := rec.Header().Get("Vary"); got != "" {
		t.Errorf("got Vary %q, want none", got)
	}
}
