// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRequestLineHandlerGolden(t *testing.T) {

	// the duration keeps its unit, whatever its size
	tests := []struct {
		took time.Duration
		want string
	}{
		{0, "0.000"},
		{56 * time.Microsecond, "0.056"},
		{1234 * time.Microsecond, "1.234"},
		{20 * time.Millisecond, "20.000"},
		{187 * time.Second, "187000.000"},
		{26 * time.Hour, "93600000.000"},
	}

	when := time.Date(2026, 10, 16, 12, 34, 56, 0, time.FixedZone("", 2*60*60))
	for _, tt := range tests {
		var buf bytes.Buffer
		r := slog.NewRecord(when, slog.LevelInfo, "request", 0)
		r.AddAttrs(
			slog.String(LogKeyRemoteIP, "192.0.2.7"),
			slog.Int(LogKeyStatus, 404),
			slog.String(LogKeyMethod, "GET"),
			slog.String(LogKeyHost, "example.com:8080"),
			slog.String(LogKeyURI, "/a/b?c=d"),
			slog.Any(LogKeyDuration, Millis(tt.took)),
			slog.String("unknown", "dropped"))

		if err := NewRequestLineHandler(&buf).Handle(context.Background(), r); err != nil {
			t.Fatal(err)
		}

		want := "2026-10-16T12:34:56+02:00\t192.0.2.7\t404\tGET\texample.com:8080/a/b?c=d\t" + tt.want + "\n"
		if got := buf.String(); got != want {
			t.Errorf("%v: got %q, want %q", tt.took, got, want)
		}
	}
}

// the stock slog handlers write the duration as the request line does
func TestMillisInSlogHandlers(t *testing.T) {

	for _, took := range []time.Duration{56 * time.Microsecond, 20 * time.Millisecond, 187 * time.Second} {
		want := Millis(took).String()

		var text, js bytes.Buffer
		slog.New(slog.NewTextHandler(&text, nil)).Info("request", LogKeyDuration, Millis(took))
		slog.New(slog.NewJSONHandler(&js, nil)).Info("request", LogKeyDuration, Millis(took))

		if !strings.Contains(text.String(), " "+LogKeyDuration+"="+want+"\n") {
			t.Errorf("%v: text handler wrote %q, want %s=%s", took, text.String(), LogKeyDuration, want)
		}
		if !strings.Contains(js.String(), `"`+LogKeyDuration+`":`+want+"}") {
			t.Errorf("%v: json handler wrote %q, want %q:%s", took, js.String(), LogKeyDuration, want)
		}
		if !json.Valid(js.Bytes()) {
			t.Errorf("%v: json handler wrote invalid JSON %q", took, js.String())
		}
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
			i := strings.LastIndex(rest, "\t")
			if ms, err := strconv.ParseFloat(strings.TrimSuffix(rest[i+1:], "\n"), 64); err != nil || ms < 0 {
				t.Errorf("got duration %q, want milliseconds", rest[i+1:])
			}
			if got := "\t" + rest[:i] + "\n"; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
