// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// attribute keys of a request record, see LogRequestHandler
const (
	LogKeyRemoteIP = "remote_ip"
	LogKeyStatus   = "status"
	LogKeyMethod   = "method"
	LogKeyHost     = "host"
	LogKeyURI      = "uri"
	LogKeyDuration = "duration_ms"
)

// LogRequestHandler returns a handler which logs all requests to 'logger',
// one record per request. it also captures the status-code and the time
// 'handler' took to answer, the body written out included, as Millis.
func LogRequestHandler(handler http.Handler, logger *slog.Logger) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sc = statusCodeCapture{w: w}
		start := time.Now()
		handler.ServeHTTP(&sc, r)
		took := time.Since(start)
		if sc.code == 0 {
			sc.code = 200
		}
		logger.LogAttrs(r.Context(), slog.LevelInfo, "request",
			slog.String(LogKeyRemoteIP, remoteIP(r.RemoteAddr)),
			slog.Int(LogKeyStatus, sc.code),
			slog.String(LogKeyMethod, r.Method),
			slog.String(LogKeyHost, r.Host),
			slog.String(LogKeyURI, r.RequestURI),
			slog.Any(LogKeyDuration, Millis(took)))
	})
}

// Millis is a duration rendered in milliseconds with three decimals,
// whatever its size: "20.000", "187000.000". a slog.Duration would be
// "20ms" and "3m7s", a float64 "1.87e+06". the slog text handler uses
// String, the JSON handler MarshalJSON; both write the same number.
type Millis time.Duration

func (m Millis) String() string {
	us := time.Duration(m).Microseconds()
	return fmt.Sprintf("%d.%03d", us/1000, us%1000)
}

func (m Millis) MarshalJSON() ([]byte, error) {
	return []byte(m.String()), nil
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
