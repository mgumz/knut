// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import "net/http"

// serveFileHandler serves the file given by 'name'
func serveFileHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, name)
	})
}

// serveStringHandler writes given 'str' to the response
func serveStringHandler(str string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(str))
	})
}

// addServerIdaddServerIDHandler adds "Server: <server_id>" to the response header
func addServerIDHandler(handler http.Handler, server_id string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", server_id)
		handler.ServeHTTP(w, r)
	})
}

// noCacheHandler adds a 'nocaching, please' hint to the response header
func noCacheHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, max-age=0, no-cache")
		handler.ServeHTTP(w, r)
	})
}

// basicAuthHandler checks the submited username and password against predefined values.
func basicAuthHandler(handler http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="knut"`)
		rUser, rPassword, ok := r.BasicAuth()
		if ok && rUser == username && password == rPassword {
			handler.ServeHTTP(w, r)
		} else {
			writeStatus(w, http.StatusUnauthorized)
		}
	})
}
