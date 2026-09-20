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
