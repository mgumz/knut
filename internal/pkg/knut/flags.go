// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

import "flag"

type Opts struct {
	BindAddr       string
	DoLog          bool
	DoAuth         string
	DoCompress     bool
	DoPrintVersion bool
	AddServerID    string
	TlsOnetime     bool
	TlsCert        string
	TlsKey         string
}

func SetupFlags(f *flag.FlagSet) *Opts {

	opts := Opts{
		BindAddr:    ":8080",
		DoLog:       true,
		DoCompress:  true,
		AddServerID: "knut/" + Version,
	}

	f.StringVar(&opts.BindAddr, "bind", opts.BindAddr, "address to bind to")
	f.BoolVar(&opts.DoLog, "log", opts.DoLog, "log requests to stdout")
	f.BoolVar(&opts.DoCompress, "compress", opts.DoCompress, `handle "Accept-Encoding" = "gzip,deflate"`)
	f.StringVar(&opts.DoAuth, "auth", "", "use 'name:password' to require")
	f.StringVar(&opts.AddServerID, "server-id", opts.AddServerID, `add "Server: <val-here>" to the response`)
	f.BoolVar(&opts.TlsOnetime, "tls-onetime", opts.TlsOnetime, "use a onetime-in-memory cert+key to drive tls")
	f.StringVar(&opts.TlsKey, "tls-key", opts.TlsKey, "use given key to start tls")
	f.StringVar(&opts.TlsCert, "tls-cert", opts.TlsCert, "use given cert to start tls")
	f.BoolVar(&opts.DoPrintVersion, "version", opts.DoPrintVersion, "print version")
	f.Usage = func() { printUsage(f) }

	return &opts
}
