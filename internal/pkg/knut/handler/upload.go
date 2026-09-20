// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// the live form posts through htmx and reports how far the bytes got. the
// bar is honest about what it measures: it fills while the browser sends,
// so it stands at 100% while the last of it is still being written to
// disk - that is what the note next to it says.
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
				// htmx needs the uri spelled out, an empty "hx-post" is
				// no url to it - and this handler sits at its window,
				// not below it
				Action: view.RequestPath(r),
			}
			view.WriteFor(w, r, uploadTmpl, form)
			return
		default:
			view.Status(w, http.StatusMethodNotAllowed)
			return
		}

		startTime := time.Now()
		r.ParseMultipartForm(4096)

		if r.MultipartForm == nil {
			view.Status(w, http.StatusBadRequest)
			return
		}

		var nBytes int64
		for _, files := range r.MultipartForm.File {
			for _, fh := range files {
				prefix := genNamePrefix(r.RemoteAddr, fh.Filename)
				n, err := storeFormFile(prefix, dir, fh)
				if err != nil {
					log.Printf("warning: %v", err)
				}
				nBytes += n
			}
		}

		done := struct {
			view.Page
			Size     int64
			Duration string
		}{
			Page:     view.PageFor(r, "file upload"),
			Size:     nBytes,
			Duration: time.Since(startTime).String(),
		}

		// no event is fired at a listing here: the upload form sits at its
		// own window, so a listing is never in the same document to hear
		// one. a listing watching the folder the bytes landed in is held
		// at the server and answers on its own, within a tick.
		view.WriteFor(w, r, uploadDoneTmpl, done)
	})
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
