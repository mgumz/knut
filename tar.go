// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// tarHandler creates a tar-archive from 'dir' on the fly and
// writes it to 'w'
func tarHandler(dir, prefix string) http.Handler {

	if dir == "" { // "tar://." yields "" after url.Parse()
		dir = "."
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := tarDirectory(w, dir, prefix); err != nil {
			fmt.Fprintf(os.Stderr, "warning: creating tar of %q: %v\n", dir, err)
		}
	})
}

// tarDirectory creates a .tar from "dir" and writes it to
// "w". it also prepends "prefix" to each name.
func tarDirectory(w io.Writer, dir, prefix string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	tarWalker := func(path string, info os.FileInfo, err error) error {

		entry := &tarEntry{tar: tw}
		entry.GetHeader(info)
		entry.SetName(prefix + path)
		entry.WriteHeader()
		entry.TarFileEventually(path)

		return entry.err
	}

	return filepath.Walk(dir, tarWalker)
}

type tarEntry struct {
	tar    *tar.Writer
	header *tar.Header
	err    error
}

func (entry *tarEntry) GetHeader(fi os.FileInfo) { entry.header, entry.err = tar.FileInfoHeader(fi, "") }
func (entry *tarEntry) SetName(name string) {
	if entry.err != nil {
		return
	}
	entry.header.Name = name
}

func (entry *tarEntry) WriteHeader() {
	if entry.err != nil {
		return
	}
	entry.err = entry.tar.WriteHeader(entry.header)
}
func (entry *tarEntry) TarFileEventually(name string) {
	if entry.err != nil {
		return
	}

	tf := entry.header.Typeflag
	if tf == tar.TypeReg || tf == tar.TypeRegA || tf == tar.TypeLink {
		entry.TarFile(name)
	}
}

func (entry *tarEntry) TarFile(name string) {
	if entry.err != nil {
		return
	}
	var file *os.File
	if file, entry.err = os.Open(name); entry.err != nil {
		return
	}
	_, entry.err = io.Copy(entry.tar, file)
	defer file.Close()
}
