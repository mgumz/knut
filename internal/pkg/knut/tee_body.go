// Copyright 2024 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

import (
	"io"
	"net/http"
	"strings"
)

func TeeBodyHandler(next http.Handler, ow io.Writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr := io.MultiReader(strings.NewReader("\n"), r.Body, strings.NewReader("\n"))
		tr := io.TeeReader(mr, ow)
		r.Body = &teeReadCloser{tr, r.Body}
		next.ServeHTTP(w, r)
	})
}

type teeReadCloser struct {
	r  io.Reader
	rc io.ReadCloser
}

func (trc *teeReadCloser) Read(p []byte) (int, error) { return trc.r.Read(p) }
func (trc *teeReadCloser) Close() error               { return trc.rc.Close() }

func FlushBodyHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		next.ServeHTTP(w, r)
	})
}
