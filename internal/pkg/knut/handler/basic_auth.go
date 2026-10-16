// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"net/http"

	"github.com/mgumz/knut/internal/pkg/knut/auth"
	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// BasicAuthHandler asks for the logins 'logins' hands out for a request.
// nil logins: the request passes without one.
func BasicAuthHandler(next http.Handler, logins func(r *http.Request) *auth.Logins) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := logins(r)
		if l == nil {
			next.ServeHTTP(w, r)
			return
		}
		if user, password, ok := r.BasicAuth(); ok && l.Check(user, password) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="`+l.Realm()+`", charset="UTF-8"`)
		view.Status(w, r, http.StatusUnauthorized)
	})
}
