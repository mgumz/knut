// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/mgumz/knut/internal/pkg/knut"
	"github.com/mgumz/knut/internal/pkg/knut/handler"
	"github.com/mgumz/knut/internal/pkg/knut/ui"
)

func main() {

	opts := knut.SetupFlags(flag.CommandLine)

	flag.CommandLine.SetOutput(os.Stdout)
	flag.Parse()

	if opts.DoPrintVersion {
		fmt.Println(knut.Version, knut.GitHash, knut.BuildDate)
		os.Exit(0)
	}

	opts.BindAddr = resolveBindAddr(opts)

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

	h := buildHandlerChain(tree, opts)
	run := makeRunner(opts, h)

	fmt.Printf("\nknut started on %s, be aware of the trees!\n\n", opts.BindAddr)

	showQR(opts)

	if err := run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

// fatal prints a message to stderr and exits with status 1.
func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}

// resolveBindAddr optionally prompts for a concrete interface address when the
// user asked for interactive binding and only gave a port (":port").
func resolveBindAddr(opts *knut.Opts) string {
	if !opts.DoInteractiveBind || !strings.HasPrefix(opts.BindAddr, ":") {
		return opts.BindAddr
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fatal("retrieving interface addresses: %s", err)
	}

	pickedAddr, err := ui.PromptBindAddr(addrs)
	if err != nil {
		fatal("prompt failed %v", err)
	}

	return pickedAddr + opts.BindAddr
}

// buildHandlerChain wraps the muxer with the optional middleware selected via
// opts. Order matters: the outermost wrapper runs first per request.
func buildHandlerChain(tree http.Handler, opts *knut.Opts) http.Handler {
	h := tree

	if opts.AddServerID != "" {
		h = handler.AddServerIDHandler(h, opts.AddServerID)
	}
	h = handler.NoCacheHandler(h)
	if opts.DoCompress {
		h = handler.CompressHandler(h)
	}
	if opts.DoAuth != "" {
		parts := strings.SplitN(opts.DoAuth, ":", 2)
		if len(parts) != 2 {
			fatal("missing separator for argument to -auth")
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

	return h
}

// makeRunner returns the serve function selected by the TLS options.
func makeRunner(opts *knut.Opts, h http.Handler) func() error {
	switch {
	case opts.TlsOnetime:
		onetime := &knut.OnetimeTLS{}
		if err := onetime.Create(opts.BindAddr); err != nil {
			fatal("%v", err)
		}
		return func() error { return http.Serve(onetime.Listener, h) }
	case opts.TlsCert != "" && opts.TlsKey != "":
		return func() error {
			server := &http.Server{Addr: opts.BindAddr, Handler: h}
			return server.ListenAndServeTLS(opts.TlsCert, opts.TlsKey)
		}
	default:
		return func() error { return http.ListenAndServe(opts.BindAddr, h) }
	}
}

// showQR prints a QR code pointing at the served root when requested.
//
// NOTE: maybe needed one day: the QR code will scroll out if enough
// requests were served (and logged). maybe not a problem for now.
func showQR(opts *knut.Opts) {
	if !opts.DoShowQR {
		return
	}

	proto := "http"
	if opts.TlsOnetime || opts.TlsCert != "" {
		proto = "https"
	}

	url := fmt.Sprintf("%s://%s/", proto, opts.BindAddr)
	qr, _ := qrcode.New(url, qrcode.Medium)

	fmt.Println(qr.ToString(true))
}
