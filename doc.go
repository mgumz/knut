/*
	*knut* is a tiny webserver which serves folders (file trees) via user
	specified urls. it grew a little bit beyond that scope and now serves
	also single files and accepts file-uploads.

	usage:

	 knut [opts] uri:folder [uri2:file1] [@upload:upload_folder] [...]

	sample:

	  knut /:. /ding.txt:/tmp/dong.txt

	options:

	 -auth="": use 'name:password' to require
	 -bind=":8080": address to bind to
	 -compress=true: handle "Accept-Encoding" = "gzip,deflate"
	 -log=true: log requests to stdout
	 -server-id="knut/1.0": add "Server: <val-here>" to the response
	 -tls-cert="": use given cert to start tls
	 -tls-key="": use given key to start tls
	 -tls-onetime=false: use a onetime-in-memory cert+key to drive tls

	the name:

	*knut* or "st.knut's day" is an annually celebrated festival in sweden /
	finland on 13 January. it marks the end of christmas, among other
	activities the christmas trees are disposed.
*/
package main
