// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"cmp"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "embed"
)

// filterJS narrows a listing in the browser. it is inlined into the listing
// page the way the stylesheet is inlined into every page: no uri has to be
// reserved for it and it survives whatever prefix a mapping is published
// under. it ships with the listing alone - no other page has rows to hide.
//
//go:embed assets/listing-filter.js
var filterJS string

// keysJS walks the rows of a listing from the keyboard. it is inlined like
// the filter, and like it it adds: the listing is read and clicked the same
// way without it.
//
//go:embed assets/listing-keys.js
var keysJS string

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

// what a listing asks of a folder or a file besides the thing itself:
// "?zip" is the folder as an archive, "?qr" the code pointing at either.
//
// they hang on the uri they mean instead of on a reserved one of their
// own: nothing has to be taken out of a published tree, and the links
// survive whatever prefix a mapping sits under. the markup writes them
// where ListOpts says they are answered, the handler which answers them
// reads them back - hence exported, the two are a package apart.
const (
	QueryZip = "zip"
	QueryQR  = "qr"
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

// ListEntry is one row of a rendered folder, either a file on disk or an
// entry inside a zip.
type ListEntry struct {
	Name       string // display name, folders carry a trailing "/"
	URL        string // href, relative to the folder being listed
	Type       entryType
	Date       string // modification time, local
	ISO        string // the same time, machine readable
	ArchiveZip string // download of the whole folder, empty where there is none
	CodeQR     bool   // this entry can be shown as a code to scan

	Bytes int64 // size of a file, 0 for a folder - the markup humanizes it
	mod   time.Time
	Sig   uint64 // content fingerprint, 0 when there is none to be had
}

// Dir marks the rows a listing renders as folders.
func (e ListEntry) Dir() bool { return e.Type == entryDir }

// NewListEntry builds the row for "name". "name" is the plain entry name
// without any path, the link is kept relative so it resolves below
// whatever uri the folder is published at.
func NewListEntry(name string, size int64, mod time.Time, isDir bool) ListEntry {
	entry := ListEntry{
		Name:  name,
		URL:   (&url.URL{Path: name}).String(),
		Type:  entryFile,
		Bytes: size,
		mod:   mod,
	}

	if isDir {
		entry.Name, entry.URL = name+"/", entry.URL+"/"
		entry.Type, entry.Bytes = entryDir, 0
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
func (sort listSort) apply(entries []ListEntry) {

	compare := func(a, b ListEntry) int { return strings.Compare(a.Name, b.Name) }
	switch sort.Key {
	case sortKeyType:
		// folders first, then the files
		compare = func(a, b ListEntry) int { return cmp.Compare(a.Type, b.Type) }
	case sortKeySize:
		compare = func(a, b ListEntry) int { return cmp.Compare(a.Bytes, b.Bytes) }
	case sortKeyDate:
		compare = func(a, b ListEntry) int { return a.mod.Compare(b.mod) }
	}

	slices.SortStableFunc(entries, func(a, b ListEntry) int {
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
			column.Arrow = " ▴"
			if sort.Order == orderDesc {
				column.Arrow = " ▾"
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
	Page
	Columns []listColumn
	Entries []ListEntry
	Parent  string // link to the enclosing folder, empty at the top
	Summary string
	Watch   string // url a live listing polls, empty when it does not

	// what the folder on screen offers of itself. the rows carry the
	// same two for what is in it.
	ArchiveZip string // download of this folder, empty where there is none
	CodeQR     bool   // this folder can be shown as a code to scan

	Filter   template.JS // the client side filter, inlined into the page
	Keys     template.JS // the keys which walk the rows, the same
	Fragment bool        // this render goes to htmx, which wants #listing alone
}

// ListOpts is what the handler behind a listing answers, and so what the
// rendered folder may offer. the page and the handler are two packages
// apart: a listing which links what nobody answers is a dead link, and one
// which hides what is answered is a feature nobody finds.
type ListOpts struct {

	// Dir is the folder on disk whose changes this listing follows: the
	// folder itself for a tree, the folder the zip sits in for a zip.
	// empty means there is nothing to watch, and then the listing renders
	// once and arms no poll - there is no point sending a client back for
	// an answer which can never come.
	Dir string

	// Parent says there is an enclosing folder to climb to.
	Parent bool

	// Zip says "?zip" is answered: the folder on screen and every folder
	// in it can be taken along as an archive.
	Zip bool

	// QR says "?qr" is answered: every row, and the folder itself, can be
	// handed to a device which is not the one reading the listing.
	QR bool
}

// offer hands the folder on screen, and every row of it, the links the
// handler behind the listing answers.
//
// the codes hang on the url of the page, so a page which has none - it was
// asked for as "localhost", which leads back to whoever reads it - offers
// none of them, the call the header makes for its own code.
func (l *listing) offer(opts ListOpts) {

	scannable := opts.QR && l.URL != ""

	if opts.Zip {
		l.ArchiveZip = "?" + QueryZip
	}
	l.CodeQR = scannable

	for i := range l.Entries {
		if opts.Zip && l.Entries[i].Dir() {
			l.Entries[i].ArchiveZip = l.Entries[i].URL + "?" + QueryZip
		}
		l.Entries[i].CodeQR = scannable
	}
}

// watch arms the listing for live updates: the rendered block polls the
// folder it shows and hands "state" - the fingerprint of what is on the
// screen - back, so a change during the round trip is not sat out.
//
// the url is spelled out instead of left empty: an empty "hx-get" is no
// url to htmx, and the sort has to survive the refresh.
func (l *listing) watch(sort listSort, state string) {
	l.Watch = "?sort=" + sort.Key +
		"&order=" + sort.Order +
		"&" + liveParam + "=" + state
}

// newListing sorts "entries" and frames them as the page "r" asked for.
func newListing(r *http.Request, entries []ListEntry, sort listSort, opts ListOpts) listing {

	sort.apply(entries)

	list := listing{
		Page:     PageFor(r, RequestPath(r)),
		Columns:  sort.columns(),
		Entries:  entries,
		Summary:  summarize(entries),
		Filter:   template.JS(filterJS),
		Keys:     template.JS(keysJS),
		Fragment: isFragment(r),
	}
	if opts.Parent {
		list.Parent = "../"
	}
	list.offer(opts)

	return list
}

// Listing answers "r" with the folder "read" lists, as "opts" says the
// handler behind it can. it is the whole response path of a listing,
// shared by the folders on disk and the ones inside a zip - what those two
// do differently is the read, not what becomes of it.
//
// a failed read is handed back instead of answered: what a folder which
// cannot be read means is the caller's to say.
func Listing(w http.ResponseWriter, r *http.Request, read ReadListing, opts ListOpts) error {

	entries, state, err := read()
	if err != nil {
		return err
	}

	watchable := liveEnabled() && opts.Dir != ""

	// a poll from a live listing showing exactly what is there: hold the
	// request until the folder moves. one held request per client, woken
	// by the filesystem.
	if want := r.URL.Query().Get(liveParam); watchable && want == state {
		entries, state = watchListing(r.Context(), opts.Dir, read, entries, state)
	}

	sort := listSortFromQuery(r.URL.Query())
	list := newListing(r, entries, sort, opts)
	if watchable {
		list.watch(sort, state)
	}

	WriteFor(w, r, listingTmpl, list)

	return nil
}

// summarize counts what is in the folder, "du"-style.
func summarize(entries []ListEntry) string {

	dirs, bytes := 0, int64(0)
	for i := range entries {
		if entries[i].Type == entryDir {
			dirs++
			continue
		}
		bytes += entries[i].Bytes
	}

	files := len(entries) - dirs
	summary := Plural(dirs, "folder") + " ι " + Plural(files, "file")
	if files > 0 {
		summary += " ι " + humanSize(bytes)
	}

	return summary
}

// Plural counts "noun"s the way english expects it.
func Plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// the whole listing sits in one block, table or bare tree: a live listing
// swaps that block for the one the server just rendered, and a folder
// which fell empty (or filled up) swaps along with everything else.
var listingTmpl = Template("listing")
