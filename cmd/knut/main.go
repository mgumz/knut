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

	qrcode "github.com/skip2/go-qrcode"

	"github.com/mgumz/knut/internal/pkg/knut"
	"github.com/mgumz/knut/internal/pkg/knut/handler"
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

	tree, windows := prepareTrees(http.NewServeMux(), flag.Args())
	if len(windows) == 0 {
		fmt.Fprintf(os.Stderr, "error: not one valid mapping given\n")
		flag.Usage()
		os.Exit(1)
	}

	if opts.DoIndexHandler {
		tree.Handle("/", handler.IndexHandler(windows))
	}

	//
	// setup the chain of handlers
	//
	var h = http.Handler(tree)

	if opts.AddServerID != "" {
		h = handler.AddServerIDHandler(h, opts.AddServerID)
	}
	h = handler.NoCacheHandler(h)
	if opts.DoCompress {
		h = handler.CompressHandler(h)
	}
	if opts.DoAuth != "" {
		parts := strings.SplitN(opts.DoAuth, ":", 2)
		if len(parts) == 0 {
			fmt.Fprintf(os.Stderr, "error: missing separator for argument to -auth")
			os.Exit(1)
		}
		h = handler.BasicAuthHandler(h, parts[0], parts[1])
	}
	if opts.DoLog {
		h = handler.LogRequestHandler(h, os.Stdout)
	}

	if opts.DoTeeBody {
		h = handler.FlushBodyHandler(h)
		h = handler.TeeBodyHandler(h, os.Stdout)
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

	//
	//
	// NOTE: maybe needed one day: the QR code will scroll out if enough
	// request were served (and logged). maybe not a problem for now.
	if opts.DoShowQR {

		proto := "http"
		if opts.TlsOnetime || opts.TlsCert != "" {
			proto = "https"
		}

		url := fmt.Sprintf("%s://%s/", proto, opts.BindAddr)
		qr, _ := qrcode.New(url, qrcode.Medium)

		fmt.Println(qr.ToString(true))
	}

	//
	// finally: run knut
	if err := run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
