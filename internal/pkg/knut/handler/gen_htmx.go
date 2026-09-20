// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

//go:build ignore

// gen_htmx vendors htmx into the handler package: it fetches the already
// minified release from a cdn, gzips it and drops the result next to
// knut.css. no minifier of our own - the distributed "htmx.min.js" is what
// upstream ships and what every other consumer of htmx runs.
//
// the asset is stored compressed: that is the shape it goes out over the
// wire in, and it keeps some 35 kb of javascript out of the binary. what
// the fetch produced is pinned in the generated go file, so a later run
// against the same version is verifiable.
//
// the pinned release is re-fetched with "go generate ./..." - the version
// to pin is what "-version" was last run with:
//
//	go run ./gen_htmx.go -version 2.0.10
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// the cdn to pull from. unpkg serves straight out of the npm registry,
// "htmx.org" is the package htmx itself publishes.
const (
	cdnURL      = "https://unpkg.com/htmx.org@%s/dist/htmx.min.js"
	licenseURL  = "https://unpkg.com/htmx.org@%s/LICENSE"
	maxAssetLen = 4 << 20 // a minified htmx is ~50 kb, 4 mb is the bad-url guard
)

func main() {

	version := flag.String("version", "2.0.10", "version of htmx to vendor")
	asset := flag.String("o", "assets/htmx.min.js.gz", "where to write the compressed asset")
	license := flag.String("license", "assets/htmx.LICENSE", "where to write the license")
	gofile := flag.String("go", "htmx_gen.go", "where to write the generated go file")
	flag.Parse()

	js, err := fetch(fmt.Sprintf(cdnURL, *version))
	if err != nil {
		fatal("fetching htmx: %v", err)
	}
	if err := looksLikeHtmx(js); err != nil {
		fatal("%s: %v", *asset, err)
	}

	gz, err := compress(js)
	if err != nil {
		fatal("compressing htmx: %v", err)
	}

	text, err := fetch(fmt.Sprintf(licenseURL, *version))
	if err != nil {
		fatal("fetching license: %v", err)
	}

	sum := sha256.Sum256(js)
	for _, out := range []struct {
		name string
		data []byte
	}{
		{*asset, gz},
		{*license, text},
		{*gofile, genGo(*version, hex.EncodeToString(sum[:]), len(js), len(gz))},
	} {
		if err := os.WriteFile(out.name, out.data, 0644); err != nil {
			fatal("writing %s: %v", out.name, err)
		}
		fmt.Printf("wrote %s (%d bytes)\n", out.name, len(out.data))
	}

	fmt.Printf("htmx %s: %d bytes, %d gzipped, sha256 %s\n",
		*version, len(js), len(gz), hex.EncodeToString(sum[:]))
}

// fetch reads "url" whole. anything but a 200 is an error - a cdn answering
// 404 with a friendly html page must not end up embedded as javascript.
func fetch(url string) ([]byte, error) {

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetLen))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("%s: empty response", url)
	}

	return body, nil
}

// looksLikeHtmx is the sanity check between "the fetch returned bytes" and
// "the binary now ships those bytes as a script tag".
func looksLikeHtmx(js []byte) error {
	switch {
	case len(js) < 10<<10:
		return fmt.Errorf("only %d bytes, that is no htmx", len(js))
	case bytes.HasPrefix(bytes.TrimSpace(js), []byte("<")):
		return fmt.Errorf("starts with %q, that is markup", "<")
	case !bytes.Contains(js, []byte("htmx")):
		return fmt.Errorf("does not mention htmx")
	}
	return nil
}

// compress gzips "data" as hard as the stdlib can - the result is written
// once per release and served for the lifetime of the binary.
func compress(data []byte) ([]byte, error) {

	buf := bytes.NewBuffer(nil)
	gz, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// genGo renders what the handler package needs to know about the asset it
// embeds: which release it is and what it weighs.
func genGo(version, sha string, raw, gz int) []byte {

	const tmpl = `// generated, do NOT edit.
//
//go:generate go run -v ./gen_htmx.go -version @version@

package handler

// the vendored htmx release. see gen_htmx.go for how to update it.
const (
	htmxVersion = "@version@"

	// sha256 of the uncompressed htmx.min.js, as fetched
	htmxSHA256 = "@sha@"

	// what the asset weighs: @raw@ bytes of javascript, @gz@ gzipped
	htmxSizeRaw = @raw@
	htmxSizeGz  = @gz@
)
`

	return []byte(strings.NewReplacer(
		"@version@", version,
		"@sha@", sha,
		"@raw@", fmt.Sprint(raw),
		"@gz@", fmt.Sprint(gz),
	).Replace(tmpl))
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}
