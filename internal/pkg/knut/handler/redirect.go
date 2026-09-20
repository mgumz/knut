package handler

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/mgumz/knut/internal/pkg/knut/view"
)

var redirectTmpl = view.Template("redirect")

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

		target := buf.String()
		if r.Method != http.MethodGet {
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		if escaped, err := url.Parse(target); err == nil {
			target = escaped.String()
		}

		w.Header().Set("Location", target)
		view.WriteStatus(w, http.StatusMovedPermanently, redirectTmpl, struct {
			view.Page
			Location string
		}{Page: view.NewPage(""), Location: target})

	})
}
