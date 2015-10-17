/*

knut [opts] uri:folder [mapping2] [mapping3] [...]

Sample:

   knut /:. /ding.txt:/tmp/dong.txt

Mapping Format:

   /:.                     - list contents of current directory via "/"
   /uri:folder             - list contents of "folder" via "/uri"
   /uri:file               - serve "file" via "/uri"
   /uri:@text              - respond with "text" at "/uri"
   @/upload:folder         - accept multipart encoded data via POST at "/upload"
                             and store it inside "folder". A simple upload form
                             is rendered on GET.
   /c.tgz:tar+gz://./      - creates a (gzipped) tarball from the current directory
                             and serves it via "/c.tgz"
   /z.zip:zipfs://a.zip    - list and servce the content of the entries of an
                             existing "z.zip" via the "/z.zip": consider a file
                             "example.txt" inside "z.zip", it will be directly
                             available via "/z.zip/example.txt"
   /uri:http://1.2.3.4/    - creates a reverse proxy and forwards requests to /uri
                             to the given http-host
   /uri:git://folder/      - serves files via "git http-backend"
   /uri:cgit://path/to/dir - serves git-repos via "cgit"


Options:

  -auth="": use 'name:password' to require
  -bind=":8080": address to bind to
  -compress=true: handle "Accept-Encoding" = "gzip,deflate"
  -log=true: log requests to stdout
  -server-id="knut/1.0": add "Server: <val-here>" to the response
  -tls-cert="": use given cert to start tls
  -tls-key="": use given key to start tls
  -tls-onetime=false: use a onetime-in-memory cert+key to drive tls

*/
package main
