// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package fswatch

import (
	"os"
	"path/filepath"
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

	first, dropFirst, err := Folder(dir)
	if err != nil {
		t.Fatalf("watching %q: %v", dir, err)
	}
	second, dropSecond, err := Folder(dir)
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

	_, drop, err := Folder(dir)
	if err != nil {
		t.Fatalf("watching %q: %v", dir, err)
	}
	_, keep, err := Folder(dir)
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

// several folders report on one channel, a folder which cannot be watched
// is left out, and leaving takes every one of them down
func TestWatchFolders(t *testing.T) {

	one, two := t.TempDir(), t.TempDir()
	gone := filepath.Join(one, "gone")

	events, drop, err := Folders(one, gone, two)
	if err != nil {
		t.Fatalf("watching: %v", err)
	}

	for _, dir := range []string{one, two} {
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), nil, 0600); err != nil {
			t.Fatalf("writing into %q: %v", dir, err)
		}
		select {
		case <-events:
		case <-time.After(2 * time.Second):
			t.Errorf("a change in %q was not reported", dir)
		}
		// the write into the next folder is the one to wait for
		time.Sleep(50 * time.Millisecond)
		select {
		case <-events:
		default:
		}
	}

	drop()
	for _, dir := range []string{one, two} {
		if got := subscribers(dir); got != 0 {
			t.Errorf("%q outlived the subscription, %d left", dir, got)
		}
	}
}

// nothing which can be watched is an error, not a channel nobody writes to
func TestWatchFoldersNone(t *testing.T) {

	gone := filepath.Join(t.TempDir(), "gone")

	if _, _, err := Folders(gone); err == nil {
		t.Error("watching a folder which is not there did not fail")
	}
	if _, _, err := Folders(); err == nil {
		t.Error("watching no folder at all did not fail")
	}
}
