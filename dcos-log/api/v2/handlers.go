package v2

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-go/dcos"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/api/middleware"
	"github.com/dcos/dcos-log/dcos-log/mesos/files/reader"
	"github.com/gorilla/mux"
)

const (
	prefix = "/system/v1/agent"
)

// ERROR is a http.Error wrapper, that also emits an error to a console
func ERROR(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
	logrus.Errorf("%s; http code: %d", msg, code)
}

func readFilesAPI(w http.ResponseWriter, req *http.Request) {
	cfg, ok := middleware.FromContextConfig(req.Context())
	if !ok {
		ERROR(w, "invalid context, unable to retrieve a config object", http.StatusInternalServerError)
		return
	}

	client, ok := middleware.FromContextHTTPClient(req.Context())
	if !ok {
		ERROR(w, "invalid context, unable to retrieve an *http.Client object", http.StatusInternalServerError)
	}

	nodeInfo, ok := middleware.FromContextNodeInfo(req.Context())
	if !ok {
		ERROR(w, "invalid context, unable to retrieve a nodeInfo object", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if cfg.FlagAuth {
		scheme = "https"
	}

	vars := mux.Vars(req)
	frameworkID := vars["frameworkID"]
	executorID := vars["executorID"]
	containerID := vars["containerID"]
	taskPath := vars["taskPath"]
	file := vars["file"]

	token, ok := middleware.FromContextToken(req.Context())
	if !ok {
		ERROR(w, "unable to get authorization header from a request", http.StatusUnauthorized)
		return
	}

	header := http.Header{}
	header.Set("Authorization", token)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	mesosID, err := nodeInfo.MesosID(nodeutil.NewContextWithHeaders(ctx, header))
	if err != nil {
		ERROR(w, "unable to get mesosID: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ip, err := nodeInfo.DetectIP()
	if err != nil {
		ERROR(w, err.Error(), http.StatusInternalServerError)
		return
	}

	masterURL := url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(ip.String(), strconv.Itoa(dcos.PortMesosAgent)),
		Path:   "/files/read",
	}

	opts := []reader.Option{reader.OptHeaders(header), reader.OptStream(true)}
	lastEventID := req.Header.Get("Last-Event-ID")
	if lastEventID != "" {
		offset, err := strconv.Atoi(lastEventID)
		if err != nil {
			ERROR(w, fmt.Sprintf("invalid Last-Event-ID: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		opts = append(opts, reader.OptOffset(offset))
	}

	defaultFormatter := reader.LineFormat
	if req.Header.Get("Accept") == "text/event-stream" {
		defaultFormatter = reader.SSEFormat
	}

	r, err := reader.NewLineReader(client, masterURL, mesosID, frameworkID, executorID, containerID,
		taskPath, file, defaultFormatter, opts...)
	if err != nil {
		ERROR(w, err.Error(), http.StatusInternalServerError)
		logrus.Errorf("unable to initialize files API reader: %s", err)
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

func redirectURL(id *nodeutil.CanonicalTaskID, file string) (string, error) {
	// find if the task is standalone of a pod.
	isPod := id.ExecutorID != ""
	executorID := id.ExecutorID
	if !isPod {
		executorID = id.ID
	}

	// take the last element
	taskID := id.ContainerIDs[len(id.ContainerIDs)-1]
	taskLogURL := fmt.Sprintf("%s/%s/logs/v2/task/frameworks/%s/executors/%s/runs/%s/", prefix, id.AgentID,
		id.FrameworkID, executorID, taskID)

	if isPod {
		taskLogURL += fmt.Sprintf("/tasks/%s/%s", id.ID, file)
	} else {
		taskLogURL += file
	}

	return taskLogURL, nil
}

func discover(w http.ResponseWriter, req *http.Request) {
	nodeInfo, ok := middleware.FromContextNodeInfo(req.Context())
	if !ok {
		ERROR(w, "invalid context, unable to retrieve a nodeInfo object", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	taskID := vars["taskID"]
	file := vars["file"]

	if file == "" {
		file = "stdout"
	}

	if taskID == "" {
		logrus.Error("taskID is empty")
		ERROR(w, "taskID is empty", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// try to get the canonical ID for a running task first.
	var (
		canonicalTaskID *nodeutil.CanonicalTaskID
		err             error
	)

	// TODO: expose this option to a user.
	for _, completed := range []bool{false, true} {
		canonicalTaskID, err = nodeInfo.TaskCanonicalID(ctx, taskID, completed)
		if err == nil {
			break
		}
	}

	if err != nil {
		errMsg := fmt.Sprintf("unable to get canonical task ID: %s", err)
		ERROR(w, errMsg, http.StatusInternalServerError)
		return
	}

	taskURL, err := redirectURL(canonicalTaskID, file)
	if err != nil {
		errMsg := fmt.Sprintf("unable to build redirect URL: %s", err)
		ERROR(w, errMsg, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, req, taskURL, http.StatusSeeOther)
}
