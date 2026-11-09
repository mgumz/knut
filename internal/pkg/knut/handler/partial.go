// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut"
)

// publishedDir is a tree on disk as knut publishes it: an http.Dir which
// does not know knut.UploadPartialDir. whatever reads the tree through it
// - the listing, the files, the zip of a folder - cannot reach the files
// an upload is still writing.
type publishedDir http.Dir

func (d publishedDir) Open(name string) (http.File, error) {

	if isPartial(name) {
		return nil, os.ErrNotExist
	}

	file, err := http.Dir(d).Open(name)
	if err != nil {
		return nil, err
	}

	return partialHidingFile{file}, nil
}

// isPartial reports whether "name", a slash separated path, is in or is a
// knut.UploadPartialDir.
func isPartial(name string) bool {

	for _, segment := range strings.Split(name, "/") {
		if segment == knut.UploadPartialDir {
			return true
		}
	}

	return false
}

type partialHidingFile struct {
	http.File
}

// Readdir leaves knut.UploadPartialDir out. asked for "n" entries it keeps
// reading until it has some or the folder is through: an empty answer
// without an error would read as a broken folder.
func (f partialHidingFile) Readdir(n int) ([]fs.FileInfo, error) {

	for {
		infos, err := f.File.Readdir(n)
		kept := infos[:0]
		for _, info := range infos {
			if info.Name() != knut.UploadPartialDir {
				kept = append(kept, info)
			}
		}
		if len(kept) > 0 || len(infos) == 0 || n <= 0 || err != nil {
			return kept, err
		}
	}
}

// skipPartial is for the walks which pack a folder: it skips
// knut.UploadPartialDir and all below it.
func skipPartial(info fs.FileInfo) bool {
	return info != nil && info.IsDir() && info.Name() == knut.UploadPartialDir
}
