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
	list := newListing(r, entries, listSort{Key: sortKeyName, Order: orderAsc}, true)

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
