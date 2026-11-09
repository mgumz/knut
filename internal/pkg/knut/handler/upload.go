// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut"
	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// the form takes any number of files in one POST. the live form sends one
// file per POST and reports how far each got, see view/assets/upload.js -
// this handler answers both alike.
var uploadTmpl = view.Template("upload")

var uploadDoneTmpl = view.Template("upload-done")

// UploadHandler handles uploads to a given 'dir'. for method "GET" an upload-form is
// rendered, "POST" handles the actual upload
func UploadHandler(dir string) http.Handler {

	os.MkdirAll(dir, 0777)
	sweepPartial(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
		case "GET", "HEAD":
			// the body is dropped by net/http for HEAD
			form := struct {
				view.Page
				Action string
			}{
				Page: view.PageFor(r, "file upload"),
				// the live form posts each file to it, see
				// assets/upload.js - spelled out, as this handler sits
				// at its window, not below it
				Action: view.RequestPath(r),
			}
			view.WriteFor(w, r, uploadTmpl, form)
			return
		default:
			view.Status(w, r, http.StatusMethodNotAllowed)
			return
		}

		startTime := time.Now()

		// the parts are read off the body as they arrive and each file
		// is written once, into the folder it ends up in - no copy in
		// os.TempDir() first, which is often memory (tmpfs)
		parts, err := r.MultipartReader()
		if err != nil {
			view.Status(w, r, http.StatusBadRequest)
			return
		}

		var (
			files  []uploadedFile
			nBytes int64
			failed int
			broken bool
		)
		for {
			part, err := parts.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				// the body ended before its closing boundary, most
				// likely the client went away
				slog.Warn("reading upload", "dir", dir, "err", err)
				broken = true
				break
			}
			if part.FileName() == "" { // a field, not a file
				part.Close()
				continue
			}

			file := uploadedFile{Name: filepath.Base(part.FileName())}
			n, err := storePart(dir, genNamePrefix(r.RemoteAddr, file.Name), part)
			part.Close()
			if err != nil {
				// the reason - a path, a full disk - goes to the log,
				// the client only learns which file did not make it
				slog.Warn("storing upload", "dir", dir, "file", file.Name, "err", err)
				file.Failed = true
				failed++
			} else {
				file.Size = n
				nBytes += n
			}
			files = append(files, file)
		}

		done := struct {
			view.Page
			Files    []uploadedFile
			Size     int64
			Failed   int
			Duration string
		}{
			Page:     view.PageFor(r, "file upload"),
			Files:    files,
			Size:     nBytes,
			Failed:   failed,
			Duration: time.Since(startTime).String(),
		}

		// a file which was not stored fails the whole answer: the page
		// still lists what did arrive, but the live form - it sends one
		// file per request, see assets/upload.js - goes by the code
		code := http.StatusOK
		switch {
		case broken:
			code = http.StatusBadRequest
		case failed > 0:
			code = http.StatusInternalServerError
		}

		// no event is fired at a listing here: the upload form sits at its
		// own window, so a listing is never in the same document to hear
		// one. a listing watching the folder the bytes landed in is held
		// at the server and answers on its own, within a tick.
		view.WriteStatusFor(w, r, code, uploadDoneTmpl, done)
	})
}

// uploadedFile is one file of a POST as the done page lists it: the name
// it was sent under, not the one it is stored as.
type uploadedFile struct {
	Name   string
	Size   int64
	Failed bool
}

func genNamePrefix(hostport, name string) string {
	host, port, _ := net.SplitHostPort(hostport)
	ip := net.ParseIP(host)
	base := filepath.Base(name)
	return fmt.Sprintf("%x_%s_%s_", ip, port, base)
}

// storePart writes "part" into knut.UploadPartialDir below "dir" and,
// once all of it is there, renames it into "dir". the rename stays on one
// filesystem, so it is atomic: a listing of "dir" sees the whole file or
// nothing. a part which breaks off is removed, nothing of it is kept.
func storePart(dir, prefix string, part io.Reader) (int64, error) {

	partial := filepath.Join(dir, knut.UploadPartialDir)
	// Mkdir, not MkdirAll: an upload folder which went away is an error,
	// not something to bring back from here
	if err := os.Mkdir(partial, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return 0, err
	}

	tmp, err := os.CreateTemp(partial, prefix)
	if err != nil {
		return 0, err
	}

	n, err := io.Copy(tmp, part)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}

	final := filepath.Join(dir, filepath.Base(tmp.Name()))
	if err == nil {
		// the name is unique below "partial", not necessarily next to
		// it - a rename would replace whatever carries it there
		if _, serr := os.Lstat(final); serr == nil {
			err = fmt.Errorf("%s exists", final)
		}
	}
	if err == nil {
		err = os.Rename(tmp.Name(), final)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return n, err
	}

	return n, nil
}

// partialMaxAge is how long a file below knut.UploadPartialDir may go
// unchanged before it is taken for the rest of an upload which never
// finished: one still arriving is written to all the time.
const partialMaxAge = time.Hour

// sweepPartial removes what uploads cut off by a stopped knut left behind
// below knut.UploadPartialDir.
func sweepPartial(dir string) {

	partial := filepath.Join(dir, knut.UploadPartialDir)
	entries, err := os.ReadDir(partial)
	if err != nil {
		return
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || time.Since(info.ModTime()) < partialMaxAge {
			continue
		}
		if err := os.Remove(filepath.Join(partial, entry.Name())); err != nil {
			slog.Warn("removing a stale upload", "dir", partial, "file", entry.Name(), "err", err)
		}
	}
}
