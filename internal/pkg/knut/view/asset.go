// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "embed"

	"github.com/mgumz/knut/internal/pkg/knut"
)

// htmxGz is the vendored htmx, gzipped. it is embedded in the shape it
// goes out over the wire in - see vendor_htmx.go for where it comes from.
//
//go:embed assets/htmx.min.js.gz
var htmxGz []byte

// AssetHandler answers the reserved htmx uri and the reserved query of a
// page holding on to knut, see alive. everything else goes to "next".
//
// it belongs above CompressHandler in the chain: the asset is embedded
// gzipped and is handed out that way, gzipping it a second time would cost
// cpu and produce a body no browser can read.
func AssetHandler(next http.Handler) http.Handler {

	// the asset only changes with the binary, so its version tags it
	etag := `"htmx-` + htmxVersion + `"`

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodGet && r.URL.Query().Has(knut.LiveAliveQuery) {
			alive(w, r)
			return
		}

		if r.URL.Path != knut.LiveAssetURI {
			next.ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead:
		default:
			Status(w, r, http.StatusMethodNotAllowed)
			return
		}

		header := w.Header()
		header.Set("Content-Type", "text/javascript; charset=utf-8")
		header.Set("Etag", etag)
		header.Set("Cache-Control", "public, max-age=86400")
		header.Set("Vary", "Accept-Encoding")

		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		if acceptsGzip(r) {
			header.Set("Content-Encoding", "gzip")
			header.Set("Content-Length", strconv.Itoa(len(htmxGz)))
			w.Write(htmxGz)
			return
		}

		js := htmxJS()
		header.Set("Content-Length", strconv.Itoa(len(js)))
		w.Write(js)
	})
}

// htmxJS unpacks the embedded asset for the rare client which does not
// take gzip. done once, the result is held - it is the same 50 kb the
// binary would carry uncompressed anyway.
var htmxJS = sync.OnceValue(func() []byte {

	gz, err := gzip.NewReader(bytes.NewReader(htmxGz))
	if err != nil {
		return nil
	}
	defer gz.Close()

	js, err := io.ReadAll(io.LimitReader(gz, htmxSizeRaw))
	if err != nil {
		return nil
	}

	return js
})

// acceptsGzip reports whether "r" takes a gzipped body.
func acceptsGzip(r *http.Request) bool {
	for _, enc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if name, _, _ := strings.Cut(enc, ";"); strings.TrimSpace(name) == "gzip" {
			return true
		}
	}
	return false
}

// alive holds "r" until LiveTimeout is up, and answers with the element
// which asks again: a page with nothing to watch keeps one request open
// this way, and a knut which goes away breaks it - which is what shows
// the bar saying knut is gone, see the layout.
func alive(w http.ResponseWriter, r *http.Request) {

	deadline := time.NewTimer(LiveTimeout)
	defer deadline.Stop()

	select {
	case <-r.Context().Done():
		return
	case <-deadline.C:
	}

	w.Header().Set("Cache-Control", "no-store")
	Write(w, aliveTmpl, knut.LiveAliveQuery)
}

var aliveTmpl = Block("alive")

func aliveQuery() string { return knut.LiveAliveQuery }
