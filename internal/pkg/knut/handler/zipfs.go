// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// zipReread is how long a live listing trusts the stat of the zip it shows
// before opening it again whatever that stat says. a var, not a const -
// the tests run the same gate on a much shorter leash.
var zipReread = 5 * time.Second

// zipSource points an fs.FS at the folder the zip sits in and names it
// inside that folder.
//
// the zip is reached through an fs.FS rather than through its path so the
// gate in zipFolderReader can be tested against a stat which lies - the
// case a filesystem never reproduces on demand, see zipFolderReader.
func zipSource(name string) (fs.FS, string) {

	dir, base := filepath.Split(name)
	if dir == "" {
		dir = "."
	}

	return os.DirFS(dir), base
}

// openZip reads the central directory of the zip "name" inside "fsys". the
// file is handed back for the caller to close: a zip is opened for one
// read and closed right after, nothing holds on to it.
func openZip(fsys fs.FS, name string) (*zip.Reader, io.Closer, error) {

	file, err := fsys.Open(name)
	if err != nil {
		return nil, nil, err
	}

	reader, ok := file.(io.ReaderAt)
	if !ok {
		file.Close()
		return nil, nil, fmt.Errorf("%s: cannot be read at an offset", name)
	}

	fi, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	z, err := zip.NewReader(reader, fi.Size())
	if err != nil {
		file.Close()
		return nil, nil, err
	}

	return z, file, nil
}

// ZipFSHandler provides access to the contents of the .zip file
// specified by "name".
//
// if "prefix" is applied to all requests, eg: a "/foo/bar" request is tried
// to find as "/prefix/foo/bar" in the zip file.
//
// if the requested path is a folder, use the "index" in that folder to
// to render the folder entries.
func ZipFSHandler(name, prefix, index string) http.Handler {

	fsys, zipName := zipSource(name)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// a stripped window leaves the path without its leading "/"
		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}

		// a file is as worth scanning as a folder, so "?qr" is answered
		// before the two part ways
		if r.URL.Query().Has(view.QueryQR) {
			zipEntryQR(w, r, fsys, zipName, name, path.Join(prefix, r.URL.Path[1:]))
			return
		}

		// "?zip" is asked of the folder itself, so it is answered before
		// anything which decides what the folder looks like. a file asked
		// for it is served as it is, like on disk.
		if r.URL.Query().Has(view.QueryZip) && strings.HasSuffix(r.URL.Path, "/") {
			zipFSFolder(w, r, fsys, zipName, name, path.Join(prefix, r.URL.Path[1:]))
			return
		}

		// a folder without an index is listed. that listing opens the zip
		// on its own - it may be held as a live poll and must not sit on
		// an open handle while it is, see indexFolderEntries.
		if strings.HasSuffix(r.URL.Path, "/") && index == "" {
			folder := path.Join(prefix, r.URL.Path[1:])
			if folder != "" {
				folder += "/"
			}
			indexFolderEntries(w, r, fsys, zipName, name, folder)
			return
		}

		// NOTE: yes, we open the zip for every request. this allows to
		// keep *knut* running and deliver trees while the the underlaying
		// zip gets replaced.
		z, file, err := openZip(fsys, zipName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(os.Stderr, "error: %q: %v\n", name, err)
			return
		}
		defer file.Close()

		// a folder shadowed by an index: serve that index
		if strings.HasSuffix(r.URL.Path, "/") {
			r.URL.Path = path.Join(r.URL.Path, index)
		}

		entry := path.Join(prefix, r.URL.Path[1:])
		for _, file := range z.File {
			if entry != file.Name {
				continue
			}
			if file.Mode().IsRegular() {
				serveZipEntry(w, file)
				return
			}
			break
		}

		view.Status(w, http.StatusNotFound)
	})
}

// zipEntryQR answers "r" with the code of "entry" inside the zip at "name",
// a 404 if the zip carries no such entry. "reported" is the zip as the
// user spelled it, the one an error names.
func zipEntryQR(w http.ResponseWriter, r *http.Request, fsys fs.FS, name, reported, entry string) {

	z, file, err := openZip(fsys, name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "error: %q: %v\n", reported, err)
		return
	}
	defer file.Close()

	if !zipHas(z, entry) {
		view.Status(w, http.StatusNotFound)
		return
	}

	view.QRImage(w, r)
}

