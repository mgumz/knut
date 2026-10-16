// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

// RequestLineHandler is a slog.Handler which writes the records of
// LogRequestHandler as one tab-separated line each:
//
//	time  remote_ip  status  method  host+uri  duration
//
// duration is a Millis. attributes it does
// not know are dropped.
type RequestLineHandler struct {
	mu *sync.Mutex
	w  io.Writer
}

// NewRequestLineHandler returns a RequestLineHandler writing to 'w'.
func NewRequestLineHandler(w io.Writer) *RequestLineHandler {
	return &RequestLineHandler{mu: &sync.Mutex{}, w: w}
}

func (h *RequestLineHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo
}

func (h *RequestLineHandler) Handle(_ context.Context, r slog.Record) error {
	var ip, method, host, uri string
	var status int64
	var took Millis
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case LogKeyRemoteIP:
			ip = a.Value.String()
		case LogKeyStatus:
			status = a.Value.Int64()
		case LogKeyMethod:
			method = a.Value.String()
		case LogKeyHost:
			host = a.Value.String()
		case LogKeyURI:
			uri = a.Value.String()
		case LogKeyDuration:
			took, _ = a.Value.Any().(Millis)
		}
		return true
	})
	line := fmt.Sprintf("%s\t%s\t%d\t%s\t%s%s\t%s\n",
		r.Time.Format(time.RFC3339), ip, status, method, host, uri, took)

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, line)
	return err
}

// WithAttrs returns 'h': the line has a fixed shape.
func (h *RequestLineHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

// WithGroup returns 'h': the line has a fixed shape.
func (h *RequestLineHandler) WithGroup(_ string) slog.Handler { return h }
