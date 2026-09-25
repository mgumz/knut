// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// indexPage shadows a generated listing, just like in http.FileServer.
const indexPage = "index.html"

// DirListHandler serves the tree rooted at "fsys": folders are rendered as
// a sortable listing, everything else is handed to http.FileServer.
//
// the listing replaces the plain <pre> block of names http.FileServer
// generates: it carries sizes and modification times and can be ordered by
// name, size or date via "?sort=" and "?order=".
func DirListHandler(fsys http.FileSystem) http.Handler {

	files := http.FileServer(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}
		name := path.Clean(r.URL.Path)

		dir, err := fsys.Open(name)
		if err != nil {
			view.Status(w, statusForError(err))
			return
		}
		defer dir.Close()

		if fi, err := dir.Stat(); err != nil || !fi.IsDir() {
			files.ServeHTTP(w, r)
			return
		}

		// a folder addressed without a trailing "/": redirect like
		// http.FileServer does, otherwise the relative links of the
		// listing resolve against the parent.
		if !strings.HasSuffix(r.URL.Path, "/") {
			localRedirect(w, r, path.Base(name)+"/")
			return
		}

		// "?zip" is asked of the folder itself, so it is answered before
		// anything which decides what the folder looks like
		if _, ok := r.URL.Query()[view.QueryZip]; ok {
			zipFolder(w, r, fsys, name)
			return
		}

		if index, err := fsys.Open(path.Join(name, indexPage)); err == nil {
			index.Close()
			files.ServeHTTP(w, r)
			return
		}

		read := func() ([]view.ListEntry, string, error) { return readDir(fsys, name) }

		// what the rows and the header of the listing may offer is what
		// the lines above answer
		opts := view.ListOpts{
			Dir:    watchDir(fsys, name),
			Parent: name != "/",
			Zip:    true,
		}

		// the folder may be gone by now - it was opened above, not held
		if err := view.Listing(w, r, read, opts); err != nil {
			view.Status(w, statusForError(err))
		}
	})
}

// watchDir is the folder on disk behind "name", the one a live listing
// follows. it is empty when there is none.
//
// knut publishes trees as http.Dir; anything else - an in-memory tree in
// a test, an fs.FS handed to http.FS - has no path to give a watcher, and
// a listing of it simply does not refresh itself.
func watchDir(fsys http.FileSystem, name string) string {

	dir, ok := fsys.(http.Dir)
	if !ok {
		return ""
	}

	return filepath.Join(string(dir), filepath.FromSlash(name))
}

// readDir lists the folder "name" below "fsys" as the rows of a listing,
// together with the fingerprint of what it saw. a live listing reads the
// same folder over and over, so this is one call, not an open handle.
func readDir(fsys http.FileSystem, name string) ([]view.ListEntry, string, error) {

	dir, err := fsys.Open(name)
	if err != nil {
		return nil, "", err
	}
	defer dir.Close()

	infos, err := dir.Readdir(-1)
	if err != nil {
		return nil, "", err
	}

	entries := make([]view.ListEntry, 0, len(infos))
	for _, fi := range infos {
		entries = append(entries,
			view.NewListEntry(fi.Name(), fi.Size(), fi.ModTime(), fi.IsDir()))
	}

	return entries, view.LiveState(entries), nil
}

// statusForError makes of an open error what http.FileServer would make
// of it - knut just renders the status itself.
func statusForError(err error) int {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, fs.ErrPermission):
		return http.StatusForbidden
	}
	return http.StatusInternalServerError
}

// localRedirect points at "to", relative to the requested folder, keeping
// the query - the same move http.FileServer makes for a missing "/".
func localRedirect(w http.ResponseWriter, r *http.Request, to string) {
	if query := r.URL.RawQuery; query != "" {
		to += "?" + query
	}
	w.Header().Set("Location", to)
	w.WriteHeader(http.StatusMovedPermanently)
}
