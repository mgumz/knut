// Copyright 2017 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

import (
	"html/template"
	"net"
	"net/http"
)

// MyIPHandler yells back the remote IP to the browser
func MyIPHandler() http.Handler {

	tmpl, _ := template.New("myip").Parse(`<!doctype html>
<html>
	<head>
		<title>knut - myip</title>
	</head>
	<body>
		<p>Your IP is: <span id="ip">{{ .IP }}</span><span id="port">{{.Port}}</span>
	</body>
</html>`)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, port, _ := net.SplitHostPort(r.RemoteAddr)
		tmpl.Execute(w, &struct{ IP, Port string }{ip, port})
	})
}
