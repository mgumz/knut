// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

		r.ParseMultipartForm(4096)

		if r.MultipartForm == nil {
			view.Status(w, r, http.StatusBadRequest)
			return
		}

		var (
			files  []uploadedFile
			nBytes int64
			failed int
		)
		for _, fhs := range r.MultipartForm.File {
			for _, fh := range fhs {
				prefix := genNamePrefix(r.RemoteAddr, fh.Filename)
				n, err := storeFormFile(prefix, dir, fh)
				file := uploadedFile{Name: filepath.Base(fh.Filename)}
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
		if failed > 0 {
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

func storeFormFile(prefix, dir string, fh *multipart.FileHeader) (int64, error) {

	postedFile, err := fh.Open()
	if err != nil {
		return 0, err
	}
	defer postedFile.Close()

	osFile, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return 0, err
	}
	defer osFile.Close()
	return io.Copy(osFile, postedFile)
}
