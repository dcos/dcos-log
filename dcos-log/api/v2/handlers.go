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
	jr "github.com/dcos/dcos-log/dcos-log/journal/reader"
	"strings"
)

const (
	prefix = "/system/v1/agent"
)

const (
	skipParam = "skip"
	cursorParam = "cursor"
	limitParam = "limit"
	filterParam = "filter"

	cursorEndParam = "END"
	cursorBegParam = "BEG"
)

// ERROR is a http.Error wrapper, that also emits an error to a console
func ERROR(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
	logrus.Errorf("%s; http code: %d", msg, code)
}

func filesAPIHandler(w http.ResponseWriter, req *http.Request) {
	cfg, ok := middleware.FromContextConfig(req.Context())
	if !ok {
		ERROR(w, "invalid context, unable to retrieve a config object", http.StatusInternalServerError)
		return
	}

	client, ok := middleware.FromContextHTTPClient(req.Context())
	if !ok {
		ERROR(w, "invalid context, unable to retrieve an *http.Client object", http.StatusInternalServerError)
		return
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

	opts := []reader.Option{reader.OptHeaders(header)}

	var limit int
	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			ERROR(w, "unable to parse integer "+limitStr, http.StatusBadRequest)
			return
		}
	}

	if limit > 0 {
		opts = append(opts, reader.OptLines(limit))
	}

	cursor := -1
	if cursorStr := req.URL.Query().Get("cursor"); cursorStr != "" {
		if cursorStr == "BEG" {
			cursor = 0
		} else if cursorStr == "END" {
			opts = append(opts, reader.OptReadFromEnd())
		} else {
			cursor, err = strconv.Atoi(cursorStr)
			if err != nil {
				ERROR(w, "unable to parse integer "+cursorStr, http.StatusBadRequest)
				return
			}

			if cursor >= 0 {
				opts = append(opts, reader.OptOffset(cursor))
			}
		}
	}

	var skip int
	if skipStr := req.URL.Query().Get("skip"); skipStr != "" {
		skip, err = strconv.Atoi(skipStr)
		if err != nil {
			ERROR(w, "unable to parse integer "+skipStr, http.StatusBadRequest)
			return
		}

		if skip != 0 {
			opts = append(opts, reader.OptSkip(skip))
		}

		if skip < 0 {
			opts = append(opts, reader.OptReadDirection(reader.BottomToTop))
		}
	}

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
	stream := false
	if req.Header.Get("Accept") == "text/event-stream" {
		defaultFormatter = reader.SSEFormat
		stream = true
	}

	opts = append(opts, reader.OptStream(stream))

	r, err := reader.NewLineReader(client, masterURL, mesosID, frameworkID, executorID, containerID,
		taskPath, file, defaultFormatter, opts...)
	if err != nil {
		ERROR(w, err.Error(), http.StatusInternalServerError)
		logrus.Errorf("unable to initialize files API reader: %s", err)
		return
	}

	if req.Header.Get("Accept") != "text/event-stream" {
		for {
			_, err := io.Copy(w, r)
			switch err {
			case nil:
				return
			case reader.ErrNoData:
				continue
			default:
				logrus.Errorf("unexpected error while reading the logs: %s", err)
				return
			}
		}
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

func redirectURL(id *nodeutil.CanonicalTaskID, file, RawQuery string) (string, error) {
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

	if RawQuery != "" {
		taskLogURL += "?" + RawQuery
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

	taskURL, err := redirectURL(canonicalTaskID, file, req.URL.RawQuery)
	if err != nil {
		errMsg := fmt.Sprintf("unable to build redirect URL: %s", err)
		ERROR(w, errMsg, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, req, taskURL, http.StatusSeeOther)
}

func journalHandler(w http.ResponseWriter, r *http.Request) {
	acceptHeader := r.Header.Get("Accept")
	useSSE := acceptHeader == "text/event-stream"

	// for streaming endpoints and SSE logs format we include id: CursorID before each log entry.
	entryFormatter := jr.NewEntryFormatter(acceptHeader, useSSE)
	var (
		cursor string
		err error
		opts []jr.Option
	)

	if componentName := mux.Vars(r)["name"]; componentName != "" {
		matches := []jr.JournalEntryMatch{
			{
				Field: "UNIT",
				Value: componentName,
			},
			{
				Field: "_SYSTEMD_UNIT",
				Value: componentName,
			},
		}

		opts = append(opts, jr.OptionMatchOR(matches))
	}

	// parse filters
	if filters := r.URL.Query()[filterParam]; len(filters) > 0 {
		var matches []jr.JournalEntryMatch
		for _, filter := range filters {
			filterArray := strings.Split(filter, ":")
			if len(filterArray) != 2 {
				ERROR(w, "incorrect filter parameter format, must be ?filer=key:value. Got "+filter, http.StatusBadRequest)
				return
			}

			// all matches must uppercase
			matches = append(matches, jr.JournalEntryMatch{
				Field: strings.ToUpper(filterArray[0]),
				Value: filterArray[1],
			})
		}

		opts = append(opts, jr.OptionMatch(matches))
	}

	// we give priority to "Last-Event-ID" header over GET parameter.
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID != "" {
		cursor = lastEventID
	} else {
		// get cursor parameter
		cursor = r.URL.Query().Get(cursorParam)

		// according to V2 API, BEG and END are valid cursors. And they are used in mesos files API reader.
		// However journald API already implements the cursor movement with OptSkipPrev()
		// ignore BEG and END options for now.
		if cursor == cursorBegParam {
			cursor = ""
		} else if cursor == cursorEndParam {
			opts = append(opts, jr.OptionSkipPrev(1))
			cursor = ""
		}

		// parse the cursor parameter
		if cursor != "" {
			cursor, err = url.QueryUnescape(cursor)
			if err != nil {
				ERROR(w, "unable to un-escape cursor parameter: " + err.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	if cursor != "" {
		opts = append(opts, jr.OptionSeekCursor(cursor))
	}

	// parse the limit parameter
	if limitStr := r.URL.Query().Get(limitParam); limitStr != "" {
		limit, err := strconv.ParseUint(limitStr, 10, 64)
		if err != nil {
			ERROR(w, "unable to parse limit parameter: "+err.Error(), http.StatusBadRequest)
			return
		}

		opts = append(opts, jr.OptionLimit(limit))
	}

	// parse skip parameter
	if skipStr := r.URL.Query().Get(skipParam); skipStr != "" {
		skip, err := strconv.Atoi(skipStr)
		if err != nil {
			ERROR(w, "unable to parse skip parameter: "+err.Error(), http.StatusBadRequest)
			return
		}

		if skip > 0 {
			opts = append(opts, jr.OptionSkipNext(uint64(skip)))
		} else {
			// make skip positive number
			skip *= -1
			opts = append(opts, jr.OptionSkipPrev(uint64(skip)))
		}
	}

	j, err := jr.NewReader(entryFormatter, opts...)
	if err != nil {
		ERROR(w, "unable to open journald: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers.
	w.Header().Set("Content-Type", entryFormatter.GetContentType().String())
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	if !useSSE{
		b, err := io.Copy(w, j)
		if err != nil {
			ERROR(w, "unable to read the journal: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if b == 0 {
			ERROR(w, "No match found", http.StatusNoContent)
		}
		return
	}

	w.Header().Set("X-Accel-Buffering", "no")
	f := w.(http.Flusher)
	notify := w.(http.CloseNotifier).CloseNotify()

	f.Flush()
	for {
		select {
		case <-notify:
			{
				logrus.Debugf("closing a client connection.")
				return
			}
		case <-time.After(time.Second):
			{
				io.Copy(w, j)
				f.Flush()
			}
		}
	}

}