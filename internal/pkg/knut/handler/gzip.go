// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func GzHandler(next http.Handler, clevel string) http.Handler {

	level, err := atoiInRange(clevel, -1, gzip.BestCompression, gzip.DefaultCompression)
	if err != nil {
		log.Printf("warning: scanning compression level: %v", err)
	}

	gzPool := newGzPool(level)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gzw := gzPool.Get().(*gzip.Writer)
		gzw.Reset(w)
		defer func() {
			gzw.Flush()
			gzPool.Put(gzw)
		}()
		next.ServeHTTP(&cWriter{ResponseWriter: w, Writer: gzw}, r)
	})
}

func atoiInRange(a string, min, max, fallback int) (int, error) {
	if a == "" {
		return fallback, nil
	}

	n, err := strconv.Atoi(a)
	if err != nil {
		return fallback, fmt.Errorf("scanning %q: %v", a, err)
	}

	if n < min || n > max {
		return fallback, fmt.Errorf("%q out of range (%d,%d)", a, min, max)
	}
	return n, nil
}
