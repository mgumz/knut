// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// zipStamp dates the downloaded file, so two zips of one folder taken at
// different times do not land on each other in a download folder.
const zipStamp = "2006-01-02T15-04"

// zipRootName names the zip of a mapping published at "/": there is no
// folder name in that uri to take one from.
const zipRootName = "knut"

// zipDeflateLevel is the cheapest deflate there is: one pass over a fixed
// hash table, no lazy matching. it is what knut spends on a download - the
// levels above it look further back for every byte they pack, and the
// archive is written while it is sent, so what they cost is what the
// client waits.
const zipDeflateLevel = flate.BestSpeed

// zipFolder answers "r" with the folder "name" below "fsys" as a zip.
//
// the archive is built while it is sent: one entry after the other is read
// and handed to the writer, so nothing is staged on disk and no more of
// the tree than the file currently being copied is held in memory. the
// length of the response is unknown when the headers go out, so it is sent
// chunked - a client shows the download growing, not a percentage.
//
// the files are deflated at the cheapest level there is, see zipDeflate.
// the folder entries are stored: they have no content to pack.
//
// once the first bytes are out no status can be sent any more. what can go
// wrong from there on is handled where it happens, see zipTree.
func zipFolder(w http.ResponseWriter, r *http.Request, fsys http.FileSystem, name string) {

	base := zipBaseName(r)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", zipDisposition(base))

	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, zipDeflate)

	if err := zipTree(zw, fsys, name, base+"/"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: writing zip of %q: %v\n", name, err)
		return
	}
	if err := zw.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: finishing zip of %q: %v\n", name, err)
	}
}

// zipBaseName is what the download is named after: the last segment of the
// uri it was asked for - the folder for a folder inside a tree, the prefix
// of the mapping for the root of one - and "knut" where that uri is "/".
func zipBaseName(r *http.Request) string {

	base := path.Base(view.RequestPath(r))
	if base == "/" || base == "." {
		return zipRootName
	}

	return base
}

// zipDisposition offers the archive as a file named after the folder and
// the moment it was taken.
//
// a folder name is whatever the filesystem accepts, which is more than a
// header field does: mime spells out the ones needing it as RFC 2231, and
// a name it cannot spell at all leaves the client to pick one from the
// uri.
func zipDisposition(base string) string {

	name := base + "-" + time.Now().Format(zipStamp) + ".zip"

	disposition := mime.FormatMediaType("attachment",
		map[string]string{"filename": name})
	if disposition == "" {
		return "attachment"
	}

	return disposition
}

// zipTree writes the folder "dir" below "fsys" into "zw", naming what it
// finds below "prefix". it walks the tree itself rather than reaching for
// filepath.Walk: what is published is an http.FileSystem, which is a tree
// on disk in every mapping knut builds today, but need not be one.
//
// folders are written as entries of their own, so an empty one survives
// the round trip. anything which is neither a folder nor a regular file -
// a symlink, a socket, a device - is skipped: it has no content to store,
// and a link pointing at one of its own parents would make this walk
// endless.
//
// a folder or a file which cannot be opened is skipped with a warning: the
// tree is served as it is found, and the rest of it is still worth
// sending. errors handed back are the ones from writing the archive, and
// they end the response - the client is gone, or the entry begun is beyond
// repair.
func zipTree(zw *zip.Writer, fsys http.FileSystem, dir, prefix string) error {

	infos, err := readDirInfos(fsys, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: listing %q for a zip: %v\n", dir, err)
		return nil
	}

	for _, fi := range infos {

		name, sub := prefix+fi.Name(), path.Join(dir, fi.Name())

		switch {
		case fi.IsDir():
			if _, err := zipEntry(zw, name+"/", fi, true); err != nil {
				return err
			}
			if err := zipTree(zw, fsys, sub, name+"/"); err != nil {
				return err
			}
		case fi.Mode().IsRegular():
			if err := zipFile(zw, fsys, sub, name, fi); err != nil {
				return err
			}
		}
	}

	return nil
}

// zipFile copies the file "name" below "fsys" into the archive.
//
// a file which stops reading halfway is fatal to the archive, unlike one
// which cannot be opened at all: its entry is already open and holds the
// bytes read so far, and an archive carrying a truncated file which looks
// whole is worse than a download which visibly breaks off.
func zipFile(zw *zip.Writer, fsys http.FileSystem, name, entryName string, fi fs.FileInfo) error {

	file, err := fsys.Open(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: opening %q for a zip: %v\n", name, err)
		return nil
	}
	defer file.Close()

	entry, err := zipEntry(zw, entryName, fi, false)
	if err != nil {
		return err
	}

	_, err = io.Copy(entry, file)

	return err
}

// zipDeflaters holds the compressors of finished entries. a response
// writes one entry at a time, so at most one of them is in use per
// download, but a tree of many small files would otherwise build a fresh
// one per file - hash table, window and all - and throw it away again
// after a handful of bytes.
var zipDeflaters sync.Pool

// zipDeflate hands archive/zip a writer for the entry it is about to
// pack. it replaces the one the package builds itself, which deflates at
// the default level, see zipDeflateLevel.
func zipDeflate(w io.Writer) (io.WriteCloser, error) {

	if fw, ok := zipDeflaters.Get().(*flate.Writer); ok {
		fw.Reset(w)
		return &zipDeflater{fw}, nil
	}

	fw, err := flate.NewWriter(w, zipDeflateLevel)
	if err != nil {
		return nil, err
	}

	return &zipDeflater{fw}, nil
}

// zipDeflater is one entry's worth of deflating. closing it flushes what
// is left of the entry and puts the compressor back, so the next entry
// picks it up instead of building one.
type zipDeflater struct{ fw *flate.Writer }

func (d *zipDeflater) Write(p []byte) (int, error) { return d.fw.Write(p) }

func (d *zipDeflater) Close() error {

	if d.fw == nil {
		return nil
	}

	err := d.fw.Close()
	zipDeflaters.Put(d.fw)
	d.fw = nil

	return err
}

// readDirInfos reads the whole folder "name" and lets go of it again: the
// walk below descends into what it finds, and holding a handle per level
// would run a deep tree out of them.
func readDirInfos(fsys http.FileSystem, name string) ([]fs.FileInfo, error) {

	dir, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	return dir.Readdir(-1)
}
