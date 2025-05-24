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

	const (
		UPLOAD_HANDLER = '@'
		STRING_HANDLER = '@'
	)

	var (
		handler      http.Handler
		windows      = []string{}
		window, tree string
		err          error
		fi           os.FileInfo
	)

	for i := range mappings {

		window, tree, err = knut.GetWindowAndTree(mappings[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parsing %q (pos %d): %v\n", mappings[i], i+1, err)
			continue
		}

		verb := "throws"
		switch {
		case window[0] == UPLOAD_HANDLER:
			if window = window[1:]; window == "" {
				fmt.Fprintf(os.Stderr, "warning: post uri in pair %d is empty\n", i)
				continue
			}
			if fi, err = os.Stat(tree); err == nil && !fi.IsDir() {
				fmt.Fprintf(os.Stderr, "warning: existing %q is not a directory\n", tree)
				continue
			}
			handler, verb = kh.UploadHandler(tree), "catches"
		case strings.HasPrefix(window, "200"):
			if window = window[3:]; window == "" {
				fmt.Fprintf(os.Stderr, "warning: 20x path in pair %d is empty\n", i)
				continue
			}
			handler, verb = kh.TwoZeroXHandler(window, http.StatusOK), "points at"
		case strings.HasPrefix(window, "30x"):
			if window = window[3:]; window == "" {
				fmt.Fprintf(os.Stderr, "warning: 30x path in pair %d is empty\n", i)
				continue
			}
			handler, verb = kh.RedirectHandler(window, tree), "points at"
		case tree[0] == STRING_HANDLER:
			handler = kh.ServeStringHandler(tree[1:])
		default:
			if treeURL, err := url.Parse(tree); err == nil {
				query := treeURL.Query()

				switch treeURL.Scheme {
				case "http", "https":
					handler = httputil.NewSingleHostReverseProxy(treeURL)

				case "file":
					handler = kh.FileOrDirHandler(knut.LocalFilename(treeURL), window)

				case "myip":
					// myip://?fuzzy&info=ripe
					infoAPI := query.Get("info")
					fuzzyIP := query.Has("fuzzy")
					handler = kh.MyIPHandler(infoAPI, fuzzyIP)

				case "qr":
					qrContent := treeURL.Path
					if len(qrContent) <= 1 {
						fmt.Fprintf(os.Stderr, "warning: qr:// needs content, %q\n", qrContent)
						continue
					}
					qrContent = qrContent[1:] // cut away the leading /
					handler = kh.QrHandler(qrContent)
					handler = kh.SetContentType(handler, "image/png")

				case "git":
					handler = kh.GitHandler(knut.LocalFilename(treeURL), window)

				case "cgit":
					handler = kh.CgitHandler(knut.LocalFilename(treeURL), window)

				case "tar":
					prefix := query.Get("prefix")
					handler = kh.TarHandler(knut.LocalFilename(treeURL), prefix)
					handler = kh.SetContentType(handler, "application/x-tar")

				case "tar+gz", "tar.gz", "tgz":
					prefix := query.Get("prefix")
					clevel := query.Get("level")
					handler = kh.TarHandler(knut.LocalFilename(treeURL), prefix)
					handler = kh.GzHandler(handler, clevel)
					handler = kh.SetContentType(handler, "application/x-gtar")

				case "zip":
					prefix := query.Get("prefix")
					store := knut.HasQueryParam("store", query)
					handler = kh.ZipHandler(knut.LocalFilename(treeURL), prefix, store)
					handler = kh.SetContentType(handler, "application/zip")

				case "zipfs":
					prefix := query.Get("prefix")
					index := query.Get("index")
					handler = kh.ZipFSHandler(knut.LocalFilename(treeURL), prefix, index)
				}
			}

			if handler == nil {
				handler = kh.FileOrDirHandler(tree, window)
			}
		}

		fmt.Printf("knut %s %q through %q\n", verb, tree, window)
		muxer.Handle(window, handler)
		windows, handler = append(windows, window), nil
	}
	return muxer, windows
}
