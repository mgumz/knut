// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

// zip64ExtraID tags the extra field carrying the sizes and the offset of an
// entry beyond 4GiB. the writer adds its own where the copy needs one.
const zip64ExtraID = 0x0001

// zipUTF8Flag says the name of an entry is utf-8 rather than cp437.
const zipUTF8Flag = 0x800

// zipFSFolder answers "r" with "folder" inside the zip at "name" as a zip
// of its own. "reported" is the zip as the user spelled it, the one an
// error names.
//
// the entries are copied across as they are stored: compressed bytes, crc
// and sizes go from one archive into the other without being inflated and
// deflated again. what knut spends on it is the copy, and an entry packed
// by a method go cannot unpack, or an encrypted one, arrives exactly as it
// was stored.
//
// the download is laid out like the zip of a folder on disk, see
// zipFolder: named after the folder, everything below one folder of that
// name.
func zipFSFolder(w http.ResponseWriter, r *http.Request, fsys fs.FS, name, reported, folder string) {

	z, file, err := openZip(fsys, name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "error: %q: %v\n", reported, err)
		return
	}
	defer file.Close()

	if !zipHas(z, folder) {
		view.Status(w, http.StatusNotFound)
		return
	}

	base := zipBaseName(r)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", zipDisposition(base))

	zw := zip.NewWriter(w)

	from := ""
	if folder != "" {
		from = folder + "/"
	}

	for _, entry := range z.File {
		rel, ok := strings.CutPrefix(entry.Name, from)
		if !ok || rel == "" {
			continue
		}
		if err := zipRawEntry(zw, entry, base+"/"+rel); err != nil {
			fmt.Fprintf(os.Stderr, "warning: writing zip of %q in %q: %v\n", folder, reported, err)
			return
		}
	}

	if err := zw.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: finishing zip of %q in %q: %v\n", folder, reported, err)
	}
}

// zipRawEntry copies "entry" into "zw" as "name", compressed as it is.
//
// an entry is skipped where it would not survive being handed on:
//
//   - a name which climbs out of the folder it is unpacked into, or which
//     carries a "\" an unpacker on windows reads as a separator. a zip is
//     free to carry such names, knut does not pass them along
//   - anything which is neither a folder nor a regular file, see zipTree
//
// errors handed back are the ones from writing the archive, see zipTree.
func zipRawEntry(zw *zip.Writer, entry *zip.File, name string) error {

	isDir := strings.HasSuffix(entry.Name, "/")

	if !fs.ValidPath(strings.TrimSuffix(entry.Name, "/")) || strings.ContainsRune(entry.Name, '\\') {
		fmt.Fprintf(os.Stderr, "warning: skipping %q for a zip: unsafe name\n", entry.Name)
		return nil
	}
	if !isDir && !entry.Mode().IsRegular() {
		return nil
	}

	raw, err := entry.OpenRaw()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: opening %q for a zip: %v\n", entry.Name, err)
		return nil
	}

	// the header travels whole - method, flags, crc, sizes, timestamps,
	// attributes and the extras an unpacker may need, like the one aes
	// encryption keeps its parameters in. two things do not fit the copy:
	// a zip64 extra names the offset in the zip it came from, and the name
	// is not the one the utf-8 flag was set for.
	fh := entry.FileHeader
	fh.Name = name
	fh.Extra = withoutZip64(entry.Extra)
	if !isASCII(name) && utf8.ValidString(name) {
		fh.Flags |= zipUTF8Flag
	}

	out, err := zw.CreateRaw(&fh)
	if err != nil {
		return err
	}
	if isDir {
		return nil
	}

	_, err = io.Copy(out, raw)

	return err
}

// withoutZip64 hands back the extra fields in "extra" but the zip64 one. a
// field running past the end is dropped with the rest of it.
func withoutZip64(extra []byte) []byte {

	kept := make([]byte, 0, len(extra))
	for len(extra) >= 4 {
		id, size := binary.LittleEndian.Uint16(extra), int(binary.LittleEndian.Uint16(extra[2:]))
		if 4+size > len(extra) {
			break
		}
		if id != zip64ExtraID {
			kept = append(kept, extra[:4+size]...)
		}
		extra = extra[4+size:]
	}

	return kept
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
