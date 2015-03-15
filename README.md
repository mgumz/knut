# *knut* - a tiny webserver which throws (file-) trees through a window

I want to make 'folder1' and 'file2' of my home directory public, without
serving both resources on different http-ports. I also wanted adhoc mapping
of folders to uris, without any complex config files or any other voodoo.

And sometimes it's quite handy to just tell someone to POST stuff to a
httpd-resource.


## Usage

    knut [opts] uri:folder [uri2:file1] [@upload:upload_folder] [...]

    Sample:

       knut /:. /ding.txt:/tmp/dong.txt

    Options:

      -auth="": use 'name:password' to require
      -bind=":8080": address to bind to
      -compress=true: handle "Accept-Encoding" = "gzip,deflate"
      -log=true: log requests to stdout
      -server-id="knut/1.0": add "Server: <val-here>" to the response
      -tls-cert="": use given cert to start tls
      -tls-key="": use given key to start tls
      -tls-onetime=false: use a onetime-in-memory cert+key to drive tls


### File uploads

To create an endpoint to POST things to, use `@/endpoint:storage_folder`. All
multiform encoded data sent to that endpoint will end up in the `storage_folder`.
If that folder is nonexistant yet, it will be created upon start.

If you hit the endpoint via GET, a simple upload-form will be rendered.


## Build & Installing

The only requirement to build *knut*: A working go-compiler. Check
http://golang.org for more information on how to setup one. If you
have a working golang-compiler:

    $> export GOPATH=`pwd`
    $> go get github.com/mgumz/knut
    $> ./bin/knut -h

If you *need* to install something:

    $> cp ./bin/knut /path/to/final/place

## The name

*knut* or "St. Knut's day" is an annually celebrated festival in sweden /
finland on 13 January. It marks the end of christmas. Among other
activities the christmas trees are disposed. Get inspired:

* https://www.youtube.com/watch?v=M2URddFDIcc
* https://www.youtube.com/watch?v=OGpGGONbTwY
* https://www.youtube.com/watch?v=nEf5yuyaXgk
