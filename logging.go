// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// logRequestHandler returns a handler which logs all requests to 'writer'. it also captures
// the status-code
func logRequestHandler(handler http.Handler, logWriter io.Writer) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sc = statusCodeCapture{w: w}
		handler.ServeHTTP(&sc, r)
		if sc.code == 0 {
			sc.code = 200
		}
		portSep := strings.LastIndex(r.RemoteAddr, ":")
		fmt.Fprintf(logWriter, "%s\t%s\t%d\t%s\t%s%s\n",
			time.Now().Format(time.RFC3339), r.RemoteAddr[:portSep], sc.code, r.Method, r.Host, r.RequestURI)
	})
}

type statusCodeCapture struct {
	w    http.ResponseWriter
	code int
}

func (sc *statusCodeCapture) Header() http.Header            { return sc.w.Header() }
func (sc *statusCodeCapture) Write(data []byte) (int, error) { return sc.w.Write(data) }
func (sc *statusCodeCapture) WriteHeader(code int)           { sc.code = code; sc.w.WriteHeader(code) }
