// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

// Package fswatch reports that a folder on disk changed.
//
// it knows nothing about what lives in that folder or about what a caller
// makes of the news - it says "look again", and the caller is the one who
// decides whether anything it cares about moved.
package fswatch

import (
	"errors"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// folders is the one watcher this package runs, however many callers are
// looking.
//
// a watcher costs a kernel object - linux hands out 128 inotify instances
// per user by default - and a folder watched twice is watched twice. so
// clients subscribe to a folder instead of watching it: the kernel sees
// one watch per folder, and the last client leaving takes it down again.
var folders = &folderWatch{subs: map[string]map[chan struct{}]bool{}}

type folderWatch struct {
	mu   sync.Mutex
	fs   *fsnotify.Watcher
	subs map[string]map[chan struct{}]bool
}

// Folder reports changes in "dir" on the returned channel until the
// returned func is called.
//
// the channel carries no event: it says "look again", and the caller is
// the one who knows what a change means. it is buffered by one and never
// blocks the pump - two wakeups nobody picked up are one wakeup.
func Folder(dir string) (<-chan struct{}, func(), error) {
	return Folders(dir)
}

// Folders reports changes in any of "dirs" on one channel, the way Folder
// does for one. what one page shows can be read from several folders, and
// the caller looks again whichever of them moved.
//
// a folder which cannot be watched is left out and the rest is watched all
// the same: it is an error only when none of them can be.
func Folders(dirs ...string) (<-chan struct{}, func(), error) {

	events := make(chan struct{}, 1)
	watched, err := []string{}, error(nil)

	for _, dir := range dirs {
		dir, subErr := folders.subscribe(dir, events)
		if subErr != nil {
			err = subErr
			continue
		}
		watched = append(watched, dir)
	}

	if len(watched) == 0 {
		if err == nil {
			err = errors.New("fswatch: no folder to watch")
		}
		return nil, nil, err
	}

	return events, func() {
		for _, dir := range watched {
			folders.unsubscribe(dir, events)
		}
	}, nil
}

// subscribe hands "events" the changes of "dir" and returns the name the
// folder is kept under.
func (fw *folderWatch) subscribe(dir string, events chan struct{}) (string, error) {

	dir = filepath.Clean(dir)

	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.fs == nil {
		fs, err := fsnotify.NewWatcher()
		if err != nil {
			return "", err
		}
		fw.fs = fs
		go fw.pump(fs)
	}

	if _, watched := fw.subs[dir]; !watched {
		if err := fw.fs.Add(dir); err != nil {
			return "", err
		}
		fw.subs[dir] = map[chan struct{}]bool{}
	}

	fw.subs[dir][events] = true

	return dir, nil
}

func (fw *folderWatch) unsubscribe(dir string, events chan struct{}) {

	fw.mu.Lock()
	defer fw.mu.Unlock()

	subs, watched := fw.subs[dir]
	if !watched {
		return
	}

	delete(subs, events)
	if len(subs) == 0 {
		delete(fw.subs, dir)
		fw.fs.Remove(dir)
	}
}

// pump hands every event to whoever subscribed to the folder it happened
// in. an event names the entry which changed, so the folder watching it
// is that entry's parent - unless the watched folder itself is what moved
// or went away, and then the event names the folder.
func (fw *folderWatch) pump(fs *fsnotify.Watcher) {

	for {
		select {
		case event, ok := <-fs.Events:
			if !ok {
				return
			}
			name := filepath.Clean(event.Name)
			fw.notify(filepath.Dir(name))
			fw.notify(name)

		case _, ok := <-fs.Errors:
			if !ok {
				return
			}
			// an error says nothing about which folder it belongs to:
			// wake everyone, a re-read is what decides anyway
			fw.notifyAll()
		}
	}
}

func (fw *folderWatch) notify(dir string) {

	fw.mu.Lock()
	defer fw.mu.Unlock()

	for events := range fw.subs[dir] {
		select {
		case events <- struct{}{}:
		default:
		}
	}
}

func (fw *folderWatch) notifyAll() {

	fw.mu.Lock()
	defer fw.mu.Unlock()

	for _, subs := range fw.subs {
		for events := range subs {
			select {
			case events <- struct{}{}:
			default:
			}
		}
	}
}
