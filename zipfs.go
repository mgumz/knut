package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
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
		zreader := zipReader{ReadCloser: z}
		zipHandler := http.FileServer(&zreader)
		zipHandler.ServeHTTP(w, r)
	})
}

type zipReader struct { // represents a http.Filesystem
	*zip.ReadCloser
}

func (zr *zipReader) Open(name string) (http.File, error) {
	// TODO: / -> just list the top-level contents
	//       /match.txt -> just the file
	//       /folder/ -> all the entries that are directly subentries

	log.Printf("zipReader.Open(%q)", name)
	for i, zfile := range zr.ReadCloser.File {
		log.Printf("%d: %s", i, zfile.Name)
		if name[1:] == zfile.Name {
			break
		}
	}
	return nil, errors.New("zipReader.Open(): not yet implemented")
}

type zipEntry struct { // represents a http.File
	zip.File
	offset int64
	whence int
}

func (ze *zipEntry) Stat() (os.FileInfo, error) {
	return ze.File.FileInfo(), nil
}

func (ze *zipEntry) Seek(offset int64, whence int) (int64, error) {
	ze.offset, ze.whence = offset, whence
	return ze.offset, nil
}

func (ze *zipEntry) Readdir(count int) ([]os.FileInfo, error) {
	if ze.Mode().IsDir() {

	}
	// TODO: if ze.isDir() -> yield entries
	return nil, nil
}
