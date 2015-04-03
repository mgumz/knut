// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// compressHandler requests having Accept-Encoding 'deflate' or 'gzip'
// taken from https://github.com/gorilla/handlers/blob/master/compress.go
func compressHandler(handler http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, enc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
			switch strings.TrimSpace(enc) {
			case "deflate":
				fw, _ := flate.NewWriter(w, flate.DefaultCompression)
				defer fw.Close()
				w.Header().Set("Content-Encoding", "deflate")
				w = &cWriter{Writer: fw, ResponseWriter: w}
				goto L
			case "gzip":
				gz := gzip.NewWriter(w)
				defer gz.Close()
				w.Header().Set("Content-Encoding", "gzip")
				w = &cWriter{Writer: gz, ResponseWriter: w}
				goto L
			}
		}
	L:
		handler.ServeHTTP(w, r)
	})
}

type cWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w *cWriter) WriteHeader(code int)           { w.ResponseWriter.WriteHeader(code) }
func (w *cWriter) Write(data []byte) (int, error) { return w.Writer.Write(data) }
func (w *cWriter) Header() http.Header            { return w.ResponseWriter.Header() }
