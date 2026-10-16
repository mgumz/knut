//go:build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut"
)

func main() {

	oname := flag.String("o", "", "name of generated file")
	flag.Parse()

	kf := flag.NewFlagSet("knut", flag.ContinueOnError)
	knut.SetupFlags(kf)
	b := bytes.NewBuffer(nil)
	kf.SetOutput(b)
	kf.Usage()

	docs := b.String()
	docs = strings.ReplaceAll(docs, "\t", "    ")

	src := bytes.NewBuffer(nil)
	fmt.Fprintln(src, "// generated, do NOT edit.")
	fmt.Fprintln(src, "//")
	fmt.Fprintln(src, "//go:generate go run -v ./gen_doc.go -o doc.go")
	fmt.Fprintln(src)
	fmt.Fprintln(src, "/*")
	fmt.Fprintln(src, docs)
	fmt.Fprintln(src, "*/")
	fmt.Fprintln(src, "package main")

	// the usage text becomes the package doc comment, which gofmt reformats
	out, err := format.Source(src.Bytes())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ofile := os.Stdout
	if *oname != "" {
		ofile, _ = os.Create(*oname)
		defer ofile.Close()
	}
	ofile.Write(out)
}
