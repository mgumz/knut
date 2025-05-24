# *knut* - a tiny webserver which throws (file-) trees through a window

[![GitHub Release](https://img.shields.io/github/v/release/mgumz/knut.svg)](https://github.com/mgumz/knut/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/mgumz/knut)](https://goreportcard.com/report/github.com/mgumz/knut)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/mgumz/knut.svg)](https://github.com/mgumz/knut)

I want to make 'folder1' and 'file2' of my home directory public, without
serving both resources on different http-ports, without exposing the rest
of my $HOME, without any copying of the files or folders to another place.

I also want to map resources to URIs, adhoc, without any complex config files
or any other voodoo.

And sometimes it's quite handy to just tell someone to POST stuff to a
httpd-resource.

![knut Screenshot](./media/knut.png)


## Usage

```
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
   /uri:myip://            - serves a "myip" endpoint, query-options:
                             fuzzy - /24 for ipv4; /56 for ipv6
                             info - api to use for meta data about the ip
                             supported: "ripe"

 Options:

  -auth string
    	use 'name:password' to require
  -bind string
    	address to bind to (default ":8080")
  -compress
    	handle "Accept-Encoding" = "gzip,deflate" (default true)
  -log
    	log requests to stdout (default true)
  -select-addr
    	interactively select -bind address
  -serve-index
    	create a small index-page, listing the various paths
  -server-id string
    	add "Server: <val-here>" to the response (default "knut/dev-build")
  -show-qr
    	show a QR code to stdout pointing to '/' (useful only if -bind is distinct)
  -tee-body
    	dump request.body to stdout
  -tls-cert string
    	use given cert to start tls
  -tls-key string
    	use given key to start tls
  -tls-onetime
    	use a onetime-in-memory cert+key to drive tls
  -version
    	print version
```

## Build & Installing

The only requirement to build *knut*: A working go-compiler. Check
http://golang.org for more information on how to setup one. If you
have a working golang-compiler:

    $> go install github.com/mgumz/knut@latest
    $> ~/go/bin/knut -h

If you *need* to install something:

    $> cp ~/go/bin/knut /path/to/final/place

## The name

*knut* or "St. Knut's day" is an annually celebrated festival in sweden /
finland on 13 January. It marks the end of christmas. Among other
activities, the christmas trees 🎄🎄 are disposed. Get inspired:

* https://youtu.be/watch?v=OGpGGONbTwY
* https://youtu.be/watch?v=nEf5yuyaXgk
* https://youtu.be/watch?v=IBhC4UevoaA
