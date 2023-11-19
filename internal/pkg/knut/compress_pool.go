// Copyright 2016 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"sync"
)

type compressPoolEntry interface {
	Write(data []byte) (int, error)
	Flush() error
	Reset(io.Writer)
}

type compressPool struct{ sync.Pool }

func newGzPool(level int) *compressPool {
	cp := &compressPool{}
	cp.Pool = sync.Pool{
		New: func() interface{} {
			w, _ := gzip.NewWriterLevel(nil, level)
			return w
		},
	}
	return cp
}

func newFlatePool(level int) *compressPool {
	cp := &compressPool{}
	cp.Pool = sync.Pool{
		New: func() interface{} {
			w, _ := flate.NewWriter(nil, level)
			return w
		},
	}
	return cp
}

func (cp *compressPool) Compress(w http.ResponseWriter, r *http.Request,
	enc string, handler http.Handler) {

	compressor := cp.Get().(compressPoolEntry)
	compressor.Reset(w)
	defer func() {
		compressor.Flush()
		cp.Put(compressor)
	}()

	w.Header().Set("Content-Encoding", enc)
	w = &cWriter{Writer: compressor, ResponseWriter: w}
	handler.ServeHTTP(w, r)
}

type cWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w *cWriter) WriteHeader(code int)           { w.ResponseWriter.WriteHeader(code) }
func (w *cWriter) Write(data []byte) (int, error) { return w.Writer.Write(data) }
func (w *cWriter) Header() http.Header            { return w.ResponseWriter.Header() }
