// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"net/http"

	qrcode "github.com/skip2/go-qrcode"
)

func qrHandler(content string) http.Handler {
	qr, _ := qrcode.New(content, qrcode.Medium)
	qrbytes, _ := qr.PNG(256)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(qrbytes)
	})
}
