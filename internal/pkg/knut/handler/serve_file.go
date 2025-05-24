package handler

import "net/http"

// serveFileHandler serves the file given by 'name'
func ServeFileHandler(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, name)
	})
}
