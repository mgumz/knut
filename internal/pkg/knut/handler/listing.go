// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"cmp"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// the listing is rendered in the order given by "?sort=" and "?order=".
const (
	sortKeyName = "name"
	sortKeyType = "type"
	sortKeySize = "size"
	sortKeyDate = "date"

	orderAsc  = "asc"
	orderDesc = "desc"

	// dateFormat keeps the column at a fixed width in a monospace font.
	dateFormat = "2006-01-02 15:04"
)

// entryType is the kind of an entry as a number: sorting by type compares
// those instead of their names, the order of the constants is the order
// they are listed in.
type entryType int

const (
	entryDir entryType = iota
	entryFile
)

var entryTypeNames = [...]string{entryDir: "dir", entryFile: "file"}

func (t entryType) String() string { return entryTypeNames[t] }

// listEntry is one row of a rendered folder, either a file on disk or an
// entry inside a zip.
type listEntry struct {
	Name string // display name, folders carry a trailing "/"
	URL  string // href, relative to the folder being listed
	Type entryType
	Size string // humanized, "-" for folders
	Date string // modification time, local
	ISO  string // the same time, machine readable

	bytes int64
	mod   time.Time
}

// Dir marks the rows a listing renders as folders.
func (e listEntry) Dir() bool { return e.Type == entryDir }

// newListEntry builds the row for "name". "name" is the plain entry name
// without any path, the link is kept relative so it resolves below
// whatever uri the folder is published at.
func newListEntry(name string, size int64, mod time.Time, isDir bool) listEntry {
	entry := listEntry{
		Name:  name,
		URL:   (&url.URL{Path: name}).String(),
		Type:  entryFile,
		Size:  humanSize(size),
		bytes: size,
		mod:   mod,
	}

	if isDir {
		entry.Name, entry.URL = name+"/", entry.URL+"/"
		entry.Type, entry.Size = entryDir, "-"
		entry.bytes = 0
	}

	if !mod.IsZero() {
		entry.Date, entry.ISO = mod.Format(dateFormat), mod.Format(time.RFC3339)
	}

	return entry
}

// listSort is the order a listing is rendered in.
type listSort struct {
	Key   string
	Order string
}

// listSortFromQuery reads the sort order from "?sort=&order=". anything
// unknown falls back to name, ascending.
func listSortFromQuery(query url.Values) listSort {

	sort := listSort{Key: query.Get("sort"), Order: query.Get("order")}

	switch sort.Key {
	case sortKeyType, sortKeySize, sortKeyDate:
	default:
		sort.Key = sortKeyName
	}
	if sort.Order != orderDesc {
		sort.Order = orderAsc
	}

	return sort
}

// apply orders "entries" in place. only the type sorts folders apart, by
// size or date the newest or biggest entry wins, folder or not.
func (sort listSort) apply(entries []listEntry) {

	compare := func(a, b listEntry) int { return strings.Compare(a.Name, b.Name) }
	switch sort.Key {
	case sortKeyType:
		// folders first, then the files
		compare = func(a, b listEntry) int { return cmp.Compare(a.Type, b.Type) }
	case sortKeySize:
		compare = func(a, b listEntry) int { return cmp.Compare(a.bytes, b.bytes) }
	case sortKeyDate:
		compare = func(a, b listEntry) int { return a.mod.Compare(b.mod) }
	}

	slices.SortStableFunc(entries, func(a, b listEntry) int {
		order := compare(a, b)
		if sort.Order == orderDesc {
			order = -order
		}
		if order != 0 {
			return order
		}
		// equal sizes or timestamps are common, the name decides
		return strings.Compare(a.Name, b.Name)
	})
}

// listColumn is a sortable column header.
type listColumn struct {
	Key   string
	URL   string // sorts the listing by this column
	Arrow string // marks the column the listing is sorted by
}

// columns builds the header links: following the column the listing is
// sorted by flips the order, any other column starts ascending.
func (sort listSort) columns() []listColumn {

	keys := []string{sortKeyName, sortKeyType, sortKeySize, sortKeyDate}
	columns := make([]listColumn, 0, len(keys))
	for _, key := range keys {
		column, order := listColumn{Key: key}, orderAsc
		if key == sort.Key {
			column.Arrow = " ↑"
			if sort.Order == orderDesc {
				column.Arrow = " ↓"
			} else {
				order = orderDesc
			}
		}
		column.URL = "?sort=" + key + "&order=" + order
		columns = append(columns, column)
	}

	return columns
}

// listing is the page behind a rendered folder.
type listing struct {
	page
	Columns []listColumn
	Entries []listEntry
	Parent  string // link to the enclosing folder, empty at the top
	Summary string
}

// newListing sorts "entries" and frames them as a page titled by "folder".
func newListing(folder string, entries []listEntry, sort listSort, parent bool) listing {

	sort.apply(entries)

	list := listing{
		page:    newPage(folder),
		Columns: sort.columns(),
		Entries: entries,
		Summary: summarize(entries),
	}
	if parent {
		list.Parent = "../"
	}

	return list
}

// summarize counts what is in the folder, "du"-style.
func summarize(entries []listEntry) string {

	dirs, bytes := 0, int64(0)
	for i := range entries {
		if entries[i].Type == entryDir {
			dirs++
			continue
		}
		bytes += entries[i].bytes
	}

	files := len(entries) - dirs
	summary := plural(dirs, "folder") + " ι " + plural(files, "file")
	if files > 0 {
		summary += " ι " + humanSize(bytes)
	}

	return summary
}

// plural counts "noun"s the way english expects it.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

var listingTmpl = newPageTemplate("listing")
