// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// the listing block renders the model the readers hand it: the rows, the
// sizes and the summary a folder page is made of.
func TestListingRenders(t *testing.T) {

	entries := []ListEntry{NewListEntry("a.txt", 12, modTime(15), false)}
	r := httptest.NewRequest("GET", "/", nil)
	list := newListing(r, entries, listSort{Key: sortKeyName, Order: orderAsc}, ListOpts{Parent: true})

	body, err := Render(Template("listing"), list)
	if err != nil {
		t.Fatalf("rendering the listing: %v", err)
	}

	for _, want := range []string{
		"<!doctype html>", `<table class="listing">`, "a.txt", "12 B",
		"2026-09-19 15:04", "1 file", `<a href="../">../</a>`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the listing does not render %q", want)
		}
	}
}

// the filter ships with the listing: the box hidden until the script
// unhides it, the script itself inlined the way the stylesheet is.
func TestListingCarriesTheFilter(t *testing.T) {

	entries := []ListEntry{NewListEntry("a.txt", 12, modTime(15), false)}
	r := httptest.NewRequest("GET", "/", nil)
	list := newListing(r, entries, listSort{Key: sortKeyName, Order: orderAsc}, ListOpts{Parent: true})

	body, err := Render(Template("listing"), list)
	if err != nil {
		t.Fatalf("rendering the listing: %v", err)
	}

	for _, want := range []string{
		`<input id="filter" type="search"`, "hidden>",
		`<span id="filter-count" class="meta"></span>`,
		"getElementById(\"filter\")",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the listing does not render %q", want)
		}
	}

	// the box is no part of the block a live listing swaps - inside it the
	// typed query would be dropped on every poll
	filter, listing := strings.Index(string(body), `<div class="filter">`),
		strings.Index(string(body), `<div id="listing"`)
	if filter < 0 || listing < 0 || filter > listing {
		t.Errorf("the filter box sits at %d, the listing block at %d", filter, listing)
	}

	// the way out of a folder is never filtered away
	if !strings.Contains(string(body), `<tr class="dir parent">`) {
		t.Error("the parent row is not marked as such")
	}
}

// htmx takes "#listing" out of the response and drops the rest: the script
// has no business travelling with every poll.
func TestListingFragmentSkipsTheFilter(t *testing.T) {

	SetLive(true)
	defer SetLive(false)

	entries := []ListEntry{NewListEntry("a.txt", 12, modTime(15), false)}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("HX-Request", "true")

	list := newListing(r, entries, listSort{Key: sortKeyName, Order: orderAsc}, ListOpts{Parent: true})
	if !list.Fragment {
		t.Fatal("a request from htmx does not render as a fragment")
	}

	// the block alone, the way WriteFor hands it to htmx
	content := Template("listing").Lookup(contentTmpl)
	if content == nil {
		t.Fatal("the listing has no content template")
	}

	body, err := Render(content, list)
	if err != nil {
		t.Fatalf("rendering the listing: %v", err)
	}

	if strings.Contains(string(body), "<script") {
		t.Error("the fragment ships the filter script")
	}
	if strings.Contains(string(body), `<div class="filter">`) {
		t.Error("the fragment ships the filter box")
	}
	if !strings.HasPrefix(string(body), `<div id="listing"`) {
		t.Errorf("the fragment starts with %.40q", string(body))
	}
}
