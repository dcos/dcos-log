package v2

import (
	"io"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-log/dcos-log/api/middleware"
)

func readFilesAPI(w http.ResponseWriter, req *http.Request) {
	//masterURL := &url.URL{
	//	Scheme: "http",
	//	Host:   "leader.mesos",
	//}

	//vars := mux.Vars(req)
	//frameworkID := vars["frameworkID"]
	//executorID := vars["executorID"]
	//containerID := vars["containerID"]
	//
	//token, err := middleware.GetAuthFromRequest(req)
	//if err != nil {
	//	http.Error(w, fmt.Sprintf("Token error: %s", err.Error()), http.StatusUnauthorized)
	//	return
	//}
	//logrus.Infof("got token: %s", token)
	//
	//header := http.Header{}
	//header.Set("Authorization", token)
	//
	//logrus.Infof("nodeInfo: %s", nodeInfo)
	//mesosID, err := nodeInfo.MesosID(nodeutil.NewContextWithHeaders(context.TODO(), header))
	//if err != nil {
	//	http.Error(w, "Unable to get mesosID: "+err.Error(), http.StatusInternalServerError)
	//	return
	//}

	//agentID := "c26a31b7-8f57-47af-9b4f-cc3e5bf2d7aa-S0"
	//frameworkID := "c26a31b7-8f57-47af-9b4f-cc3e5bf2d7aa-0001"
	//executorID := "chatter.69e42c0d-9266-11e7-ba2d-8aa5927b4a10"
	//containerID := "4e9e26bf-0ddd-47de-849a-8eaa9aeacea9"

	// h := http.Header{}
	//h.Add("Cookie", `ajs_group_id=null; dcos-acs-auth-cookie=eyJhbGciOiJIUzI1NiIsImtpZCI6InNlY3JldCIsInR5cCI6IkpXVCJ9.eyJhdWQiOiIzeUY1VE9TemRsSTQ1UTF4c3B4emVvR0JlOWZOeG05bSIsImVtYWlsIjoibW5hYm9rYUBtZXNvc3BoZXJlLmlvIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImV4cCI6MTUwNTA2MjcwMCwiaWF0IjoxNTA0NjMwNzAwLCJpc3MiOiJodHRwczovL2Rjb3MuYXV0aDAuY29tLyIsInN1YiI6Imdvb2dsZS1vYXV0aDJ8MTAyMDQyMTI2NDI5MDQ2MTY3MTY2IiwidWlkIjoibW5hYm9rYUBtZXNvc3BoZXJlLmlvIn0.F6UJxPZAUhuCLqGWVWrlrks3LOf3hU8P4a9IoZnQ_4k; dcos-acs-info-cookie=eyJ1aWQiOiJtbmFib2thQG1lc29zcGhlcmUuaW8iLCJkZXNjcmlwdGlvbiI6Im1uYWJva2FAbWVzb3NwaGVyZS5pbyJ9; ajs_anonymous_id=%225008a4f8-fb13-4bfa-98ff-3c39b62fc612%22; ajs_user_id=%22mnaboka%40mesosphere.io%22`)

	//opts := []reader.Option{reader.OptHeaders(h), reader.OptStream(true), reader.OptReadDirection(reader.TopToBottom)}
	//opts := []reader.Option{reader.OptHeaders(header), reader.OptStream(true),
	//	reader.OptReadDirection(reader.BottomToTop), reader.OptLines(10)}
	//lastEventID := req.Header.Get("Last-Event-ID")
	//if lastEventID != "" {
	//	offset, err := strconv.Atoi(lastEventID)
	//	if err != nil {
	//		http.Error(w, fmt.Sprintf("invalid Last-Event-ID: %s", err.Error()), http.StatusInternalServerError)
	//		return
	//	}
	//
	//	opts = append(opts, reader.OptOffset(offset))
	//}
	//
	//r, err := reader.NewLineReader(client, masterURL, mesosID, frameworkID, executorID, containerID, opts...)
	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//	logrus.Errorf("unable to initialize files API reader: %s", err)
	//	return
	//}

	r, err := middleware.FromContext(req.Context())
	if err != nil {
		http.Error(w, "invalid context, unable to initialize files API reader: " + err.Error(), http.StatusBadRequest)
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
		case <-time.After(time.Microsecond*100):
			{
				io.Copy(w, r)
				f.Flush()
			}
		}
	}
}
