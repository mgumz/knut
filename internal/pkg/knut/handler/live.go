// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/binary"
	"hash/fnv"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut"
)

// htmxGz is the vendored htmx, gzipped. it is embedded in the shape it
// goes out over the wire in - see gen_htmx.go for where it comes from.
//
//go:embed assets/htmx.min.js.gz
var htmxGz []byte

// liveParam carries the state of the listing a client currently shows.
// a request carrying it is a poll: knut holds it until the folder differs
// from that state.
const liveParam = "live"

// how long a client is held before it is sent home to come back. a var,
// not a const - the tests run the same loop on a much shorter leash.
//
// nothing else paces a poll: the folder is looked at when the filesystem
// says it changed, not on a clock.
var liveTimeout = 30 * time.Second

// live gates everything in this file. knut without "-live" renders exactly
// the html it rendered before: no script tag, no reserved uri, no polling.
var live atomic.Bool

// SetLive turns live mode on or off. it is a process wide switch, set once
// from the command line before the first request is served.
func SetLive(on bool) { live.Store(on) }

// liveEnabled reports whether live mode is on.
func liveEnabled() bool { return live.Load() }

// isFragment reports whether "r" comes from htmx. those requests get the
// content of a page alone - the layout around it is already on screen.
func isFragment(r *http.Request) bool {
	return liveEnabled() && r.Header.Get("HX-Request") == "true"
}

// LiveAssetHandler answers the reserved htmx uri, everything else goes to
// "next".
//
// it belongs above CompressHandler in the chain: the asset is embedded
// gzipped and is handed out that way, gzipping it a second time would cost
// cpu and produce a body no browser can read.
func LiveAssetHandler(next http.Handler) http.Handler {

	// the asset only changes with the binary, so its version tags it
	etag := `"htmx-` + htmxVersion + `"`

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != knut.LiveAssetURI {
			next.ServeHTTP(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet, http.MethodHead:
		default:
			writeStatus(w, http.StatusMethodNotAllowed)
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

// liveState fingerprints what a listing shows. a client hands the state of
// the listing on its screen back with every poll, so a folder which
// changed between the render and the poll is answered right away instead
// of being sat out.
// the entries are summed up instead of hashed in a row: in which order a
// filesystem hands a folder over is none of its business, a readdir
// returning the same names shuffled is not a change.
//
// name, size and timestamp are what listing a folder costs nothing extra:
// the name carries the trailing "/" of a folder, so a file replaced by a
// folder of its name is a change too. "sig" is a content fingerprint on
// top, for a source which has one to offer - a zip hands out a crc per
// entry, and folding it in is what tells two zips apart whose entries kept
// their names, sizes and timestamps.
//
// a folder on disk has no such number, and reading every file to compute
// one is out of the question at a tick per half second: there, a file
// rewritten to the same size under the same timestamp reads as unchanged.
func liveState(entries []listEntry) string {

	sum, hash, num := uint64(0), fnv.New64a(), make([]byte, 8)
	write := func(n uint64) {
		binary.BigEndian.PutUint64(num, n)
		hash.Write(num)
	}

	for i := range entries {
		hash.Reset()
		io.WriteString(hash, entries[i].Name)
		write(uint64(entries[i].bytes))
		write(uint64(entries[i].mod.UnixNano()))
		write(entries[i].sig)
		sum += hash.Sum64()
	}

	return strconv.FormatUint(sum, 36)
}

// readListing lists a folder again, the way the handler holding it does.
type readListing func() ([]listEntry, string, error)

// watchListing holds a poll until the folder it lists differs from
// "state", the client walks away or liveTimeout is up. it answers with the
// listing to render - an unchanged folder simply renders again and the
// client re-arms, which is what keeps a long poll going.
//
// "dir" is the folder on disk to watch, "entries" and "state" are what the
// caller already read - they are the answer if nothing happens.
//
// an event is a reason to look, not an answer: the filesystem reports a
// file being opened or an attribute being touched the same way it reports
// one being created, and a listing shows neither. so every wakeup is
// followed by a read, and only a fingerprint which moved ends the wait.
func watchListing(ctx context.Context, dir string, read readListing, entries []listEntry, state string) ([]listEntry, string) {

	events, unwatch, err := watchFolder(dir)
	if err != nil {
		// no watch, nothing to wait on: answer now and let the client
		// re-arm rather than hold it for half a minute for nothing
		return entries, state
	}
	defer unwatch()

	deadline := time.NewTimer(liveTimeout)
	defer deadline.Stop()

	for {
		// read first: the caller read the folder before this watch
		// existed, so anything which happened in between would otherwise
		// wait for an event which already came and went
		if next, nextState, err := read(); err == nil && nextState != state {
			return next, nextState
		}

		select {
		case <-ctx.Done():
			return entries, state
		case <-deadline.C:
			return entries, state
		case <-events:
		}
	}
}
