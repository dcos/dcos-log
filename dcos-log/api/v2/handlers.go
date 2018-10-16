package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dcos/dcos-go/dcos"
	"github.com/dcos/dcos-go/dcos/nodeutil"
	"github.com/dcos/dcos-log/dcos-log/api/middleware"
	jr "github.com/dcos/dcos-log/dcos-log/journal/reader"
	"github.com/dcos/dcos-log/dcos-log/mesos/files/reader"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	prefix = "/system/v1/agent"
)

const (
	skipParam   = "skip"
	cursorParam = "cursor"
	limitParam  = "limit"
	filterParam = "filter"

	cursorEndParam = "END"
	cursorBegParam = "BEG"
)

const (
	eventStreamContentType = "text/event-stream"
)

type errSetupFilesAPIReader struct {
	msg  string
	code int
}

func (e errSetupFilesAPIReader) Error() string {
	return e.msg
}

func logError(w http.ResponseWriter, req *http.Request, msg string, code int) {
	http.Error(w, msg, code)
	logrus.Errorf("%s; http code: %d, request %s", msg, code, req.URL)
}

func setupFilesAPIReader(req *http.Request, urlPath string, opts ...reader.Option) (r *reader.ReadManager, err error) {

	cfg, ok := middleware.FromContextConfig(req.Context())
	if !ok {
		return nil, errSetupFilesAPIReader{
			msg:  fmt.Sprintf("invalid context, unable to retrieve %T object", cfg),
			code: http.StatusInternalServerError,
		}
	}

	client, ok := middleware.FromContextHTTPClient(req.Context())
	if !ok {
		return nil, errSetupFilesAPIReader{
			msg:  fmt.Sprintf("invalid context, unable to retrieve %T object", client),
			code: http.StatusInternalServerError,
		}
	}

	nodeInfo, ok := middleware.FromContextNodeInfo(req.Context())
	if !ok {
		return nil, errSetupFilesAPIReader{
			msg:  fmt.Sprintf("invalid context, unable to retrieve a %T object", nodeInfo),
			code: http.StatusInternalServerError,
		}
	}

	scheme := "http"
	if cfg.FlagAuth {
		scheme = "https"
	}

	token, ok := middleware.FromContextToken(req.Context())
	if !ok {
		return nil, errSetupFilesAPIReader{
			msg:  "unable to get authorization header from a request",
			code: http.StatusUnauthorized,
		}
	}

	vars := mux.Vars(req)
	frameworkID := vars["frameworkID"]
	executorID := vars["executorID"]
	containerID := vars["containerID"]
	taskPath := vars["taskPath"]
	file := vars["file"]

	header := http.Header{}
	header.Set("Authorization", token)

	newOpts := []reader.Option{reader.OptHeaders(header)}
	newOpts = append(newOpts, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	mesosID, err := nodeInfo.MesosID(nodeutil.NewContextWithHeaders(ctx, header))
	if err != nil {
		return nil, errSetupFilesAPIReader{
			msg:  "unable to get mesosID: " + err.Error(),
			code: http.StatusInternalServerError,
		}
	}

	ip, err := nodeInfo.DetectIP()
	if err != nil {
		return nil, errSetupFilesAPIReader{
			msg:  "unable to run detect_ip: " + err.Error(),
			code: http.StatusInternalServerError,
		}
	}

	masterURL := &url.URL{
		Host:   net.JoinHostPort(ip.String(), strconv.Itoa(dcos.PortMesosAgent)),
		Scheme: scheme,
		Path:   urlPath,
	}

	formatter := reader.LineFormat
	if req.Header.Get("Accept") == eventStreamContentType {
		formatter = reader.SSEFormat
	}

	return reader.NewLineReader(client, *masterURL, mesosID, frameworkID, executorID, containerID, taskPath, file, formatter,
		newOpts...)
}

func optLimit(limitStr string) ([]reader.Option, error) {
	// return early on empty parameter
	if limitStr == "" {
		return nil, nil
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse limit parameter. %s not an integer", limitStr)
	}

	return []reader.Option{reader.OptLines(limit)}, nil
}

func optCursor(cursorStr string) ([]reader.Option, error) {
	// return early on empty parameter
	if cursorStr == "" {
		return nil, nil
	}

	switch cursorStr {
	case cursorBegParam:
		return []reader.Option{reader.OptOffset(0)}, nil
	case cursorEndParam:
		return []reader.Option{reader.OptReadFromEnd()}, nil
	default:
	}

	cursor, err := strconv.Atoi(cursorStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cursor parameter. %s not an integer", cursorStr)
	}

	return []reader.Option{reader.OptOffset(cursor)}, nil
}

func optSkip(skipStr string) (opts []reader.Option, err error) {
	// return early on empty parameter
	if skipStr == "" {
		return nil, nil
	}

	skip, err := strconv.Atoi(skipStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse skip parameter. %s not an integer", skipStr)
	}

	if skip != 0 {
		opts = append(opts, reader.OptSkip(skip))
	}

	if skip < 0 {
		opts = append(opts, reader.OptReadDirection(reader.BottomToTop))
	}

	return
}

func lastEventIDHeader(lastEventID string) (reader.Option, bool, error) {
	// return early on empty parameter
	if lastEventID == "" {
		return nil, false, nil
	}

	offset, err := strconv.Atoi(lastEventID)
	if err != nil {
		return nil, false, fmt.Errorf("unable to parse Last-Event-ID header. %s not an integer", lastEventID)
	}

	return reader.OptOffset(offset), true, nil
}

func buildOpts(req *http.Request) ([]reader.Option, error) {
	opt, ok, err := lastEventIDHeader(req.Header.Get("Last-Event-ID"))
	if err != nil {
		return nil, err
	}

	// if the Last-Evet-ID header is present we should ignore the rest of initial parameters
	// because it indicates the client has reconnected. The Last-Event-ID must have a higher
	// precedence.
	if ok {
		return []reader.Option{opt}, nil
	}

	var collectedOpts []reader.Option
	for _, paramFn := range []struct {
		fn    func(string) ([]reader.Option, error)
		param string
	}{
		{fn: optCursor, param: req.URL.Query().Get(cursorParam)},
		{fn: optSkip, param: req.URL.Query().Get(skipParam)},
		{fn: optLimit, param: req.URL.Query().Get(limitParam)},
	} {
		opts, err := paramFn.fn(paramFn.param)
		if err != nil {
			return nil, err
		}

		collectedOpts = append(collectedOpts, opts...)
	}

	return collectedOpts, nil
}

func filesAPIHandler(w http.ResponseWriter, req *http.Request) {
	opts, err := buildOpts(req)
	if err != nil {
		logError(w, req, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Header.Get("Accept") == eventStreamContentType {
		opts = append(opts, reader.OptStream(true))
	}

	r, err := setupFilesAPIReader(req, "/files/read", opts...)
	switch err {
	case nil:
		break
	case reader.ErrFileNotFound:
		logError(w, req, "File not found", http.StatusNoContent)
		return
	default:
		e, ok := err.(errSetupFilesAPIReader)
		if !ok {
			logError(w, req, "unable to initialize files API reader: "+err.Error(), http.StatusInternalServerError)
			return
		}

		logError(w, req, e.msg, e.code)
		return
	}

	if req.Header.Get("Accept") != eventStreamContentType {
		for {
			_, err := io.Copy(w, r)
			switch err {
			case nil:
				return
			case reader.ErrNoData:
				continue
			case reader.ErrFileNotFound:
				logError(w, req, "File not found", http.StatusNotFound)
				return
			default:
				logError(w, req, fmt.Sprintf("unexpected error while reading the logs: %s. Request: %s", err, req.RequestURI), http.StatusInternalServerError)
				return
			}
		}
	}

	w.Header().Set("Content-Type", eventStreamContentType)

	// Set response headers.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	w.Header().Set("X-Accel-Buffering", "no")
	f, ok := w.(http.Flusher)
	if !ok {
		logError(w, req, "unable to type assert ResponseWriter to Flusher", http.StatusInternalServerError)
		return
	}
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
				// TODO(rgoegge): This is a temporary fix.
				// Not ideal, but will feel responsive enough to the enduser for now.
				// The right fix should be a blocking io.Copy() call until there is data to read.
				bytes, err := io.Copy(w, r)
				if bytes == 0 {
					time.Sleep(time.Second)
				}
				if err != nil && err != reader.ErrNoData {
					logrus.Errorf("error while reading the files API reader: %s. Request: %s", err, req.RequestURI)
				}
				f.Flush()
			}
		}
	}
}

