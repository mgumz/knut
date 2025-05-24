// Copyright 2025 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"bytes"
	"fmt"
	"net/http"
)

func IndexHandler(windows []string) http.Handler {

	indexPage := bytes.NewBuffer(nil)
	fmt.Fprint(indexPage, `<!DOCTYPE html>
<html>
	<head>
		<title>Knut</title>
	</head>
	<body>
		<h1>Knut</h1>
		<ul>
`)
	for _, window := range windows {
		fmt.Fprintf(indexPage, `			<li><a href=".%s">%s</a></li>`,
			window, window)
		fmt.Fprintln(indexPage)
	}

	fmt.Fprint(indexPage, `
		</ul>
	</body>
</html>`)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(indexPage.Bytes())
	})
}
