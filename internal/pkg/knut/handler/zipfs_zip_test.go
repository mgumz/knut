// Copyright 2026 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rawItem is one entry of a zip written for a test as it is stored: the
// bytes are put into the zip untouched, the header says what they are.
type rawItem struct {
	header zip.FileHeader
	stored string
}

// writeRawZip puts a zip carrying "entries" at a fresh path and names it.
func writeRawZip(t *testing.T, entries ...rawItem) string {
	t.Helper()

	buf := bytes.NewBuffer(nil)
	zw := zip.NewWriter(buf)

	for _, entry := range entries {
		header := entry.header
		header.CompressedSize64 = uint64(len(entry.stored))
		w, err := zw.CreateRaw(&header)
		if err != nil {
			t.Fatalf("adding %q: %v", header.Name, err)
		}
		if _, err := io.WriteString(w, entry.stored); err != nil {
			t.Fatalf("writing %q: %v", header.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing the zip: %v", err)
	}

	name := filepath.Join(t.TempDir(), "raw.zip")
	if err := os.WriteFile(name, buf.Bytes(), 0600); err != nil {
		t.Fatalf("writing %q: %v", name, err)
	}

	return name
}

// storedItem is a file kept as it is, the crc and size of it filled in.
func storedItem(name, content string) rawItem {
	return rawItem{
		header: zip.FileHeader{
			Name:               name,
			Method:             zip.Store,
			CRC32:              crc32.ChecksumIEEE([]byte(content)),
			UncompressedSize64: uint64(len(content)),
		},
		stored: content,
	}
}

// zipEntryOf finds "name" in "reader", failing the test where it is not.
func zipEntryOf(t *testing.T, reader *zip.Reader, name string) *zip.File {
	t.Helper()

	for _, file := range reader.File {
		if file.Name == name {
			return file
		}
	}
	t.Fatalf("the zip carries no %q, only %q", name, zipNames(reader))
	return nil
}

// rawBytes reads what "file" stores, compressed as it is.
func rawBytes(t *testing.T, file *zip.File) []byte {
	t.Helper()

	raw, err := file.OpenRaw()
	if err != nil {
		t.Fatalf("opening %q raw: %v", file.Name, err)
	}
	b, err := io.ReadAll(raw)
	if err != nil {
		t.Fatalf("reading %q raw: %v", file.Name, err)
	}
	return b
}

func TestZipFSZipStreamsFolder(t *testing.T) {

	rec := get(ZipFSHandler(testZip(t), "", ""), "/sub/?zip")

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/zip"; got != want {
		t.Errorf("got content type %q, want %q", got, want)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, `filename=sub-`) {
		t.Errorf("got disposition %q, want one named after the folder", got)
	}

	reader := zipOf(t, rec.Body.Bytes())
	if got, want := strings.Join(zipNames(reader), " "), "sub/b.txt"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}

	// read back through the checksum the entry carries
	r, err := zipEntryOf(t, reader, "sub/b.txt").Open()
	if err != nil {
		t.Fatalf("opening the entry: %v", err)
	}
	if b, err := io.ReadAll(r); err != nil || string(b) != "knut knut" {
		t.Errorf("got %q (%v), want %q", b, err, "knut knut")
	}
}

// the root takes everything along, explicit folders included
func TestZipFSZipWholeTree(t *testing.T) {

	reader := zipOf(t, get(ZipFSHandler(testZip(t), "", ""), "/?zip").Body.Bytes())

	if got, want := strings.Join(zipNames(reader), " "), "knut/a.txt knut/empty/ knut/sub/b.txt"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}
}

// a prefix narrows the zip the same way it narrows the listing
func TestZipFSZipBehindPrefix(t *testing.T) {

	reader := zipOf(t, get(ZipFSHandler(testZip(t), "sub", ""), "/?zip").Body.Bytes())

	if got, want := strings.Join(zipNames(reader), " "), "knut/b.txt"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}
}

// a folder the zip does not carry has no zip either - nor does one which
// only shares the start of a name
func TestZipFSZipMissing(t *testing.T) {

	for _, target := range []string{"/nope/?zip", "/su/?zip"} {
		if rec := get(ZipFSHandler(testZip(t), "", ""), target); rec.Code != http.StatusNotFound {
			t.Errorf("%q: got status %d, want %d", target, rec.Code, http.StatusNotFound)
		}
	}
}

// a file asked for its zip is the file
func TestZipFSZipOfFile(t *testing.T) {

	rec := get(ZipFSHandler(testZip(t), "", ""), "/a.txt?zip")

	if got, want := rec.Body.String(), "knut"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// an index shadowing the folder does not shadow its zip
func TestZipFSZipPastIndexPage(t *testing.T) {

	name := filepath.Join(t.TempDir(), "site.zip")
	writeZip(t, name, zipItem{"index.html", "hand written"}, zipItem{"b.txt", "knut"})

	reader := zipOf(t, get(ZipFSHandler(name, "", "index.html"), "/?zip").Body.Bytes())

	if got, want := strings.Join(zipNames(reader), " "), "knut/b.txt knut/index.html"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}
}

