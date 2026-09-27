// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// NotFoundHandler answers every request with the knut 404 page.
func NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		view.Status(w, r, http.StatusNotFound)
	})
}
