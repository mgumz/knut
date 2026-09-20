// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"context"
	"encoding/binary"
	"hash/fnv"
	"io"
	"strconv"
	"time"

	"github.com/mgumz/knut/internal/pkg/knut/fswatch"
)

// liveParam carries the state of the listing a client currently shows.
// a request carrying it is a poll: knut holds it until the folder differs
// from that state.
const liveParam = "live"

// how long a client is held before it is sent home to come back. a var,
// not a const - the tests run the same loop on a much shorter leash.
//
// nothing else paces a poll: the folder is looked at when the filesystem
// says it changed, not on a clock.
var LiveTimeout = 30 * time.Second

// LiveState fingerprints what a listing shows. a client hands the state of
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
// one is out of the question: there, a file rewritten to the same size
// under the same timestamp reads as unchanged.
func LiveState(entries []ListEntry) string {

	sum, hash, num := uint64(0), fnv.New64a(), make([]byte, 8)
	write := func(n uint64) {
		binary.BigEndian.PutUint64(num, n)
		hash.Write(num)
	}

	for i := range entries {
		hash.Reset()
		io.WriteString(hash, entries[i].Name)
		write(uint64(entries[i].Bytes))
		write(uint64(entries[i].mod.UnixNano()))
		write(entries[i].Sig)
		sum += hash.Sum64()
	}

	return strconv.FormatUint(sum, 36)
}

// ReadListing lists a folder again, the way the handler holding it does.
type ReadListing func() ([]ListEntry, string, error)

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
func watchListing(ctx context.Context, dir string, read ReadListing, entries []ListEntry, state string) ([]ListEntry, string) {

	events, unwatch, err := fswatch.Folder(dir)
	if err != nil {
		// no watch, nothing to wait on: answer now and let the client
		// re-arm rather than hold it for half a minute for nothing
		return entries, state
	}
	defer unwatch()

	deadline := time.NewTimer(LiveTimeout)
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