// zipHas says whether "entry" is a file or a folder inside the zip. a
// folder need not have an entry of its own, a name below it is enough, see
// listFolderEntries. "" is the root of the zip, which is always there.
func zipHas(zreader *zip.Reader, entry string) bool {

	if entry == "" {
		return true
	}

	for _, file := range zreader.File {
		if file.Name == entry || strings.HasPrefix(file.Name, entry+"/") {
			return true
		}
	}

	return false
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

// indexFolderEntries renders the entries of "folder" inside the zip at
// "name" the same way a folder on disk is listed: sortable, with sizes and
// modification times, and under -live polling itself.
//
// it does watch: a zip carries the entries it was written with, but the
// zip this handler serves is a path, and that path is meant to be replaced
// underneath a running knut - the reason the zip is opened per request in
// the first place.
// "name" is the zip inside "fsys", "reported" the same zip as the user
// spelled it - that is what an error names.
//
// what is watched is the folder the zip sits in, not the zip: a zip is
// replaced by writing a new one next to it and moving it over, and the
// file the watch was put on is the one which just got unlinked.
func indexFolderEntries(w http.ResponseWriter, r *http.Request, fsys fs.FS, name, reported, folder string) {
	opts := view.ListOpts{Dir: filepath.Dir(reported), Parent: folder != "", Zip: true, QR: true}

	if err := view.Listing(w, r, zipFolderReader(fsys, name, folder), opts); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "error: %q: %v\n", reported, err)
	}
}

// zipFolderReader lists "folder" inside the zip at "name", over and over
// for as long as a poll is held.
//
// the zip is opened for the read and closed again right away: holding an
// open handle across a poll is what would get in the way of the
// replacement it watches for - on windows an open handle refuses it
// outright.
//
// a re-read means parsing the whole central directory, which for a zip of
// any size is far more than a folder on disk costs. so a tick pays for it
// only when the zip looks moved: size and modification time are what a
// replacement changes, an untouched zip is answered from the last read.
//
// a stat is not a guarantee though, and the ways it lies all point the
// same way:
//
//   - a filesystem keeping modification times by the second hides a zip
//     replaced by one of exactly its size within that second
//   - over nfs a stat is answered from the attribute cache for as long as
//     acregmin..acregmax say, while an open revalidates - the cheap check
//     is the stale one there, of all places
//
// so the gate only ever postpones a read, it never cancels one: every
// zipReread the zip is opened whatever the stat claims. that bounds how
// stale a listing can get on any filesystem, without parsing the central
// directory twice a second for a zip nobody touches.
func zipFolderReader(fsys fs.FS, name, folder string) view.ReadListing {

	var (
		size    int64
		mod     time.Time
		last    time.Time
		entries []view.ListEntry
		state   string
	)

	return func() ([]view.ListEntry, string, error) {

		fi, err := fs.Stat(fsys, name)
		if err != nil {
			return nil, "", err
		}

		gated := entries != nil && time.Since(last) < zipReread
		if gated && fi.Size() == size && fi.ModTime().Equal(mod) {
			return entries, state, nil
		}

		z, file, err := openZip(fsys, name)
		if err != nil {
			return nil, "", err
		}
		defer file.Close()

		// stat before open: a zip replaced in between is recorded under
		// the timestamp of the one before it, so the next tick sees a
		// difference and reads again. one tick late beats missing it.
		size, mod, last = fi.Size(), fi.ModTime(), time.Now()
		entries = listFolderEntries(z, folder)
		state = view.LiveState(entries)

		return entries, state, nil
	}
}

// listFolderEntries collects the direct children of "folder". zips do not
// have to carry entries for their folders, so the folders in between are
// picked up from the names of the entries below them.
func listFolderEntries(zreader *zip.Reader, folder string) []view.ListEntry {

	entries, seen := make([]view.ListEntry, 0), map[string]bool{}
	for _, file := range zreader.File {

		// skip entries not children of 'folder'
		if !strings.HasPrefix(file.Name, folder) {
			continue
		}

		name := file.Name[len(folder):]
		if name == "" {
			continue
		}

		// the crc a zip carries per entry is the content fingerprint a
		// folder on disk cannot offer, see liveState
		isDir, size, mod, sig := strings.HasSuffix(name, "/"),
			int64(file.UncompressedSize64), file.Modified, uint64(file.CRC32)

		// a name still carrying a separator belongs to a subfolder. that
		// subfolder is the entry to list, and neither the timestamp nor
		// the content of a child says anything about it.
		if i := strings.IndexByte(strings.TrimSuffix(name, "/"), '/'); i > -1 {
			name, isDir, size, mod, sig = name[:i], true, 0, time.Time{}, 0
		}
		name = strings.TrimSuffix(name, "/")

		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		entry := view.NewListEntry(name, size, mod, isDir)
		if !isDir {
			entry.Sig = sig
		}
		entries = append(entries, entry)
	}

	return entries
}
