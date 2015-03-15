// Copyright 2015 Mathias Gumz. All rights reserved. Use of this source code
// is governed by a BSD-style license that can be found in the LICENSE file.

package main

// *knut* is a tiny webserver which serves folders (file trees) via user
// specified urls. it grew a little bit beyond that scope and now serves
// also single files and accepts file-uploads.
//
// the name:
//
// *knut* or "st.knut's day" is an annually celebrated festival in sweden /
// finland on 13 January. it marks the end of christmas, among other
// activities the christmas trees are disposed.

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var version = "knut/1.0"

func usage() {
	flag.CommandLine.SetOutput(os.Stdout)
	fmt.Println(`
knut [opts] uri:folder [uri2:file1] [@upload:upload_folder] [...]

Sample:

   knut /:. /ding.txt:/tmp/dong.txt

Options:
	`)
	flag.PrintDefaults()
	fmt.Println()
}

func main() {

	opts := struct {
		bindAddr    string
		doLog       bool
		doAuth      string
		doCompress  bool
		addServerId string
		tlsOnetime  bool
		tlsCert     string
		tlsKey      string
	}{
		bindAddr:    ":8080",
		doLog:       true,
		doCompress:  true,
		addServerId: version,
	}

	flag.StringVar(&opts.bindAddr, "bind", opts.bindAddr, "address to bind to")
	flag.BoolVar(&opts.doLog, "log", opts.doLog, "log requests to stdout")
	flag.BoolVar(&opts.doCompress, "compress", opts.doCompress, `handle "Accept-Encoding" = "gzip,deflate"`)
	flag.StringVar(&opts.doAuth, "auth", "", "use 'name:password' to require")
	flag.StringVar(&opts.addServerId, "server-id", opts.addServerId, `add "Server: <val-here>" to the response`)
	flag.BoolVar(&opts.tlsOnetime, "tls-onetime", opts.tlsOnetime, "use a onetime-in-memory cert+key to drive tls")
	flag.StringVar(&opts.tlsKey, "tls-key", opts.tlsKey, "use given key to start tls")
	flag.StringVar(&opts.tlsCert, "tls-cert", opts.tlsCert, "use given cert to start tls")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "error: missing mapping\n")
		usage()
		os.Exit(1)
	}

	tree, nHandlers := prepareTrees(http.NewServeMux(), flag.Args())
	if nHandlers == 0 {
		fmt.Fprintf(os.Stderr, "error: not one valid mapping given\n")
		usage()
		os.Exit(1)
	}

	//
	// setup the chain of handlers
	//
	var h = http.Handler(tree)

	if opts.addServerId != "" {
		h = AddServerId(h, opts.addServerId)
	}
	h = NoCache(h)
	if opts.doCompress {
		h = CompressHandler(h)
	}
	if opts.doAuth != "" {
		parts := strings.SplitN(opts.doAuth, ":", 2)
		if len(parts) == 0 {
			fmt.Fprintf(os.Stderr, "error: missing separator for argument to -auth")
			os.Exit(1)
		}
		h = BasicAuth(h, parts[0], parts[1])
	}
	if opts.doLog {
		h = LogRequests(h, os.Stdout)
	}

	//
	// and .. action.
	//
	var run func() error
	switch {
	case opts.tlsOnetime:
		onetime := &onetimeTLS{}
		if err := onetime.Create(opts.bindAddr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		run = func() error { return http.Serve(onetime.listener, h) }
	case opts.tlsCert != "" && opts.tlsKey != "":
		run = func() error { return http.ListenAndServeTLS(opts.bindAddr, opts.tlsCert, opts.tlsKey, h) }
	default:
		run = func() error { return http.ListenAndServe(opts.bindAddr, h) }
	}

	fmt.Printf("\nknut started on %s, be aware of the trees!\n", opts.bindAddr)
	if err := run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

// binds a list of uri:tree pairs to 'muxer'
func prepareTrees(muxer *http.ServeMux, mappings []string) (*http.ServeMux, int) {

	const (
		errMsgMissingSep  = "error: pair %d (%q) doesn't contain the uri-tree seprator ':', ignoring\n"
		errMsgInvalidPair = "error: invalid pair %d: uri: %q, tree: %q\n"
	)

	var (
		nHandlers, i int
		parts        []string
		uri, tree    string
		handler      http.Handler
		fi           os.FileInfo
		err          error
	)

	for i, tree = range mappings {

		if parts = strings.SplitN(tree, ":", 2); len(parts) != 2 {
			fmt.Fprintf(os.Stderr, errMsgMissingSep, i+1, tree)
			continue
		}

		if uri, tree = parts[0], parts[1]; uri == "" || tree == "" {
			fmt.Fprintf(os.Stderr, errMsgInvalidPair, i+1, uri, tree)
			continue
		}

		verb := "throws"
		switch {
		case uri[0] == '@': // indicates upload handler
			if uri = uri[1:]; uri == "" {
				fmt.Fprintf(os.Stderr, "error: post uri in pair %d is empty\n", i)
				continue
			}
			if fi, err = os.Stat(tree); err == nil && !fi.IsDir() {
				fmt.Fprintf(os.Stderr, "error: existing %q is not a directory\n", tree)
				continue
			}
			handler, verb = UploadInto(tree), "catches"
		case tree[0] == '@':
			handler = ServeString(tree[1:])
		default:
			if fi, err = os.Stat(tree); err == nil && !fi.IsDir() {
				handler = ServeFile(tree)
			} else {
				handler = http.FileServer(http.Dir(tree))
				handler = http.StripPrefix(uri, handler)
			}
		}

		fmt.Printf("knut %s %q through %q\n", verb, tree, uri)
		muxer.Handle(uri, handler)
		nHandlers++
	}
	return muxer, nHandlers
}

// create an in-memory root-certificate, sign it with an in-memory private key
// (elliptic curve521), and create a tlsListener based upon that certificate.
// it's only purpose is to have a tls-cert with a onetime, throw-away
// certificate. fyi: http://safecurves.cr.yp.to/
type onetimeTLS struct {
	listener     net.Listener
	privKey      *ecdsa.PrivateKey
	sn           *big.Int
	template     *x509.Certificate
	privKeyBytes []byte
	certBytes    []byte
	tlsConfig    tls.Config
	err          error
}

func (ot *onetimeTLS) Create(addr string) error {

	ot.createListener(addr)
	ot.createPrivateKey()
	ot.createSerialNumber(128)
	ot.createTemplate()
	ot.createPKBytes()
	ot.createCertBytes()
	ot.fillTLSConfig()

	if ot.err != nil {
		return ot.err
	}

	ot.listener = tls.NewListener(ot.listener, &ot.tlsConfig)
	return nil
}

func (ot *onetimeTLS) createListener(addr string) {
	if ot.err == nil {
		ot.listener, ot.err = net.Listen("tcp", addr)
	}
}
func (ot *onetimeTLS) createPrivateKey() {
	if ot.err == nil {
		ot.privKey, ot.err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	}
}
func (ot *onetimeTLS) createSerialNumber(n uint) {
	if ot.err == nil {
		ot.sn, ot.err = rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), n))
	}
}
func (ot *onetimeTLS) createTemplate() {
	if ot.err == nil {
		ot.template = &x509.Certificate{
			SerialNumber: ot.sn,
			Issuer: pkix.Name{
				Organization:       []string{"Association of united Knuts"},
				OrganizationalUnit: []string{"Knut CA of onetime, throw-away certificates"},
				CommonName:         "Onetime, throwaway certificate of *knut*",
			},
			Subject: pkix.Name{
				Organization:       []string{"Knut"},
				OrganizationalUnit: []string{"Knut operators"},
				CommonName:         "Onetime, throwaway certificate of *knut*",
			},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().Add(24 * time.Hour),
			KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
	}
}
func (ot *onetimeTLS) createPKBytes() {
	if ot.err == nil {
		ot.privKeyBytes, ot.err = x509.MarshalECPrivateKey(ot.privKey)
	}
	if ot.err == nil {
		ot.privKeyBytes = bytesToPem(ot.privKeyBytes, "EC PRIVATE KEY")
	}
}
func (ot *onetimeTLS) createCertBytes() {
	if ot.err == nil {
		ot.certBytes, ot.err = x509.CreateCertificate(rand.Reader, ot.template, ot.template, &ot.privKey.PublicKey, ot.privKey)
	}
	if ot.err == nil {
		ot.certBytes = bytesToPem(ot.certBytes, "CERTIFICATE")
	}
}
func (ot *onetimeTLS) fillTLSConfig() {
	if ot.err == nil {
		ot.tlsConfig.NextProtos = []string{"http/1.1"}
		ot.tlsConfig.MinVersion = tls.VersionTLS11
		ot.tlsConfig.SessionTicketsDisabled = true
		ot.tlsConfig.Certificates = make([]tls.Certificate, 1)
		ot.tlsConfig.Certificates[0], ot.err = tls.X509KeyPair(ot.certBytes, ot.privKeyBytes)
	}
}

func bytesToPem(in []byte, blockType string) []byte {
	buf := bytes.NewBuffer(nil)
	pem.Encode(buf, &pem.Block{Type: blockType, Bytes: in})
	return buf.Bytes()
}

//
// Handler
//
//

// Log all requests to 'writer', capture also the status-code
func LogRequests(handler http.Handler, logWriter io.Writer) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sc = statusCodeCapture{w: w}
		handler.ServeHTTP(&sc, r)
		if sc.code == 0 {
			sc.code = 200
		}
		port_sep := strings.LastIndex(r.RemoteAddr, ":")
		fmt.Fprintf(logWriter, "%s\t%s\t%d\t%s\t%s%s\n",
			time.Now().Format(time.RFC3339), r.RemoteAddr[:port_sep], sc.code, r.Method, r.Host, r.RequestURI)
	})
}

