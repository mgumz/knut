// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"log/slog"
	"net/http"
	"strings"
)

// attribute keys of a request record, see LogRequestHandler
const (
	LogKeyRemoteIP = "remote_ip"
	LogKeyStatus   = "status"
	LogKeyMethod   = "method"
	LogKeyHost     = "host"
	LogKeyURI      = "uri"
)

// LogRequestHandler returns a handler which logs all requests to 'logger',
// one record per request. it also captures the status-code
func LogRequestHandler(handler http.Handler, logger *slog.Logger) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sc = statusCodeCapture{w: w}
		handler.ServeHTTP(&sc, r)
		if sc.code == 0 {
			sc.code = 200
		}
		logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
			slog.String(LogKeyRemoteIP, remoteIP(r.RemoteAddr)),
			slog.Int(LogKeyStatus, sc.code),
			slog.String(LogKeyMethod, r.Method),
			slog.String(LogKeyHost, r.Host),
			slog.String(LogKeyURI, r.RequestURI))
	})
}

// remoteIP cuts the port off 'addr'. an IPv6 address keeps its brackets,
// the request line always looked like that. 'addr' without a port, as
// some listeners hand it in, stays as it is.
func remoteIP(addr string) string {
	if i := strings.LastIndex(addr, ":"); i >= 0 && !strings.HasSuffix(addr, "]") {
		return addr[:i]
	}
	return addr
}

type statusCodeCapture struct {
	w    http.ResponseWriter
	code int
}

func (sc *statusCodeCapture) Header() http.Header            { return sc.w.Header() }
func (sc *statusCodeCapture) Write(data []byte) (int, error) { return sc.w.Write(data) }
func (sc *statusCodeCapture) WriteHeader(code int)           { sc.code = code; sc.w.WriteHeader(code) }
