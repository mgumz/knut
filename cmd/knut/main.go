// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

//go:generate go run -v ./gen_doc.go -o doc.go

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut"
)

func main() {

	opts := knut.SetupFlags(flag.CommandLine)

	flag.CommandLine.SetOutput(os.Stdout)
	flag.Parse()

	if opts.DoPrintVersion {
		fmt.Println(knut.Version, knut.GitHash, knut.BuildDate)
		os.Exit(0)
	}

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "error: missing mapping\n")
		flag.Usage()
		os.Exit(1)
	}

	tree, nHandlers := prepareTrees(http.NewServeMux(), flag.Args())
	if nHandlers == 0 {
		fmt.Fprintf(os.Stderr, "error: not one valid mapping given\n")
		flag.Usage()
		os.Exit(1)
	}

	//
	// setup the chain of handlers
	//
	var h = http.Handler(tree)

	if opts.AddServerID != "" {
		h = knut.AddServerIDHandler(h, opts.AddServerID)
	}
	h = knut.NoCacheHandler(h)
	if opts.DoCompress {
		h = knut.CompressHandler(h)
	}
	if opts.DoAuth != "" {
		parts := strings.SplitN(opts.DoAuth, ":", 2)
		if len(parts) == 0 {
			fmt.Fprintf(os.Stderr, "error: missing separator for argument to -auth")
			os.Exit(1)
		}
		h = knut.BasicAuthHandler(h, parts[0], parts[1])
	}
	if opts.DoLog {
		h = knut.LogRequestHandler(h, os.Stdout)
	}

	//
	// and .. action.
	//
	var run func() error
	switch {
	case opts.TlsOnetime:
		onetime := &knut.OnetimeTLS{}
		if err := onetime.Create(opts.BindAddr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		run = func() error { return http.Serve(onetime.Listener, h) }
	case opts.TlsCert != "" && opts.TlsKey != "":
		run = func() error {
			server := &http.Server{Addr: opts.BindAddr, Handler: h}
			return server.ListenAndServeTLS(opts.TlsCert, opts.TlsKey)
		}
	default:
		run = func() error { return http.ListenAndServe(opts.BindAddr, h) }
	}

	fmt.Printf("\nknut started on %s, be aware of the trees!\n\n", opts.BindAddr)
	if err := run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

// prepareTrees binds a list of uri:tree pairs to 'muxer'
func prepareTrees(muxer *http.ServeMux, mappings []string) (*http.ServeMux, int) {

	const (
		UPLOAD_HANDLER = '@'
		STRING_HANDLER = '@'
	)

	var (
		nHandlers    int
		handler      http.Handler
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
			handler, verb = knut.UploadHandler(tree), "catches"
		case strings.HasPrefix(window, "30x"):
			if window = window[3:]; window == "" {
				fmt.Fprintf(os.Stderr, "warning: post uri in pair %d is empty\n", i)
				continue
			}
			handler, verb = knut.RedirectHandler(window, tree), "points at"
		case tree[0] == STRING_HANDLER:
			handler = knut.ServeStringHandler(tree[1:])
		default:
			if treeURL, err := url.Parse(tree); err == nil {
				query := treeURL.Query()
				switch treeURL.Scheme {
				case "http", "https":
					handler = httputil.NewSingleHostReverseProxy(treeURL)
				case "file":
					handler = knut.FileOrDirHandler(knut.LocalFilename(treeURL), window)
				case "myip":
					handler = knut.MyIPHandler()
				case "qr":
					qrContent := treeURL.Path
					if len(qrContent) <= 1 {
						fmt.Fprintf(os.Stderr, "warning: qr:// needs content, %q\n", qrContent)
						continue
					}
					qrContent = qrContent[1:] // cut away the leading /
					handler = knut.QrHandler(qrContent)
					handler = knut.SetContentType(handler, "image/png")
				case "git":
					handler = knut.GitHandler(knut.LocalFilename(treeURL), window)
				case "cgit":
					handler = knut.CgitHandler(knut.LocalFilename(treeURL), window)
				case "tar":
					prefix := query.Get("prefix")
					handler = knut.TarHandler(knut.LocalFilename(treeURL), prefix)
					handler = knut.SetContentType(handler, "application/x-tar")
				case "tar+gz", "tar.gz", "tgz":
					prefix := query.Get("prefix")
					clevel := query.Get("level")
					handler = knut.TarHandler(knut.LocalFilename(treeURL), prefix)
					handler = knut.GzHandler(handler, clevel)
					handler = knut.SetContentType(handler, "application/x-gtar")
				case "zip":
					prefix := query.Get("prefix")
					store := knut.HasQueryParam("store", query)
					handler = knut.ZipHandler(knut.LocalFilename(treeURL), prefix, store)
					handler = knut.SetContentType(handler, "application/zip")
				case "zipfs":
					prefix := query.Get("prefix")
					index := query.Get("index")
					handler = knut.ZipFSHandler(knut.LocalFilename(treeURL), prefix, index)
				}
			}

			if handler == nil {
				handler = knut.FileOrDirHandler(tree, window)
			}
		}

		fmt.Printf("knut %s %q through %q\n", verb, tree, window)
		muxer.Handle(window, handler)
		nHandlers, handler = nHandlers+1, nil
	}
	return muxer, nHandlers
}
