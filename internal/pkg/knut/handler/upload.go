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
)

var uploadTmpl = newPageTemplate("upload", `<form method="post" enctype="multipart/form-data">
<div><input type="file" name="upload_file"></div>
<div><input type="submit" value="Upload"></div>
</form>
`)

var uploadDoneTmpl = newPageTemplate("upload-done", `<p>ok, received {{ .Size }} in {{ .Duration }}</p>
<p class="meta"><a href="">upload another file</a></p>
`)

// UploadHandler handles uploads to a given 'dir'. for method "GET" an upload-form is
// rendered, "POST" handles the actual upload
func UploadHandler(dir string) http.Handler {

	os.MkdirAll(dir, 0777)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
		case "GET", "HEAD":
			// the body is dropped by net/http for HEAD
			writePage(w, uploadTmpl, newPage("file upload"))
			return
		default:
			writeStatus(w, http.StatusMethodNotAllowed)
			return
		}

		startTime := time.Now()
		r.ParseMultipartForm(4096)

		if r.MultipartForm == nil {
			writeStatus(w, http.StatusBadRequest)
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
			page
			Size     string
			Duration string
		}{
			page:     newPage("file upload"),
			Size:     humanSize(nBytes),
			Duration: time.Since(startTime).String(),
		}
		writePage(w, uploadDoneTmpl, done)
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
