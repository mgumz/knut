// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// folders is the one watcher knut runs, however many clients are looking.
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

// watchFolder reports changes in "dir" on the returned channel until the
// returned func is called.
//
// the channel carries no event: it says "look again", and the caller is
// the one who knows what a change means. it is buffered by one and never
// blocks the pump - two wakeups nobody picked up are one wakeup.
func watchFolder(dir string) (<-chan struct{}, func(), error) {
	return folders.subscribe(dir)
}

func (fw *folderWatch) subscribe(dir string) (<-chan struct{}, func(), error) {

	dir = filepath.Clean(dir)

	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.fs == nil {
		fs, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, nil, err
		}
		fw.fs = fs
		go fw.pump(fs)
	}

	if _, watched := fw.subs[dir]; !watched {
		if err := fw.fs.Add(dir); err != nil {
			return nil, nil, err
		}
		fw.subs[dir] = map[chan struct{}]bool{}
	}

	events := make(chan struct{}, 1)
	fw.subs[dir][events] = true

	return events, func() { fw.unsubscribe(dir, events) }, nil
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
