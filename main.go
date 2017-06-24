// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

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
knut [opts] [uri:]folder-or-file [mapping2] [mapping3] [...]

Sample:

   knut file.txt /this/:. /ding.txt:/tmp/dong.txt

Mapping Format:

   file.txt                - publish the file "file.txt" via "/file.txt"
   /:.                     - list contents of current directory via "/"
   /uri:folder             - list contents of "folder" via "/uri"
   /uri:file               - serve "file" via "/uri"
   /uri:@text              - respond with "text" at "/uri"
   30x/uri:location        - respond with 301 at "/uri"
   @/upload:folder         - accept multipart encoded data via POST at "/upload"
                             and store it inside "folder". A simple upload form
                             is rendered on GET.
   /c.tgz:tar+gz://./      - creates a (gzipped) tarball from the current directory
                             and serves it via "/c.tgz"
   /z.zip:zip://./         - creates a zip files from the current directory
                             and serves it via "/z.zip"
   /z.zip:zipfs://a.zip    - list and servce the content of the entries of an
                             existing "z.zip" via the "/z.zip": consider a file
                             "example.txt" inside "z.zip", it will be directly
                             available via "/z.zip/example.txt"
   /uri:http://1.2.3.4/    - creates a reverse proxy and forwards requests to /uri
                             to the given http-host
   /uri:git://folder/      - serves files via "git http-backend"
   /uri:cgit://path/to/dir - serves git-repos via "cgit"
   /uri:myip://            - serves a "myip" endpoint

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

	fmt.Printf("\nknut started on %s, be aware of the trees!\n\n", opts.bindAddr)
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

		window, tree, err = getWindowAndTree(mappings[i])
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
			handler, verb = uploadHandler(tree), "catches"
		case strings.HasPrefix(window, "30x"):
			if window = window[3:]; window == "" {
				fmt.Fprintf(os.Stderr, "warning: post uri in pair %d is empty\n", i)
				continue
			}
			handler, verb = redirectHandler(window, tree), "points at"
		case tree[0] == STRING_HANDLER:
			handler = serveStringHandler(tree[1:])
		default:
			if treeURL, err := url.Parse(tree); err == nil {
				query := treeURL.Query()
				switch treeURL.Scheme {
				case "http", "https":
					handler = httputil.NewSingleHostReverseProxy(treeURL)
				case "file":
					handler = fileOrDirHandler(localFilename(treeURL), window)
				case "myip":
					handler = myIPHandler()
				case "qr":
					qrContent := treeURL.Path
					if len(qrContent) <= 1 {
						fmt.Fprintf(os.Stderr, "warning: qr:// needs content, %q\n", qrContent)
						continue
					}
					qrContent = qrContent[1:] // cut away the leading /
					handler = qrHandler(qrContent)
					handler = setContentType(handler, "image/png")
				case "git":
					handler = gitHandler(localFilename(treeURL), window)
				case "cgit":
					handler = cgitHandler(localFilename(treeURL), window)
				case "tar":
					prefix := query.Get("prefix")
					handler = tarHandler(localFilename(treeURL), prefix)
					handler = setContentType(handler, "application/x-tar")
				case "tar+gz", "tar.gz", "tgz":
					prefix := query.Get("prefix")
					clevel := query.Get("level")
					handler = tarHandler(localFilename(treeURL), prefix)
					handler = gzHandler(handler, clevel)
					handler = setContentType(handler, "application/x-gtar")
				case "zip":
					prefix := query.Get("prefix")
					store := hasQueryParam("store", query)
					handler = zipHandler(localFilename(treeURL), prefix, store)
					handler = setContentType(handler, "application/zip")
				case "zipfs":
					prefix := query.Get("prefix")
					index := query.Get("index")
					handler = zipFSHandler(localFilename(treeURL), prefix, index)
				}
			}

			if handler == nil {
				handler = fileOrDirHandler(tree, window)
			}
		}

		fmt.Printf("knut %s %q through %q\n", verb, tree, window)
		muxer.Handle(window, handler)
		nHandlers, handler = nHandlers+1, nil
	}
	return muxer, nHandlers
}
