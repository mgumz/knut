package handler

import "net/http"

// addServerIDHandler adds "Server: <serverID>" to the response
// header
func AddServerIDHandler(next http.Handler, serverID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", serverID)
		next.ServeHTTP(w, r)
	})
}
