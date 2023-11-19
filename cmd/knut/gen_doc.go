//go:build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	"github.com/mgumz/knut/internal/pkg/knut"
)

func main() {

	oname := flag.String("o", "", "name of generated file")
	flag.Parse()

	ofile := os.Stdout
	if *oname != "" {
		ofile, _ = os.Create(*oname)
		defer ofile.Close()
	}

	kf := flag.NewFlagSet("knut", flag.ContinueOnError)
	knut.SetupFlags(kf)
	b := bytes.NewBuffer(nil)
	kf.SetOutput(b)
	kf.Usage()
	fmt.Fprintln(ofile, "// generated, do not edit.")
	fmt.Fprintln(ofile)
	fmt.Fprintln(ofile, "package main")
	fmt.Fprintln(ofile)
	fmt.Fprintln(ofile, "/*")
	fmt.Fprintln(ofile, string(b.Bytes()))
	fmt.Fprintln(ofile, "*/")
}
