package router

import "net/http"

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ... auth code here
		next.ServeHTTP(w, r)
	})
}
