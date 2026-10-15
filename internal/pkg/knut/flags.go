// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

import (
	"flag"
	"strings"
)

type Opts struct {
	BindAddr          string
	DoLog             bool
	Auth              []string
	AuthFiles         []string
	HashUser          string
	DoCompress        bool
	DoInteractiveBind bool
	DoTeeBody         bool
	DoIndexHandler    bool
	DoLive            bool
	DoShowQR          bool
	DoPrintVersion    bool
	AddServerID       string
	TlsOnetime        bool
	TlsCert           string
	TlsKey            string
}

// stringList collects a flag given more than once.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ", ") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

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
	f.BoolVar(&opts.DoInteractiveBind, "select-addr", opts.DoInteractiveBind, `interactively select -bind address`)
	f.BoolVar(&opts.DoIndexHandler, "serve-index", opts.DoIndexHandler, `create a small index-page, listing the various paths`)
	f.BoolVar(&opts.DoLive, "live", opts.DoLive, `listings refresh themselves, uploads report progress (serves htmx at "`+LiveAssetURI+`")`)
	f.BoolVar(&opts.DoShowQR, "show-qr", opts.DoShowQR, `show a QR code to stdout pointing to '/' (useful only if -bind is distinct)`)
	f.BoolVar(&opts.DoTeeBody, "tee-body", opts.DoTeeBody, `dump request.body to stdout`)
	f.Var((*stringList)(&opts.Auth), "auth", "ask for a login, an `entry` is '[/window=]name:password',\nrepeatable (see \"Logins\")")
	f.Var((*stringList)(&opts.AuthFiles), "auth-file", "read -auth entries from `file`, one per line, repeatable")
	f.StringVar(&opts.HashUser, "hash", opts.HashUser, "print an entry with a bcrypt hash for `name` and exit,\nthe password is asked for, or read from stdin")
	f.StringVar(&opts.AddServerID, "server-id", opts.AddServerID, `add "Server: <val-here>" to the response`)
	f.BoolVar(&opts.TlsOnetime, "tls-onetime", opts.TlsOnetime, "use a self-signed in-memory cert+key, valid while knut runs, to drive tls")
	f.StringVar(&opts.TlsKey, "tls-key", opts.TlsKey, "use given key to start tls")
	f.StringVar(&opts.TlsCert, "tls-cert", opts.TlsCert, "use given cert to start tls")
	f.BoolVar(&opts.DoPrintVersion, "version", opts.DoPrintVersion, "print version")
	f.Usage = func() { printUsage(f) }

	return &opts
}
