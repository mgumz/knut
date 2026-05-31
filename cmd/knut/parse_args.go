// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut"
	kh "github.com/mgumz/knut/internal/pkg/knut/handler"
)

// prepareTrees binds a list of uri:tree pairs to 'muxer'
func prepareTrees(muxer *http.ServeMux, mappings []string) (*http.ServeMux, []string) {

	windows := []string{}

	for i := range mappings {
		window, tree, handler, verb, ok := handlerForMapping(mappings[i], i)
		if !ok {
			continue
		}

		fmt.Printf("knut %s %q through %q\n", verb, tree, window)
		muxer.Handle(window, handler)
		windows = append(windows, window)
	}

	return muxer, windows
}

// handlerForMapping parses a single uri:tree pair and builds its handler.
// ok=false means the mapping warned (if needed) and must be skipped.
func handlerForMapping(mapping string, pos int) (window, tree string, handler http.Handler, verb string, ok bool) {

	const (
		UPLOAD_HANDLER = '@'
		STRING_HANDLER = '@'
	)

	window, tree, err := knut.GetWindowAndTree(mapping)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: parsing %q (pos %d): %v\n", mapping, pos+1, err)
		return "", "", nil, "", false
	}

	verb = "throws"
	switch {
	case window[0] == UPLOAD_HANDLER:
		if window = window[1:]; window == "" {
			fmt.Fprintf(os.Stderr, "warning: post uri in pair %d is empty\n", pos)
			return "", "", nil, "", false
		}
		if fi, err := os.Stat(tree); err == nil && !fi.IsDir() {
			fmt.Fprintf(os.Stderr, "warning: existing %q is not a directory\n", tree)
			return "", "", nil, "", false
		}
		handler, verb = kh.UploadHandler(tree), "catches"
	case strings.HasPrefix(window, "200"):
		if window = window[3:]; window == "" {
			fmt.Fprintf(os.Stderr, "warning: 20x path in pair %d is empty\n", pos)
			return "", "", nil, "", false
		}
		handler, verb = kh.TwoZeroXHandler(window, http.StatusOK), "points at"
	case strings.HasPrefix(window, "30x"):
		if window = window[3:]; window == "" {
			fmt.Fprintf(os.Stderr, "warning: 30x path in pair %d is empty\n", pos)
			return "", "", nil, "", false
		}
		handler, verb = kh.RedirectHandler(window, tree), "points at"
	case tree[0] == STRING_HANDLER:
		handler = kh.ServeStringHandler(tree[1:])
	default:
		if treeURL, err := url.Parse(tree); err == nil {
			var skip bool
			if handler, skip = schemeHandler(treeURL, window); skip {
				return "", "", nil, "", false
			}
		}
		if handler == nil {
			handler = kh.FileOrDirHandler(tree, window)
		}
	}

	return window, tree, handler, verb, true
}

// schemeHandler builds a handler from tree's URL scheme.
// skip=true means the mapping warned and must be skipped.
func schemeHandler(treeURL *url.URL, window string) (http.Handler, bool) {
	query := treeURL.Query()
	switch treeURL.Scheme {
	case "http", "https":
		return httputil.NewSingleHostReverseProxy(treeURL), false
	case "file":
		return kh.FileOrDirHandler(knut.LocalFilename(treeURL), window), false
	case "myip":
		// myip://?fuzzy&info=ripe
		return kh.MyIPHandler(query.Get("info"), query.Has("fuzzy")), false
	case "qr":
		qrContent := treeURL.Path
		if len(qrContent) <= 1 {
			fmt.Fprintf(os.Stderr, "warning: qr:// needs content, %q\n", qrContent)
			return nil, true
		}
		qrContent = qrContent[1:] // cut away the leading /
		handler := kh.QrHandler(qrContent)
		return kh.SetContentType(handler, "image/png"), false
	case "git":
		return kh.GitHandler(knut.LocalFilename(treeURL), window), false
	case "cgit":
		return kh.CgitHandler(knut.LocalFilename(treeURL), window), false
	case "tar":
		prefix := query.Get("prefix")
		handler := kh.TarHandler(knut.LocalFilename(treeURL), prefix)
		return kh.SetContentType(handler, "application/x-tar"), false
	case "tar+gz", "tar.gz", "tgz":
		prefix := query.Get("prefix")
		clevel := query.Get("level")
		handler := kh.TarHandler(knut.LocalFilename(treeURL), prefix)
		handler = kh.GzHandler(handler, clevel)
		return kh.SetContentType(handler, "application/x-gtar"), false
	case "zip":
		prefix := query.Get("prefix")
		store := knut.HasQueryParam("store", query)
		handler := kh.ZipHandler(knut.LocalFilename(treeURL), prefix, store)
		return kh.SetContentType(handler, "application/zip"), false
	case "zipfs":
		prefix := query.Get("prefix")
		index := query.Get("index")
		return kh.ZipFSHandler(knut.LocalFilename(treeURL), prefix, index), false
	}
	return nil, false
}
