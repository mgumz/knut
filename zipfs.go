package main

import (
	"archive/zip"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
)

// zipFSHandler provides access to the contents of the .zip file
// specified by "name".
func zipFSHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		z, err := zip.OpenReader(name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(os.Stderr, "error: %q: %v\n", name, err)
			return
		}
		defer z.Close()

		// NOTE: i tried golang.org/x/tools/godoc/vfs/zipfs but it did not
		// list the contents of the top directory, neither for "/" nor for "."
		// nor "". zipfs would also introduce an external dependency; as long as
		// i can get by without 3rd party stuff it's ok to use something as
		// simple as this here.

		if r.URL.Path != "/" {
			for _, file := range z.File {
				if r.URL.Path[1:] != file.Name {
					continue
				}
				if file.Mode().IsRegular() {
					serveZipEntry(w, file)
					return
				}
				break
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf8")
		indexFolderEntries(w, z.Reader, r.URL.Path[1:])
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
	io.Copy(w, zr)
}

// indexFolderEntries creates an index pages page of all the file entries
// in the given "folder"
func indexFolderEntries(w http.ResponseWriter, zreader zip.Reader, folder string) {

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

func listFolderEntries(zreader zip.Reader, folder string) []string {
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
