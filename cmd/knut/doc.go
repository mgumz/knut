// generated, do NOT edit.
//
//go:generate go run -v ./gen_doc.go -o doc.go

/*
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

Logins:

	  -auth name:password       - every window asks for this login
	  -auth /uri=name:password  - "/uri" asks for this login instead
	  -auth-file knut.auth      - the same entries, one per line, "#" starts a
	                              comment. the file must be closed to other
	                              users: chmod 600

	  a password is plain text, "env:NAME" (read from the environment) or a
	  bcrypt hash: "knut -hash name" makes one, so does "htpasswd -nB name"

	Options:

	 -auth entry
	       ask for a login, an entry is '[/window=]name:password',
	       repeatable (see "Logins")
	 -auth-file file
	       read -auth entries from file, one per line, repeatable
	 -bind string
	       address to bind to (default ":8080")
	 -compress
	       handle "Accept-Encoding" = "gzip,deflate" (default true)
	 -hash name
	       print an entry with a bcrypt hash for name and exit,
	       the password is asked for, or read from stdin
	 -live
	       listings refresh themselves, uploads report progress (serves htmx at "/.knut/htmx.js")
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
	       use a self-signed in-memory cert+key, valid while knut runs, to drive tls.
	       "=keep,for=30d" adds attributes, separated by ",":
	       keep - store cert+key in the user cache folder, reuse them on the next start
	       for=<duration> - expire the cert, "30d" or "12h"
	 -version
	       print version
*/
package main
