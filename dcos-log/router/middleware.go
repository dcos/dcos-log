package router

import "net/http"

func validateQueryStringMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for paramName, paramValues := range r.Form {
			for _, paramValue := range paramValues {
				// validate user input parameters length
				// since rfc http://www.faqs.org/rfcs/rfc2068.html does not specify the required
				// parameters length, let's let's make sure that neither key, nor value
				// bigger then 100 bytes.
				if len(paramName) > 100 || len(paramValue) > 100 {
					http.Error(w, "Incorrect query string", http.StatusBadRequest)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ... auth code here
		next.ServeHTTP(w, r)
	})
}
