// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// the request line is byte for byte the one knut wrote before slog
func TestRequestLineHandlerGolden(t *testing.T) {

	var buf bytes.Buffer
	when := time.Date(2026, 10, 16, 12, 34, 56, 0, time.FixedZone("", 2*60*60))
	r := slog.NewRecord(when, slog.LevelInfo, "request", 0)
	r.AddAttrs(
		slog.String(LogKeyRemoteIP, "192.0.2.7"),
		slog.Int(LogKeyStatus, 404),
		slog.String(LogKeyMethod, "GET"),
		slog.String(LogKeyHost, "example.com:8080"),
		slog.String(LogKeyURI, "/a/b?c=d"),
		slog.String("unknown", "dropped"))

	if err := NewRequestLineHandler(&buf).Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	want := "2026-10-16T12:34:56+02:00\t192.0.2.7\t404\tGET\texample.com:8080/a/b?c=d\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLogRequestHandler(t *testing.T) {

	tests := []struct {
		name       string
		remoteAddr string
		inner      http.HandlerFunc
		want       string
	}{
		{"default status", "192.0.2.7:51234",
			func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("hi")) },
			"\t192.0.2.7\t200\tGET\texample.com/x?y=z\n"},
		{"explicit status", "192.0.2.7:51234",
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) },
			"\t192.0.2.7\t418\tGET\texample.com/x?y=z\n"},
		{"ipv6 keeps brackets", "[2001:db8::1]:51234",
			func(http.ResponseWriter, *http.Request) {},
			"\t[2001:db8::1]\t200\tGET\texample.com/x?y=z\n"},
		{"no port", "192.0.2.7",
			func(http.ResponseWriter, *http.Request) {},
			"\t192.0.2.7\t200\tGET\texample.com/x?y=z\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(NewRequestLineHandler(&buf))
			req := httptest.NewRequest(http.MethodGet, "/x?y=z", nil)
			req.RemoteAddr = tt.remoteAddr

			LogRequestHandler(tt.inner, logger).ServeHTTP(httptest.NewRecorder(), req)

			stamp, rest, ok := strings.Cut(buf.String(), "\t")
			if !ok {
				t.Fatalf("got %q, want a tab-separated line", buf.String())
			}
			if _, err := time.Parse(time.RFC3339, stamp); err != nil {
				t.Errorf("got time %q, want RFC3339: %v", stamp, err)
			}
			if got := "\t" + rest; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
