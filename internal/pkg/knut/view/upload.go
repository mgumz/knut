// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

import (
	"html/template"

	_ "embed"
)

// uploadJS sends the files of a live upload form one per request, each
// with its own bar and a retry when it did not make it. it is inlined into
// the upload page with -live only: without it the form posts all files at
// once, which works the same, just without the bars.
//
//go:embed assets/upload.js
var uploadJS string

func uploadScript() template.JS { return template.JS(uploadJS) }
