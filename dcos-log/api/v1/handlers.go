package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-log/dcos-log/journal/reader"
	"github.com/gorilla/mux"
)

// AllowedFields contain `Journald Container Logger module` fields except ExecutorInfo.
// https://github.com/dcos/dcos-mesos-modules/blob/master/journald/README.md#journald-container-logger-module
var AllowedFields = []string{"FRAMEWORK_ID", "AGENT_ID", "EXECUTOR_ID", "CONTAINER_ID", "STREAM"}

// Constants used as request valid GET parameters. All other parameter is ignored.
const (
	getParamLimit       getParam = "limit"
	getParamSkipNext    getParam = "skip_next"
	getParamSkipPrev    getParam = "skip_prev"
	getParamFilter      getParam = "filter"
	getParamCursor      getParam = "cursor"
	getParamReadReverse getParam = "read_reverse"
)

type getParam string

func (g getParam) String() string {
	return string(g)
}

func httpError(w http.ResponseWriter, msg string, code int, req *http.Request) {
	debugString := fmt.Sprintf("Message: %s [request URI: %s; remote address: %s; Accept: %s; Proto: %s]", msg,
		req.RequestURI, req.RemoteAddr, req.Header.Get("Accept"), req.Proto)
	logrus.Error(debugString)
	http.Error(w, debugString, code)
}

// Cursor string contains special characters we have to escape. This function returns un-escaped cursor.
func getCursor(req *http.Request) (string, error) {
	cursor := req.URL.Query().Get(getParamCursor.String())
	if cursor == "" {
		return cursor, nil
	}

	cursor, err := url.QueryUnescape(cursor)
	if err != nil {
		return cursor, fmt.Errorf("Unable to unescape cursor parameter: %s", err)
	}
	return cursor, nil
}

// GET paramter `limit` is a string which must contain positive uint64 value. This parameter cannot be used with
// stream-events option.
func getLimit(req *http.Request, stream bool) (uint64, error) {
	limitParam := req.URL.Query().Get(getParamLimit.String())
	if limitParam == "" {
		return 0, nil
	}

	limit, err := strconv.ParseUint(limitParam, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("Error parsing paramter `limit`: %s", err)
	}

	if stream && limit > 0 {
		return 0, errors.New("Unable to stream events with `limit` parameter")
	}

	return limit, nil
}

// try to parse `skip_next` and `skip_prev`
func getSkip(req *http.Request) (uint64, uint64, error) {
	var (
		skipNext, skipPrev uint64
		err                error
	)

	if skipParamNext := req.URL.Query().Get(getParamSkipNext.String()); skipParamNext != "" {
		skipNext, err = strconv.ParseUint(skipParamNext, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("Error parsing parameter %s: %s", getParamSkipNext, err)
		}
	}

	if skipParamPrev := req.URL.Query().Get(getParamSkipPrev.String()); skipParamPrev != "" {
		skipPrev, err = strconv.ParseUint(skipParamPrev, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("Error parsing parameter %s: %s", getParamSkipPrev, err)
		}
	}

	return skipNext, skipPrev, nil
}

// getMatches parses the GET parameter `filter` and returns []reader.JournalEntryMatch.
func getMatches(req *http.Request) ([]reader.JournalEntryMatch, error) {
	var matches []reader.JournalEntryMatch
	for _, filter := range req.URL.Query()[getParamFilter.String()] {
		filterArray := strings.Split(filter, ":")
		if len(filterArray) != 2 {
			return matches, fmt.Errorf("Incorrect filter parameter format, must be ?filer=key:value. Got %s", filter)
		}

		// all matches must uppercase
		matches = append(matches, reader.JournalEntryMatch{
			Field: strings.ToUpper(filterArray[0]),
			Value: filterArray[1],
		})
	}

	return matches, nil
}

func getReadReverse(req *http.Request, stream bool) (bool, error) {
	readReverse := req.URL.Query().Get(getParamReadReverse.String())
	if readReverse == "" {
		return false, nil
	}

	if stream {
		return false, fmt.Errorf("Unable to stream events with `read_reverse` parameter")
	}
	return strconv.ParseBool(readReverse)
}

func pathMatches(req *http.Request) []reader.JournalEntryMatch {
	var matches []reader.JournalEntryMatch

	// try to find container_id, framework_id and executor_id in request variables and apply
	// appropriate matches.
	for _, requestVar := range []struct{ fieldName, pathVar string }{
		{
			fieldName: "CONTAINER_ID",
			pathVar:   "container_id",
		},
		{
			fieldName: "FRAMEWORK_ID",
			pathVar:   "framework_id",
		},
		{
			fieldName: "EXECUTOR_ID",
			pathVar:   "executor_id",
		},
	} {
		value := mux.Vars(req)[requestVar.pathVar]
		if value != "" {
			matches = append(matches, reader.JournalEntryMatch{
				Field: requestVar.fieldName,
				Value: value,
			})
		}
	}
	return matches
}

