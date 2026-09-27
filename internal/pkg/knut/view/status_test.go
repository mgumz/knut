// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// a status page is framed like the page which was asked for: its path as
// the heading, its code in the header
func TestStatusFramedLikeThePage(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/d/missing%20file.txt", nil)
	req.Header.Set("Accept", "text/html")
	req.Host = "10.0.0.1:8080"
	rec := httptest.NewRecorder()
	Status(rec, req, http.StatusNotFound)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusNotFound)
	}

	body := rec.Body.String()
	for _, want := range []string{
		`<h1>/d/missing file.txt</h1>`,
		`<span class="code">404</span> Not Found`,
		`alt="http://10.0.0.1:8080/d/missing%20file.txt"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the status page does not carry %q", want)
		}
	}
}

// without a request in hand the status page is the bare one
func TestStatusWithoutRequest(t *testing.T) {

	rec := httptest.NewRecorder()
	Status(rec, nil, http.StatusInternalServerError)

	body := rec.Body.String()
	if strings.Contains(body, "<h1>") || strings.Contains(body, `class="qr"`) {
		t.Error("a status page without a request carries a heading or a code")
	}
	if !strings.Contains(body, `<span class="code">500</span>`) {
		t.Error("the status page does not carry its code")
	}
}

func TestRootFrom(t *testing.T) {

	for uri, want := range map[string]string{
		"":             "./",
		"/":            "./",
		"/f.txt":       "./",
		"/d/":          "../",
		"/d/a.txt":     "../",
		"/d/sub/":      "../../",
		"/g/knut/x/y/": "../../../../",
	} {
		if got := rootFrom(uri); got != want {
			t.Errorf("rootFrom(%q) = %q, want %q", uri, got, want)
		}
	}
}

// the wordmark leads to the index where there is one, and is plain text
// where there is none
func TestWordmarkLeadsToIndex(t *testing.T) {

	render := func() string {
		req := httptest.NewRequest(http.MethodGet, "/d/sub/", nil)
		req.Header.Set("Accept", "text/html")
		rec := httptest.NewRecorder()
		Status(rec, req, http.StatusNotFound)
		return rec.Body.String()
	}

	if body := render(); !strings.Contains(body, `<span class="wordmark">knut</span>`) {
		t.Error("without an index the wordmark is not plain text")
	}

	SetIndex(true)
	t.Cleanup(func() { SetIndex(false) })

	if body := render(); !strings.Contains(body, `<a class="wordmark" href="../../">knut</a>`) {
		t.Error("the wordmark does not lead to the index")
	}
}
