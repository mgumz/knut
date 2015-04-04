// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var version = "knut/1.0"

func usage() {
	flag.CommandLine.SetOutput(os.Stdout)
	fmt.Println(`
knut [opts] uri:folder [uri2:file1] [@upload:upload_folder] [...]

Sample:

   knut /:. /ding.txt:/tmp/dong.txt

Options:
	`)
	flag.PrintDefaults()
	fmt.Println()
}

func main() {

	opts := struct {
		bindAddr    string
		doLog       bool
		doAuth      string
		doCompress  bool
		addServerID string
		tlsOnetime  bool
		tlsCert     string
		tlsKey      string
	}{
		bindAddr:    ":8080",
		doLog:       true,
		doCompress:  true,
		addServerID: version,
	}

	flag.StringVar(&opts.bindAddr, "bind", opts.bindAddr, "address to bind to")
	flag.BoolVar(&opts.doLog, "log", opts.doLog, "log requests to stdout")
	flag.BoolVar(&opts.doCompress, "compress", opts.doCompress, `handle "Accept-Encoding" = "gzip,deflate"`)
	flag.StringVar(&opts.doAuth, "auth", "", "use 'name:password' to require")
	flag.StringVar(&opts.addServerID, "server-id", opts.addServerID, `add "Server: <val-here>" to the response`)
	flag.BoolVar(&opts.tlsOnetime, "tls-onetime", opts.tlsOnetime, "use a onetime-in-memory cert+key to drive tls")
	flag.StringVar(&opts.tlsKey, "tls-key", opts.tlsKey, "use given key to start tls")
	flag.StringVar(&opts.tlsCert, "tls-cert", opts.tlsCert, "use given cert to start tls")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "error: missing mapping\n")
		usage()
		os.Exit(1)
	}

	tree, nHandlers := prepareTrees(http.NewServeMux(), flag.Args())
	if nHandlers == 0 {
		fmt.Fprintf(os.Stderr, "error: not one valid mapping given\n")
		usage()
		os.Exit(1)
	}

	//
	// setup the chain of handlers
	//
	var h = http.Handler(tree)

	if opts.addServerID != "" {
		h = addServerIDHandler(h, opts.addServerID)
	}
	h = noCacheHandler(h)
	if opts.doCompress {
		h = compressHandler(h)
	}
	if opts.doAuth != "" {
		parts := strings.SplitN(opts.doAuth, ":", 2)
		if len(parts) == 0 {
			fmt.Fprintf(os.Stderr, "error: missing separator for argument to -auth")
			os.Exit(1)
		}
		h = basicAuthHandler(h, parts[0], parts[1])
	}
	if opts.doLog {
		h = logRequestHandler(h, os.Stdout)
	}

	//
	// and .. action.
	//
	var run func() error
	switch {
	case opts.tlsOnetime:
		onetime := &onetimeTLS{}
		if err := onetime.Create(opts.bindAddr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		run = func() error { return http.Serve(onetime.listener, h) }
	case opts.tlsCert != "" && opts.tlsKey != "":
		run = func() error { return http.ListenAndServeTLS(opts.bindAddr, opts.tlsCert, opts.tlsKey, h) }
	default:
		run = func() error { return http.ListenAndServe(opts.bindAddr, h) }
	}

	fmt.Printf("\nknut started on %s, be aware of the trees!\n", opts.bindAddr)
	if err := run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

// prepareTrees binds a list of uri:tree pairs to 'muxer'
func prepareTrees(muxer *http.ServeMux, mappings []string) (*http.ServeMux, int) {

	var (
		nHandlers    int
		handler      http.Handler
		window, tree string
		err          error
		fi           os.FileInfo
	)

	for i := range mappings {

		window, tree, err = getWindowAndTree(mappings[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: parsing %q (pos %d): %v\n", mappings[i], i+1, err)
			continue
		}

		verb := "throws"
		switch {
		case window[0] == '@': // indicates upload handler
			if window = window[1:]; window == "" {
				fmt.Fprintf(os.Stderr, "warning: post uri in pair %d is empty\n", i)
				continue
			}
			if fi, err = os.Stat(tree); err == nil && !fi.IsDir() {
				fmt.Fprintf(os.Stderr, "warning: existing %q is not a directory\n", tree)
				continue
			}
			handler, verb = uploadHandler(tree), "catches"
		case tree[0] == '@':
			handler = serveStringHandler(tree[1:])
		default:
			if treeURL, err := url.Parse(tree); err == nil {
				log.Printf("%q => %q | %q", tree, treeURL.Path, treeURL.Host)
				switch treeURL.Scheme {
				case "http", "https":
					handler = httputil.NewSingleHostReverseProxy(treeURL)
				case "file":
					handler = fileOrDirHandler(localFileName(treeURL), window)
				case "tar":
					handler = setContentType(tarHandler(localFileName(treeURL)), "application/x-tar")
				case "tar+gz", "tar.gz", "tgz":
					handler = setContentType(gzHandler(tarHandler(localFileName(treeURL))), "application/x-gtar")
				case "zipfs":
					handler = zipFsHandler(localFileName(treeURL))
				}
			}

			if handler == nil {
				handler = fileOrDirHandler(tree, window)
			}
		}

		fmt.Printf("knut %s %q through %q\n", verb, tree, window)
		muxer.Handle(window, handler)
		nHandlers++
	}
	return muxer, nHandlers
}

var (
	errMissingSep     = fmt.Errorf("doesn't contain the uri-tree seprator ':', ignoring")
	errEmptyPairParts = fmt.Errorf("empty pair parts")
)

func getWindowAndTree(arg string) (window, tree string, err error) {

	var parts []string
	if parts = strings.SplitN(arg, ":", 2); len(parts) != 2 {
		log.Println(arg, len(parts), parts)
		return "", "", errMissingSep
	}
	if window, tree = parts[0], parts[1]; window == "" || tree == "" {
		return "", "", errEmptyPairParts
	}

	return window, tree, nil
}

func localFileName(fileURL *url.URL) string {
	// TODO: write tests:  file://./localfile file:///absolutefile
	return filepath.Join(fileURL.Host, fileURL.Path)
}
