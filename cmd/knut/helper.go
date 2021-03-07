// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var (
	errMissingSep     = fmt.Errorf("doesn't contain the uri-tree seprator ':', ignoring")
	errEmptyPairParts = fmt.Errorf("empty pair parts")
)

// getWindowAndTree splits "arg" at the ':' seperator. in the context
// of *knut* the first part is called "window" (it is the url-endpoint,
// essentially), the part after the first ':' is called "the tree", it's
// the content that will be delivered.
func getWindowAndTree(arg string) (window, tree string, err error) {

	// if the user just feeds in a list of (existing) filenames
	// we assume the user just wants to expose the given filenames
	// as they are. eg, `ls *.go | xargs knut` and be done with it.
	if fi, err := os.Stat(arg); err == nil {
		if fi.IsDir() {
			return "/" + fi.Name() + "/", arg, nil
		}
		return "/" + fi.Name(), arg, nil
	}

	var parts []string
	if parts = strings.SplitN(arg, ":", 2); len(parts) != 2 {
		return "", "", errMissingSep
	}
	if window, tree = parts[0], parts[1]; window == "" || tree == "" {
		return "", "", errEmptyPairParts
	}

	return window, tree, nil
}

func localFilename(fileURL *url.URL) string {
	return filepath.Join(fileURL.Host, fileURL.Path)
}

// writeStatus renders the given status code and
// a text associated with that code
func writeStatus(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "%d: %s", code, http.StatusText(code))
}

func hasQueryParam(key string, vals url.Values) bool {
	_, exists := vals[key]
	return exists
}
