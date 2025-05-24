// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"fmt"
	"net/http"
)

// writeStatus renders the given status code and
// a text associated with that code
func writeStatus(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "%d: %s", code, http.StatusText(code))
}
