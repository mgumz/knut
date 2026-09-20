// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// live mode is a process wide switch, so a test turning it on turns it
// off again - otherwise the next test renders pages it never asked for.
func liveMode(t *testing.T) {
	t.Helper()
	SetLive(true)
	t.Cleanup(func() { SetLive(false) })
}

// modTime dates an entry. fixed timestamps give a listing ordered by date
// exactly one right answer.
func modTime(hour int) time.Time {
	return time.Date(2026, time.September, 19, hour, 4, 0, 0, time.Local)
}

// livePace shortens the long poll: the tests wait for the same loop the
// server runs, just not for half a minute.
func livePace(t *testing.T, timeout time.Duration) {
	t.Helper()
	old := LiveTimeout
	LiveTimeout = timeout
	t.Cleanup(func() { LiveTimeout = old })
}

// in which order a filesystem hands a folder over is not a change
func TestLiveStateIgnoresOrder(t *testing.T) {

	mod := modTime(12)
	entries := []ListEntry{
		NewListEntry("a.txt", 1, mod, false),
		NewListEntry("b.txt", 2, mod, false),
		NewListEntry("sub", 0, mod, true),
	}
	shuffled := []ListEntry{entries[2], entries[0], entries[1]}

	if LiveState(entries) != LiveState(shuffled) {
		t.Error("the same folder in another order is reported as changed")
	}
	if LiveState(entries) == LiveState(entries[:2]) {
		t.Error("a folder missing an entry is reported as unchanged")
	}
	if LiveState(entries[:1]) == LiveState([]ListEntry{NewListEntry("a.txt", 2, mod, false)}) {
		t.Error("an entry which grew is reported as unchanged")
	}
}

// a folder which cannot be watched must not swallow the request: the
// client is answered and comes back on its own terms
func TestWatchListingWithoutAWatchAnswersAtOnce(t *testing.T) {

	livePace(t, time.Minute)

	gone := filepath.Join(t.TempDir(), "not-there")
	read := func() ([]ListEntry, string, error) { return nil, "state", nil }

	done := make(chan struct{})
	go func() {
		watchListing(context.Background(), gone, read, nil, "state")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a listing of an unwatchable folder was held anyway")
	}
}
