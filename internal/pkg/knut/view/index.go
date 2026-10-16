// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package view

// Window is one row of the index: a published window, and whether it
// asks for a login.
type Window struct {
	Path      string
	Protected bool
}