// main handler.
func readJournalHandler(w http.ResponseWriter, req *http.Request) {
	stream := requestStreamKeyFromContext(req.Context())

	// for streaming endpoints and SSE logs format we include id: CursorID before each log entry.
	entryFormatter := reader.NewEntryFormatter(req.Header.Get("Accept"), stream)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// get a list of matches from request path
	matches := pathMatches(req)

	// Read `filter` parameters.
	requestMatches, err := getMatches(req)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
	}

	// Append matches from get params.
	if len(requestMatches) > 0 {
		matches = append(matches, requestMatches...)
	}

	// Read `cursor` parameter.
	cursor, err := getCursor(req)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
	}

	// Read `limit` parameter.
	limit, err := getLimit(req, stream)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
	}

	// Read `skip` parameter.
	skipNext, skipPrev, err := getSkip(req)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
	}

	// Read `read_reverse` parameter.
	readReverse, err := getReadReverse(req, stream)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
	}

	// Last-Event-ID is a value that contains a cursor. If the header is in the request, we should take
	// the value and override the cursor parameter. This will work for streaming endpoints only.
	// https://www.html5rocks.com/en/tutorials/eventsource/basics/#toc-lastevent-id
	if stream {
		lastEventID := req.Header.Get("Last-Event-ID")
		if lastEventID != "" {
			logrus.Debugf("Received `Last-Event-ID`: %s", lastEventID)
			cursor = lastEventID

			// if the browser sends `Last-Event-ID` we have to null skipPrev and skipNext counters
			// since we don't want to see duplicate log entries.
			skipPrev = 0
			skipNext = 0
		}
	}

	// create a journal reader instance with required options.
	j, err := reader.NewReader(entryFormatter,
		reader.OptionMatch(matches),
		reader.OptionSeekCursor(cursor),
		reader.OptionLimit(limit),
		reader.OptionSkipNext(skipNext),
		reader.OptionSkipPrev(skipPrev),
		reader.OptionReadReverse(readReverse))
	if err != nil {
		httpError(w, fmt.Sprintf("Error opening journal reader: %s", err), http.StatusInternalServerError, req)
		return
	}

	requestStartTime := time.Now()
	go func() {
		select {
		case <-ctx.Done():
			j.Journal.Close()
			logrus.Debugf("Request done in %s, URI: %s, remote addr: %s", time.Since(requestStartTime).String(),
				req.RequestURI, req.RemoteAddr)
		}
	}()

	w.Header().Set("Content-Type", entryFormatter.GetContentType().String())

	// X-Journal-Skip-Next indicates how many entries we actually skipped forward from the current position.
	// X-Journal-Skip-Prev indicates how many entries we actually skipped backwards from the current position.
	// This feature can be used to tell whether we reached journal's top and/or bottom.
	w.Header().Set("X-Journal-Skip-Next", strconv.FormatUint(j.SkippedNext, 10))
	w.Header().Set("X-Journal-Skip-Prev", strconv.FormatUint(j.SkippedPrev, 10))

	// Set response headers.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Transfer-Encoding", "chunked")

	if !stream {
		b, err := io.Copy(w, j)
		if err != nil {
			httpError(w, err.Error(), http.StatusInternalServerError, req)
			return
		}
		if b == 0 {
			httpError(w, "No match found", http.StatusNoContent, req)
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
				logrus.Debugf("Closing a client connection.Request URI: %s", req.RequestURI)
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

func fieldHandler(w http.ResponseWriter, req *http.Request) {
	field := mux.Vars(req)["field"]

	// validate that we are allowed to get values for requested field.
	err := func() error {
		for _, validField := range AllowedFields {
			if validField == field {
				return nil
			}
		}
		return fmt.Errorf("%s is not an allowed field", field)
	}()
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
	}

	j, err := reader.NewReader(nil)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
	}
	defer j.Journal.Close()

	values, err := j.Journal.GetUniqueValues(field)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
	}

	if len(values) == 0 {
		msg := fmt.Sprintf("Field %s not found", field)
		httpError(w, msg, http.StatusNoContent, req)
		return
	}

	v, err := json.Marshal(values)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError, req)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(v)
	if err != nil {
		logrus.Errorf("Error writing to client: %s", err)
	}
}
