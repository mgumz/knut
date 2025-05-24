package handler

import "net/http"

// serveStringHandler writes given 'str' to the response
func ServeStringHandler(str string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(str))
	})
}
