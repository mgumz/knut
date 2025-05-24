package handler

import "net/http"

// setContentType sets the "Content-Type" to "contentType", if not already
// set
func SetContentType(next http.Handler, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, exists := w.Header()["Content-Type"]; !exists {
			w.Header().Set("Content-Type", contentType)
		}
		next.ServeHTTP(w, r)
	})
}
