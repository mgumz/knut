// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// subscribers returns how many clients watch "dir" right now.
func subscribers(dir string) int {
	folders.mu.Lock()
	defer folders.mu.Unlock()
	return len(folders.subs[filepath.Clean(dir)])
}

// a folder watched by two clients is one watch in the kernel, not two:
// inotify instances are a limited resource and a busy folder would
// otherwise cost one per browser tab
func TestWatchFolderSharesOneWatch(t *testing.T) {

	dir := t.TempDir()

	first, dropFirst, err := watchFolder(dir)
	if err != nil {
		t.Fatalf("watching %q: %v", dir, err)
	}
	second, dropSecond, err := watchFolder(dir)
	if err != nil {
		t.Fatalf("watching %q twice: %v", dir, err)
	}

	if got := subscribers(dir); got != 2 {
		t.Errorf("got %d subscribers, want 2", got)
	}

	// one change, both are woken
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), nil, 0600); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}
	for i, events := range []<-chan struct{}{first, second} {
		select {
		case <-events:
		case <-time.After(2 * time.Second):
			t.Errorf("subscriber %d was not woken by the change", i)
		}
	}

	dropFirst()
	if got := subscribers(dir); got != 1 {
		t.Errorf("got %d subscribers after one left, want 1", got)
	}

	dropSecond()
	if got := subscribers(dir); got != 0 {
		t.Errorf("the watch outlived its last subscriber, %d left", got)
	}
}

// dropping a subscription twice is not a reason to take the folder down
// under whoever is still watching it
func TestWatchFolderDropsOnce(t *testing.T) {

	dir := t.TempDir()

	_, drop, err := watchFolder(dir)
	if err != nil {
		t.Fatalf("watching %q: %v", dir, err)
	}
	_, keep, err := watchFolder(dir)
	if err != nil {
		t.Fatalf("watching %q twice: %v", dir, err)
	}
	defer keep()

	drop()
	drop()

	if got := subscribers(dir); got != 1 {
		t.Errorf("got %d subscribers, want the one which never left", got)
	}
}

// a folder which cannot be watched must not swallow the request: the
// client is answered and comes back on its own terms
func TestWatchListingWithoutAWatchAnswersAtOnce(t *testing.T) {

	livePace(t, time.Minute)

	gone := filepath.Join(t.TempDir(), "not-there")
	read := func() ([]listEntry, string, error) { return nil, "state", nil }

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

// an in-memory tree has no path to hand a watcher. it is listed like any
// other, it just never asks the client to come back for an answer which
// could not arrive.
func TestLiveListingWithoutAPathIsNotArmed(t *testing.T) {

	liveMode(t)

	body := get(DirListHandler(testFS()), "/").Body.String()

	if strings.Contains(body, "hx-get") {
		t.Error("a listing which cannot be watched armed a poll")
	}
	if !strings.Contains(body, "a.txt") {
		t.Error("the listing did not render its entries")
	}
}

// the same listing, asked as a poll: answered, not held
func TestLivePollWithoutAPathIsNotHeld(t *testing.T) {

	liveMode(t)
	livePace(t, time.Minute)

	done := make(chan int, 1)
	go func() { done <- get(DirListHandler(testFS()), "/?live=whatever").Code }()

	select {
	case code := <-done:
		if code != 200 {
			t.Errorf("got status %d, want 200", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a poll for an unwatchable tree was held")
	}
}
