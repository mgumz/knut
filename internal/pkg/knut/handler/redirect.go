package handler

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"strings"
)

func RedirectHandler(path, location string) http.Handler {

	type uriHostPort struct {
		url.URL
		HostOnly string
		Port     string
	}

	var templ = template.Must(template.New(path).Parse(location))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var buf = bytes.NewBuffer(nil)
		var rUri, _ = url.Parse(r.RequestURI)
		var requestURI = uriHostPort{URL: *rUri}
		requestURI.Host, requestURI.HostOnly, requestURI.Port = r.Host, r.Host, "80"
		if i := strings.IndexByte(r.Host, ':'); i > -1 {
			requestURI.HostOnly = r.Host[:i]
			requestURI.Port = r.Host[i+1:]
		}
		templ.Execute(buf, &requestURI)
		http.Redirect(w, r, buf.String(), http.StatusMovedPermanently)
	})
}
