// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package knut

import (
	"flag"
	"fmt"
)

const usageText = `
knut [opts] [uri:]folder-or-file [mapping2] [mapping3] [...]

Sample:

   knut file.txt /this/:. /ding.txt:/tmp/dong.txt

Mapping Format:

   file.txt                - publish the file "file.txt" via "/file.txt"
   /:.                     - list contents of current directory via "/"
   /uri:folder             - list contents of "folder" via "/uri"
   /uri:file               - serve "file" via "/uri"
   /uri:@text              - respond with "text" at "/uri"
   30x/uri:location        - respond with 301 at "/uri"
   @/upload:folder         - accept multipart encoded data via POST at "/upload"
                             and store it inside "folder". A simple upload form
                             is rendered on GET.
   /c.tgz:tar+gz://./      - creates a (gzipped) tarball from the current directory
                             and serves it via "/c.tgz"
   /z.zip:zip://./         - creates a zip files from the current directory
                             and serves it via "/z.zip"
   /z.zip:zipfs://a.zip    - list and servce the content of the entries of an
                             existing "z.zip" via the "/z.zip": consider a file
                             "example.txt" inside "z.zip", it will be directly
                             available via "/z.zip/example.txt"
   /uri:http://1.2.3.4/    - creates a reverse proxy and forwards requests to /uri
                             to the given http-host
   /uri:git://folder/      - serves files via "git http-backend"
   /uri:cgit://path/to/dir - serves git-repos via "cgit"
   /uri:myip://            - serves a "myip" endpoint

`

func printUsage(fs *flag.FlagSet) {
	w := fs.Output()
	fmt.Fprintln(w, usageText, "Options:")
	fmt.Fprintln(w)
	fs.PrintDefaults()
	fmt.Fprintln(w)
}
