package v2

import (
	"io"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-log/dcos-log/api/middleware"
)

func readFilesAPI(w http.ResponseWriter, req *http.Request) {
	r, err := middleware.FromContext(req.Context())
	if err != nil {
		http.Error(w, "invalid context, unable to initialize files API reader: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Header.Get("Accept") != "text/event-stream" {
		io.Copy(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")

	// Set response headers.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	w.Header().Set("X-Accel-Buffering", "no")
	f := w.(http.Flusher)
	notify := w.(http.CloseNotifier).CloseNotify()

	f.Flush()
	for {
		select {
		case <-notify:
			{
				logrus.Debugf("Closing a client connection. Request URI: %s", req.RequestURI)
				return
			}
		case <-time.After(time.Microsecond * 100):
			{
				io.Copy(w, r)
				f.Flush()
			}
		}
	}
}
