// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
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
				w.Header().Set("Content-Type", "text/html; charset=utf8")
				indexFolderEntries(w, &z.Reader, name)
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

		http.NotFound(w, r)
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

// indexFolderEntries creates an index pages page of all the file entries
// in the given "folder"
func indexFolderEntries(w http.ResponseWriter, zreader *zip.Reader, folder string) {

	fmt.Fprint(w, "<pre>")
	defer fmt.Fprint(w, "</pre>")
	if folder != "" {
		fmt.Fprintln(w, `<a href="../">..</a>`)
	}

	for _, entry := range listFolderEntries(zreader, folder) {
		fmt.Fprintf(w, "<a href=\"/%s\">%s</a>\n",
			html.EscapeString(entry),
			html.EscapeString(entry[len(folder):]))
	}
}

func listFolderEntries(zreader *zip.Reader, folder string) []string {
	entries := make([]string, 0)
	for _, file := range zreader.File {

		// skip entries not children of 'folder'
		if !strings.HasPrefix(file.Name, folder) {
			continue
		}

		// only direct children of 'folder'
		if name := file.Name[len(folder):]; name != "" &&
			strings.Count(path.Clean(name), "/") == 0 {
			entries = append(entries, file.Name)
		}
	}
	sort.Strings(entries)
	return entries
}
