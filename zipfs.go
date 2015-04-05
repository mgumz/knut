package main

import (
	"archive/zip"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

func zipFsHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		z, err := zip.OpenReader(name)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(os.Stderr, "error: %q: %v\n", name, err)
			return
		}
		defer z.Close()

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

		w.Header().Set("Content-Type", "text/html")
		listEntries(w, z.Reader, r.URL.Path[1:])
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

func listEntries(w http.ResponseWriter, zreader zip.Reader, prefix string) {

	fmt.Fprint(w, "<pre>")
	if prefix != "" {
		fmt.Fprintln(w, `<a href="../">..</a>`)
	}
	for _, file := range zreader.File {
		if !strings.HasPrefix(file.Name, prefix) {
			continue
		}
		if name := file.Name[len(prefix):]; name != "" {
			if path.Dir(name) != "." { // other directory
				log.Println(name, path.Dir(name))
				continue
			}
			fmt.Fprintf(w, "<a href=%q>%s</a>\n",
				html.EscapeString(file.Name), html.EscapeString(name))
		}
	}
	fmt.Fprint(w, "</pre>")
}
