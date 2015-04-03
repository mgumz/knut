// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"compress/gzip"
	"net/http"
)

func gzHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gzw, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
		defer gzw.Close()
		next.ServeHTTP(&gzWriter{ResponseWriter: w, gz: gzw}, r)
	})
}

type gzWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (gzw *gzWriter) Header() http.Header            { return gzw.ResponseWriter.Header() }
func (gzw *gzWriter) WriteHeader(status int)         { gzw.ResponseWriter.WriteHeader(status) }
func (gzw *gzWriter) Write(data []byte) (int, error) { return gzw.gz.Write(data) }
