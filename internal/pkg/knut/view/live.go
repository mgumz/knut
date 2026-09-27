// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import "sync/atomic"

// live is the "-live" flag, once the command line has been read.
//
// it is not a component of its own - it is a switch three layers consult:
// the layout asks it whether to pull htmx in (page.go), a listing asks it
// whether to arm a poll (listing.go), and the uri of the asset is only
// answered while it is on (htmx.go). knut without it renders exactly the
// html it rendered before: no script tag, no reserved uri, no watcher.
var live atomic.Bool

// SetLive turns live mode on or off. it is a process wide switch, set once
// from the command line before the first request is served.
func SetLive(on bool) { live.Store(on) }

// liveEnabled reports whether live mode is on.
func liveEnabled() bool { return live.Load() }

// index says the index of the mappings is served at "/", the page the
// wordmark in the header leads to. set once, like live.
var index atomic.Bool

// SetIndex tells the pages the index is served.
func SetIndex(on bool) { index.Store(on) }
