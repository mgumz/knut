// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

// *knut* is a tiny webserver which serves folders (file trees) via user
// specified urls. it grew a little bit beyond that scope and now serves
// also single files and accepts file-uploads.
//
// usage:
//
//  knut [opts] uri:folder [uri2:file1] [@upload:upload_folder] [...]
//
// sample:
//
//   knut /:. /ding.txt:/tmp/dong.txt
//
// options:
//
//  -auth="": use 'name:password' to require
//  -bind=":8080": address to bind to
//  -compress=true: handle "Accept-Encoding" = "gzip,deflate"
//  -log=true: log requests to stdout
//  -server-id="knut/1.0": add "Server: <val-here>" to the response
//  -tls-cert="": use given cert to start tls
//  -tls-key="": use given key to start tls
//  -tls-onetime=false: use a onetime-in-memory cert+key to drive tls
//
// the name:
//
// *knut* or "st.knut's day" is an annually celebrated festival in sweden /
// finland on 13 January. it marks the end of christmas, among other
// activities the christmas trees are disposed.
//
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
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

	const (
		errMsgMissingSep  = "error: pair %d (%q) doesn't contain the uri-tree seprator ':', ignoring\n"
		errMsgInvalidPair = "error: invalid pair %d: uri: %q, tree: %q\n"
	)

	var (
		nHandlers, i int
		parts        []string
		uri, tree    string
		handler      http.Handler
		fi           os.FileInfo
		err          error
	)

	for i, tree = range mappings {

		if parts = strings.SplitN(tree, ":", 2); len(parts) != 2 {
			fmt.Fprintf(os.Stderr, errMsgMissingSep, i+1, tree)
			continue
		}

		if uri, tree = parts[0], parts[1]; uri == "" || tree == "" {
			fmt.Fprintf(os.Stderr, errMsgInvalidPair, i+1, uri, tree)
			continue
		}

		verb := "throws"
		switch {
		case uri[0] == '@': // indicates upload handler
			if uri = uri[1:]; uri == "" {
				fmt.Fprintf(os.Stderr, "error: post uri in pair %d is empty\n", i)
				continue
			}
			if fi, err = os.Stat(tree); err == nil && !fi.IsDir() {
				fmt.Fprintf(os.Stderr, "error: existing %q is not a directory\n", tree)
				continue
			}
			handler, verb = uploadHandler(tree), "catches"
		case tree[0] == '@':
			handler = serveStringHandler(tree[1:])
		default:
			if extra, ok := url.Parse(tree); ok == nil {
				switch extra.Scheme {
				case "http", "https":
					handler = httputil.NewSingleHostReverseProxy(extra)
				}
			}

			if handler != nil {
				break
			} else if fi, err = os.Stat(tree); err == nil && !fi.IsDir() {
				handler = serveFileHandler(tree)
			} else {
				handler = http.FileServer(http.Dir(tree))
				handler = http.StripPrefix(uri, handler)
			}
		}

		fmt.Printf("knut %s %q through %q\n", verb, tree, uri)
		muxer.Handle(uri, handler)
		nHandlers++
	}
	return muxer, nHandlers
}
