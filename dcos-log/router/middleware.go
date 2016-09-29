package router

import (
	"encoding/base64"
	"net/http"
)

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
				// bigger then 255 bytes.
				if len(paramName) > 255 || len(paramValue) > 255 {
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

// rangeGETParamToReqestHeaderMiddleware will try to decode `__range` GET parameter and set update
// `Range` header. We need this since javascript EventSource cannot send custom headers.
func rangeGETParamToReqestHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		encodedRangeParam := r.FormValue("__range")
		if encodedRangeParam != "" {
			// make sure we don't pass a Range header and __range GET param.
			reqRange := r.Header.Get("Range")
			if reqRange != "" {
				http.Error(w, "Cannot use GET parameter `__header ` and `Range` header together",
					http.StatusBadRequest)
				return
			}

			data, err := base64.StdEncoding.DecodeString(encodedRangeParam)
			if err != nil {
				http.Error(w, "Could not decode range header. Must be encoded with base64",
					http.StatusBadRequest)
				return
			}

			r.Header.Add("Range", string(data))
		}

		next.ServeHTTP(w, r)
	})
}
