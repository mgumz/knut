// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

// LiveAssetURI is the one uri knut reserves for itself, and only while
// "-live" is given. knut maps arbitrary uris, so taking one away is not
// free: it is dotted and namespaced to stay out of the way, and a mapping
// claiming it is warned about at startup.
const LiveAssetURI = "/.knut/htmx.js"
