// Copyright 2016 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

func zipHandler(dir, prefix string, store bool) http.Handler {

	if dir == "" { // "zip://." yields "" after url.Parse()
		dir = "."
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := zipDirectory(w, dir, prefix, store); err != nil {
			fmt.Fprintf(os.Stderr, "warning: creating zip of %q: %v\n", dir, err)
		}
	})
}

// zipDirectory creates a .zip from "dir" and writes it to
// "w". it also prepends "prefix" to each name.
func zipDirectory(w io.Writer, dir, prefix string, store bool) error {

	zw := zip.NewWriter(w)
	defer zw.Close()

	zipWalker := func(path string, fi os.FileInfo, err error) error {

		if fi.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: problem open file %s for reading: %v\n",
				path, err)
			return nil
		}
		defer f.Close()

		name := zipName(path, dir, prefix)
		entry, err := zipEntry(zw, name, fi, store)

		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: can't create entry for zip: %v\n",
				err)
			return nil
		}

		io.Copy(entry, f)

		return nil
	}

	return filepath.Walk(dir, zipWalker)
}

func zipName(name, dir, prefix string) string {

	name, _ = filepath.Rel(dir, name)
	base := filepath.Base(dir)
	if base == "." {
		base = ""
	}
	return path.Join(prefix+base, name)
}

func zipEntry(zw *zip.Writer, name string, fi os.FileInfo, store bool) (io.Writer, error) {

	fh, err := zip.FileInfoHeader(fi)
	if err != nil {
		return nil, err
	}

	fh.Name = name
	if store {
		fh.Method = zip.Store
	} else {
		fh.Method = zip.Deflate
	}

	return zw.CreateHeader(fh)
}