// the entries are copied as they are stored, not unpacked and packed again:
// a deflated one keeps its bytes, a stored one stays stored, and one packed
// by a method go cannot unpack arrives all the same
func TestZipFSZipCopiesRaw(t *testing.T) {

	bzip2 := storedItem("odd.bz2", "not really bzip2")
	bzip2.header.Method = 12

	name := writeRawZip(t, storedItem("kept.txt", "knut knut knut"), bzip2)

	src, err := zip.OpenReader(name)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	defer src.Close()

	reader := zipOf(t, get(ZipFSHandler(name, "", ""), "/?zip").Body.Bytes())

	for _, want := range src.File {
		got := zipEntryOf(t, reader, "knut/"+want.Name)
		if got.Method != want.Method {
			t.Errorf("%q: got method %d, want %d", want.Name, got.Method, want.Method)
		}
		if got.CRC32 != want.CRC32 {
			t.Errorf("%q: got crc %08x, want %08x", want.Name, got.CRC32, want.CRC32)
		}
		if !bytes.Equal(rawBytes(t, got), rawBytes(t, want)) {
			t.Errorf("%q: the stored bytes changed on the way", want.Name)
		}
	}

	// and the deflated ones of an ordinary zip
	deflated := testZip(t)
	src2, err := zip.OpenReader(deflated)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	defer src2.Close()

	reader = zipOf(t, get(ZipFSHandler(deflated, "", ""), "/?zip").Body.Bytes())
	want := zipEntryOf(t, &src2.Reader, "sub/b.txt")
	got := zipEntryOf(t, reader, "knut/sub/b.txt")
	if got.Method != zip.Deflate || !bytes.Equal(rawBytes(t, got), rawBytes(t, want)) {
		t.Error("a deflated entry was packed again")
	}
}

// what would not survive being unpacked somewhere else is not handed on
func TestZipFSZipSkipsUnsafe(t *testing.T) {

	link := storedItem("link", "a.txt")
	link.header.SetMode(fs.ModeSymlink | 0777)

	name := writeRawZip(t,
		storedItem("a.txt", "knut"),
		storedItem("../evil.txt", "knut"),
		storedItem(`back\slash.txt`, "knut"),
		storedItem("/abs.txt", "knut"),
		link)

	reader := zipOf(t, get(ZipFSHandler(name, "", ""), "/?zip").Body.Bytes())

	if got, want := strings.Join(zipNames(reader), " "), "knut/a.txt"; got != want {
		t.Errorf("got entries %q, want %q", got, want)
	}
}

// the extras travel along, the zip64 one does not: it names an offset in
// the zip the entry came from
func TestZipFSZipDropsZip64Extra(t *testing.T) {

	extra := binary.LittleEndian.AppendUint16(nil, zip64ExtraID)
	extra = binary.LittleEndian.AppendUint16(extra, 8)
	extra = binary.LittleEndian.AppendUint64(extra, 1<<40)
	kept := binary.LittleEndian.AppendUint16(nil, 0xcafe)
	kept = binary.LittleEndian.AppendUint16(kept, 2)
	kept = append(kept, 'k', 'n')

	item := storedItem("a.txt", "knut")
	item.header.Extra = append(extra, kept...)

	reader := zipOf(t, get(ZipFSHandler(writeRawZip(t, item), "", ""), "/?zip").Body.Bytes())

	if got := zipEntryOf(t, reader, "knut/a.txt").Extra; !bytes.Equal(got, kept) {
		t.Errorf("got extra %x, want %x", got, kept)
	}
}

// a name spelled in utf-8 says so, even where the zip it came from did not
// have to
func TestZipFSZipFlagsUTF8Names(t *testing.T) {

	item := storedItem("ünï/a.txt", "knut")
	item.header.NonUTF8 = true

	reader := zipOf(t, get(ZipFSHandler(writeRawZip(t, item), "", ""), "/ünï/?zip").Body.Bytes())

	if got := zipEntryOf(t, reader, "ünï/a.txt"); got.Flags&zipUTF8Flag == 0 {
		t.Errorf("got flags %04x, want the utf-8 one set", got.Flags)
	}
}

// the header offers the folder on screen, every folder row its own zip
func TestZipFSListingZipLinks(t *testing.T) {

	body := get(ZipFSHandler(testZip(t), "", ""), "/").Body.String()

	for _, want := range []string{`<a class="zip" href="?zip"`, `href="sub/?zip"`, `href="empty/?zip"`} {
		if !strings.Contains(body, want) {
			t.Errorf("a listing of a zip does not offer %q", want)
		}
	}
	if strings.Contains(body, `href="a.txt?zip"`) {
		t.Error("a file inside a zip is offered as a zip")
	}
}