func redirectURL(id *nodeutil.CanonicalTaskID, file, RawQuery string, browse, download bool) (string, error) {
	if browse && download {
		return "", errors.New("browse and download are mutually excluded and cannot be used at the same time")
	}

	// find if the task is standalone of a pod.
	isPod := id.ExecutorID != ""
	executorID := id.ExecutorID
	if !isPod {
		executorID = id.ID
	}

	// take the last element
	taskID := id.ContainerIDs[len(id.ContainerIDs)-1]
	taskLogURL := fmt.Sprintf("%s/%s/logs/v2/task/frameworks/%s/executors/%s/runs/%s", prefix, id.AgentID,
		id.FrameworkID, executorID, taskID)

	if isPod {
		taskLogURL += path.Join("/tasks", id.ID)
	}

	if browse {
		taskLogURL = path.Join(taskLogURL, "/files/browse")
	} else {
		taskLogURL = path.Join(taskLogURL, file)

		if download {
			taskLogURL = path.Join(taskLogURL, "/download")
		}
	}

	if RawQuery != "" {
		taskLogURL += "?" + RawQuery
	}

	return taskLogURL, nil
}

func discoverHandler(w http.ResponseWriter, req *http.Request) {
	discover(w, req, false, false)
}

func browseHandler(w http.ResponseWriter, req *http.Request) {
	discover(w, req, true, false)
}

