package middleware

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"compress/gzip"
	"github.com/gorilla/mux"
)

// Gzip Compression
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// DownloadGzippedContentMiddleware is a middleware which sets Content-disposition header and makes a postfix
// for a downloadable item.
func DownloadGzippedContent(next http.Handler, prefix string, vars ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// log name lazy evaluation
		filenameParts := []string{prefix}
		muxVars := mux.Vars(r)
		for _, v := range vars {
			if muxVar := muxVars[v]; muxVar != "" {
				filenameParts = append(filenameParts, muxVar)
			}
		}

		// get user provided postfix
		if err := r.ParseForm(); err == nil {
			if postfix := r.Form.Get("postfix"); postfix != "" {
				filenameParts = append(filenameParts, postfix)
			}
		}

		filename := strings.Join(filenameParts, "-")
		if filename == "" {
			filename = "download"
		}

		f := fmt.Sprintf("%s.log.gz", filename)
		w.Header().Add("Content-disposition", "attachment; filename="+f)
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		next.ServeHTTP(gzw, r)
	})
}