// log requests to stderr. capture the status.code by wrapping
// the handler
type statusCodeCapture struct {
	w    http.ResponseWriter
	code int
}

func (sc *statusCodeCapture) Header() http.Header            { return sc.w.Header() }
func (sc *statusCodeCapture) Write(data []byte) (int, error) { return sc.w.Write(data) }
func (sc *statusCodeCapture) WriteHeader(code int)           { sc.code = code; sc.w.WriteHeader(code) }

// Handle requests having Accept-Encoding 'deflate' or 'gzip'
// taken from https://github.com/gorilla/handlers/blob/master/compress.go
func CompressHandler(handler http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, enc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
			switch strings.TrimSpace(enc) {
			case "deflate":
				fw, _ := flate.NewWriter(w, flate.DefaultCompression)
				defer fw.Close()
				w.Header().Set("Content-Encoding", "deflate")
				w = &cWriter{Writer: fw, ResponseWriter: w}
				goto L
			case "gzip":
				gz := gzip.NewWriter(w)
				defer gz.Close()
				w.Header().Set("Content-Encoding", "gzip")
				w = &cWriter{Writer: gz, ResponseWriter: w}
				goto L
			}
		}
	L:
		handler.ServeHTTP(w, r)
	})
}

type cWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w *cWriter) WriteHeader(code int)           { w.ResponseWriter.WriteHeader(code) }
func (w *cWriter) Write(data []byte) (int, error) { return w.Writer.Write(data) }
func (w *cWriter) Header() http.Header            { return w.ResponseWriter.Header() }