func downloadHandler(w http.ResponseWriter, req *http.Request) {
	discover(w, req, false, true)
}

func discover(w http.ResponseWriter, req *http.Request, browse, download bool) {
	nodeInfo, ok := middleware.FromContextNodeInfo(req.Context())
	if !ok {
		logError(w, req, "invalid context, unable to retrieve a nodeInfo object", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	taskID := vars["taskID"]
	file := vars["file"]

	if file == "" {
		file = "stdout"
	}

	if taskID == "" {
		logError(w, req, "taskID is empty", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// try to get the canonical ID for a running task first.
	var (
		canonicalTaskID *nodeutil.CanonicalTaskID
		err             error
	)

	// add headers to context
	token, ok := middleware.FromContextToken(req.Context())
	if !ok {
		logError(w, req, "unable to get authorization header from a request", http.StatusUnauthorized)
		return
	}

	header := http.Header{}
	header.Set("Authorization", token)
	ctx = nodeutil.NewContextWithHeaders(ctx, header)

	// TODO: expose this option to a user.
	for _, completed := range []bool{false, true} {
		canonicalTaskID, err = nodeInfo.TaskCanonicalID(ctx, taskID, completed)
		if err == nil {
			break
		}
	}

	if err != nil {
		errMsg := fmt.Sprintf("unable to get canonical task ID: %s", err)
		logError(w, req, errMsg, http.StatusInternalServerError)
		return
	}

	taskURL, err := redirectURL(canonicalTaskID, file, req.URL.RawQuery, browse, download)
	if err != nil {
		errMsg := fmt.Sprintf("unable to build redirect URL: %s", err)
		logError(w, req, errMsg, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, req, taskURL, http.StatusSeeOther)
}

func journalHandler(w http.ResponseWriter, req *http.Request) {
	acceptHeader := req.Header.Get("Accept")
	useSSE := acceptHeader == eventStreamContentType

	// for streaming endpoints and SSE logs format we include id: CursorID before each log entry.
	entryFormatter := jr.NewEntryFormatter(acceptHeader, useSSE)
	var (
		cursor string
		err    error
		opts   []jr.Option
	)

	if componentName := mux.Vars(req)["name"]; componentName != "" {
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
	if filters := req.URL.Query()[filterParam]; len(filters) > 0 {
		var matches []jr.JournalEntryMatch
		for _, filter := range filters {
			filterArray := strings.Split(filter, ":")
			if len(filterArray) != 2 {
				logError(w, req, "incorrect filter parameter format, must be ?filer=key:value. Got "+filter, http.StatusBadRequest)
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
	lastEventID := req.Header.Get("Last-Event-ID")
	if lastEventID != "" {
		cursor = lastEventID
	} else {
		// get cursor parameter
		cursor = req.URL.Query().Get(cursorParam)

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
				logError(w, req, "unable to un-escape cursor parameter: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	if cursor != "" {
		opts = append(opts, jr.OptionSeekCursor(cursor))
	}

	// parse the limit parameter
	if limitStr := req.URL.Query().Get(limitParam); limitStr != "" {
		limit, err := strconv.ParseUint(limitStr, 10, 64)
		if err != nil {
			logError(w, req, "unable to parse limit parameter: "+err.Error(), http.StatusBadRequest)
			return
		}

		opts = append(opts, jr.OptionLimit(limit))
	}

	// parse skip parameter
	if skipStr := req.URL.Query().Get(skipParam); skipStr != "" {
		skip, err := strconv.Atoi(skipStr)
		if err != nil {
			logError(w, req, "unable to parse skip parameter: "+err.Error(), http.StatusBadRequest)
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
		logError(w, req, "unable to open journald: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		err := j.Close()
		if err != nil {
			logrus.Errorf("error closing journald: %s", err)
		}
	}()

	// Set response headers.
	w.Header().Set("Content-Type", entryFormatter.GetContentType().String())
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	if !useSSE {
		b, err := io.Copy(w, j)
		if err != nil {
			logError(w, req, "unable to read the journal: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if b == 0 {
			logError(w, req, "No match found", http.StatusNoContent)
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
		case <- time.After(time.Second):
			err := j.Follow(time.Millisecond * 100, w)
			if err != nil {
				logrus.Errorf("error reading journal %s", err)
				return
			}
			f.Flush()
		}
	}

}

func browseFiles(w http.ResponseWriter, req *http.Request) {
	token, ok := middleware.FromContextToken(req.Context())
	if !ok {
		logError(w, req, "unable to get authorization header from a request", http.StatusUnauthorized)
		return
	}

	header := http.Header{}
	header.Set("Authorization", token)

	opts := []reader.Option{reader.OptHeaders(header)}

	r, err := setupFilesAPIReader(req, "/files/browse", opts...)
	if err != nil {
		e, ok := err.(errSetupFilesAPIReader)
		if !ok {
			logError(w, req, err.Error(), http.StatusInternalServerError)
			return
		}

		logError(w, req, e.msg, e.code)
		return
	}

	files, err := r.BrowseSandbox()
	if err != nil {
		logError(w, req, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(files); err != nil {
		logError(w, req, fmt.Sprintf("unable to encode sandbox files: %s. Items: %s", err, files), http.StatusInternalServerError)
		return
	}
}

func downloadFile(w http.ResponseWriter, req *http.Request) {
	token, ok := middleware.FromContextToken(req.Context())
	if !ok {
		logError(w, req, "unable to get authorization header from a request", http.StatusUnauthorized)
		return
	}

	header := http.Header{}
	header.Set("Authorization", token)

	opts := []reader.Option{reader.OptHeaders(header)}

	r, err := setupFilesAPIReader(req, "/files/download", opts...)
	if err != nil {
		e, ok := err.(errSetupFilesAPIReader)
		if !ok {
			logError(w, req, err.Error(), http.StatusInternalServerError)
			return
		}

		logError(w, req, e.msg, e.code)
		return
	}

	downloadResp, err := r.Download()
	if err != nil {
		logError(w, req, err.Error(), http.StatusInternalServerError)
		return
	}
	defer downloadResp.Body.Close()

	for k, vs := range downloadResp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	_, err = io.Copy(w, downloadResp.Body)
	if err != nil {
		logrus.Errorf("error raised while reading the download endpoint: %s", err)
	}
}
