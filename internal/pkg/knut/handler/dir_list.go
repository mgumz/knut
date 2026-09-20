// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"errors"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
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
			writeStatus(w, statusForError(err))
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

		if index, err := fsys.Open(path.Join(name, indexPage)); err == nil {
			index.Close()
			files.ServeHTTP(w, r)
			return
		}

		// the folder was opened above, not held: by now it may be gone,
		// and then it is a 404 like any other, not a server error
		infos, err := dir.Readdir(-1)
		if err != nil {
			writeStatus(w, statusForError(err))
			return
		}

		entries := make([]listEntry, 0, len(infos))
		for _, fi := range infos {
			entries = append(entries,
				newListEntry(fi.Name(), fi.Size(), fi.ModTime(), fi.IsDir()))
		}

		list := newListing(requestPath(r), entries,
			listSortFromQuery(r.URL.Query()), name != "/")

		writePage(w, listingTmpl, list)
	})
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

// requestPath returns the path as the client asked for it: by the time a
// listing is rendered, r.URL.Path has lost the prefix of its mapping.
func requestPath(r *http.Request) string {

	uri := r.RequestURI
	if uri == "" {
		return r.URL.Path
	}
	if i := strings.IndexByte(uri, '?'); i > -1 {
		uri = uri[:i]
	}
	if unescaped, err := url.PathUnescape(uri); err == nil {
		return unescaped
	}

	return uri
}
