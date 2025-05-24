// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import "net/http"

// basicAuthHandler checks the submited username and password against predefined
// values.
func BasicAuthHandler(next http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="knut"`)
		rUser, rPassword, ok := r.BasicAuth()
		if ok && rUser == username && password == rPassword {
			next.ServeHTTP(w, r)
		} else {
			writeStatus(w, http.StatusUnauthorized)
		}
	})
}
