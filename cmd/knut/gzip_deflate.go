// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"compress/flate"
	"compress/gzip"
	"net/http"
	"strings"
)

// compressHandler requests having Accept-Encoding 'deflate' or 'gzip'
// taken from https://github.com/gorilla/handlers/blob/master/compress.go
func compressHandler(handler http.Handler) http.Handler {

	gzPool := newGzPool(gzip.DefaultCompression)
	flatePool := newFlatePool(flate.DefaultCompression)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, enc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
			switch strings.TrimSpace(enc) {
			case "deflate":
				flatePool.Compress(w, r, enc, handler)
				return
			case "gzip":
				gzPool.Compress(w, r, enc, handler)
				return
			}
		}
		handler.ServeHTTP(w, r)
	})
}
