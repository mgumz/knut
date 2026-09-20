// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"encoding/base64"
	"html/template"
	"sync"

	_ "embed"
)

// knutSVG is the icon of a page: knut's tree in the two colours the
// stylesheet paints the header in, --accent and --on-accent. it is drawn
// small enough to still read as a tree at 16 pixels.
//
//go:embed assets/knut.svg
var knutSVG string

// iconURI is knutSVG as a data uri, built once. it rides in every page
// instead of being served, same trade as the stylesheet.
//
// the markup calls it, like it calls qrcode: the go side carries the
// drawing, what is made of it is the block's decision.
//
// base64 rather than percent-encoding: the encoding is then the library's
// and not a table here which has to list every character a drawing must
// not carry raw - the "#" of a colour above all, which would otherwise
// open the fragment of the uri and cut the svg short.
//
// a template.URL because html/template hands out "#ZgotmplZ" instead of a
// "data:" href otherwise, see qrCode.
var iconURI = sync.OnceValue(func() template.URL {
	return template.URL("data:image/svg+xml;base64," +
		base64.StdEncoding.EncodeToString([]byte(knutSVG)))
})
