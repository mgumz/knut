// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// ZipFSHandler provides access to the contents of the .zip file
// specified by "name".
//
// if "prefix" is applied to all requests, eg: a "/foo/bar" request is tried
// to find as "/prefix/foo/bar" in the zip file.
//
// if the requested path is a folder, use the "index" in that folder to
// to render the folder entries.
func ZipFSHandler(name, prefix, index string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// a stripped window leaves the path without its leading "/"
		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}

		// NOTE: yes, we open the zip for every request. this allows to
		// keep *knut* running and deliver trees while the the underlaying
		// zip gets replaced.
		z, err := zip.OpenReader(name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(os.Stderr, "error: %q: %v\n", name, err)
			return
		}
		defer z.Close()

		// handle folders
		if strings.HasSuffix(r.URL.Path, "/") {
			if index == "" {
				folder := path.Join(prefix, r.URL.Path[1:])
				if folder != "" {
					folder += "/"
				}
				indexFolderEntries(w, r, &z.Reader, folder)
				return
			}
			r.URL.Path = path.Join(r.URL.Path, index)
		}

		name := path.Join(prefix, r.URL.Path[1:])
		for _, file := range z.File {
			if name != file.Name {
				continue
			}
			if file.Mode().IsRegular() {
				serveZipEntry(w, file)
				return
			}
			break
		}

		writeStatus(w, http.StatusNotFound)
	})
}

func serveZipEntry(w http.ResponseWriter, zFile *zip.File) {

	zr, err := zFile.Open()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%v", err)
		return
	}
	defer zr.Close()

	// read the first 512 bytes to detect the content type. then push these
	// 512 bytes to the remote and send the remaining rest.
	buf := bytes.NewBuffer(make([]byte, 0, 512))
	_, err = io.CopyN(buf, zr, 512)
	if err != nil && err != io.EOF {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "%v", err)
		return
	}

	// detect the content-type by suffix first, if
	// if that fails, try to guess.
	ctype := mime.TypeByExtension(path.Ext(zFile.Name))
	if ctype == "" {
		ctype = http.DetectContentType(buf.Bytes())
	}
	w.Header().Set("Content-Type", ctype)

	w.Write(buf.Bytes())
	io.Copy(w, zr)
}

// indexFolderEntries renders the entries of "folder" the same way a folder
// on disk is listed: sortable, with sizes and modification times.
func indexFolderEntries(w http.ResponseWriter, r *http.Request, zreader *zip.Reader, folder string) {

	list := newListing(requestPath(r), listFolderEntries(zreader, folder),
		listSortFromQuery(r.URL.Query()), folder != "")

	writePage(w, listingTmpl, list)
}

// listFolderEntries collects the direct children of "folder". zips do not
// have to carry entries for their folders, so the folders in between are
// picked up from the names of the entries below them.
func listFolderEntries(zreader *zip.Reader, folder string) []listEntry {

	entries, seen := make([]listEntry, 0), map[string]bool{}
	for _, file := range zreader.File {

		// skip entries not children of 'folder'
		if !strings.HasPrefix(file.Name, folder) {
			continue
		}

		name := file.Name[len(folder):]
		if name == "" {
			continue
		}

		isDir, size, mod := strings.HasSuffix(name, "/"),
			int64(file.UncompressedSize64), file.Modified

		// a name still carrying a separator belongs to a subfolder. that
		// subfolder is the entry to list, and the timestamp of a child
		// says nothing about it.
		if i := strings.IndexByte(strings.TrimSuffix(name, "/"), '/'); i > -1 {
			name, isDir, size, mod = name[:i], true, 0, time.Time{}
		}
		name = strings.TrimSuffix(name, "/")

		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		entries = append(entries, newListEntry(name, size, mod, isDir))
	}

	return entries
}
