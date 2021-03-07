// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"
)

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

func redirectHandler(path, location string) http.Handler {

	type uriHostPort struct {
		url.URL
		HostOnly string
		Port     string
	}

	var templ = template.Must(template.New(path).Parse(location))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf = bytes.NewBuffer(nil)
		var rUri, _ = url.Parse(r.RequestURI)
		var requestURI = uriHostPort{URL: *rUri}
		requestURI.Host, requestURI.HostOnly, requestURI.Port = r.Host, r.Host, "80"
		if i := strings.IndexByte(r.Host, ':'); i > -1 {
			requestURI.HostOnly = r.Host[:i]
			requestURI.Port = r.Host[i+1:]
		}
		templ.Execute(buf, &requestURI)
		http.Redirect(w, r, buf.String(), http.StatusMovedPermanently)
	})
}

func fileOrDirHandler(path, uri string) http.Handler {

	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return serveFileHandler(path)
	}

	handler := http.FileServer(http.Dir(path))
	handler = http.StripPrefix(uri, handler)
	return handler
}

// addServerIDHandler adds "Server: <serverID>" to the response
// header
func addServerIDHandler(next http.Handler, serverID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", serverID)
		next.ServeHTTP(w, r)
	})
}

// noCacheHandler adds a 'nocaching, please' hint to the response header
func noCacheHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, max-age=0, no-cache")
		next.ServeHTTP(w, r)
	})
}

// basicAuthHandler checks the submited username and password against predefined
// values.
func basicAuthHandler(next http.Handler, username, password string) http.Handler {
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

// setContentType sets the "Content-Type" to "contentType", if not already
// set
func setContentType(next http.Handler, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, exists := w.Header()["Content-Type"]; !exists {
			w.Header().Set("Content-Type", contentType)
		}
		next.ServeHTTP(w, r)
	})
}