//
// add 'Sever' to the response header
func AddServerId(handler http.Handler, server_id string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", server_id)
		handler.ServeHTTP(w, r)
	})
}

//
// add a 'nocaching, please' hint to the response header
func NoCache(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, max-age=0, no-cache")
		handler.ServeHTTP(w, r)
	})
}

//
// checks the submited username and password against predefined values.
func BasicAuth(handler http.Handler, username, password string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rUser, rPassword, ok := r.BasicAuth()
		if ok && rUser == username && password == rPassword {
			handler.ServeHTTP(w, r)
		} else {
			w.Header().Set("WWW-Authenticate", `Basic realm="knut"`)
			WriteStatus(w, http.StatusUnauthorized)
		}
	})
}

//
// serve a single file
func ServeFile(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, name)
	})
}

//
// serve 'str'
func ServeString(str string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, str)
	})
}

//
// handle upload-requests to 'dir':
//
// GET - render an upload form
// POST - write the submitted files into 'dir'
func UploadInto(dir string) http.Handler {

	const htmlDoc = `<!doctype html>
<head>
	<title>knut - file upload</title>
	<style type="text/css">
* { font-family: monospace }
input[type="submit"] { margin-top: 1em }
	</style>
</head>
<h1>knut - file upload</h1>`

	const uploadForm = `<form method="post" enctype="multipart/form-data">
	<div>
		<div><input type="file" name="upload_file"></div>
	</div>
	<div>
		<input type="submit" value="Upload">
	</div>
</form>
`

	os.MkdirAll(dir, 0777)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.Header().Set("Content-Type", "text/html")
		case "GET":
			defer fmt.Fprintln(w, htmlDoc, uploadForm)
			fallthrough
		case "HEAD":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Length", strconv.Itoa(len(uploadForm)))
			return
		default:
			WriteStatus(w, http.StatusMethodNotAllowed)
			return
		}

		startTime := time.Now()
		r.ParseMultipartForm(4096)

		if r.MultipartForm == nil {
			WriteStatus(w, http.StatusBadRequest)
			return
		}

		var nBytes int64
		for _, files := range r.MultipartForm.File {
			for _, fh := range files {
				prefix := genNamePrefix(r.RemoteAddr, fh.Filename)
				n, err := storeFormFile(prefix, dir, fh)
				if err != nil {
					log.Printf("warning: %v", err)
				}
				nBytes += n
			}
		}

		fmt.Fprintln(w, htmlDoc)
		fmt.Fprintf(w, "ok, received %d bytes over %s", nBytes, time.Now().Sub(startTime))
	})
}

func genNamePrefix(hostport, name string) string {
	host, port, _ := net.SplitHostPort(hostport)
	ip := net.ParseIP(host)
	base := filepath.Base(name)
	return fmt.Sprintf("%x_%s_%s_", ip, port, base)
}

func storeFormFile(prefix, dir string, fh *multipart.FileHeader) (int64, error) {

	postedFile, err := fh.Open()
	if err != nil {
		return 0, err
	}
	defer postedFile.Close()

	osFile, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		return 0, err
	}
	defer osFile.Close()
	return io.Copy(osFile, postedFile)
}

// small helper to render the given status code
func WriteStatus(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "%d: %s", code, http.StatusText(code))
}
