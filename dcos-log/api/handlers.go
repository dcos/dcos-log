package api

import (
	"context"
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
)

type getParam string

func (g getParam) String() string {
	return string(g)
}

// Constants used as request valid GET parameters. All other parameter is ignored.
const (
	getParamLimit    getParam = "limit"
	getParamSkipNext getParam = "skip_next"
	getParamSkipPrev getParam = "skip_prev"
	getParamFilter   getParam = "filter"
	getParamCursor   getParam = "cursor"
)

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

// main handler.
func readJournalHandler(w http.ResponseWriter, req *http.Request, stream bool, entryFormatter reader.EntryFormatter) {
	ctx, cancel := context.WithCancel(context.Background())

	// Read `filter` parameters.
	matches, err := getMatches(req)
	if err != nil {
		httpError(w, err.Error(), http.StatusBadRequest, req)
		return
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

	// Last-Event-ID is a value that contains a cursor. If the header is in the request, we should take
	// the value and override the cursor parameter.
	// https://www.html5rocks.com/en/tutorials/eventsource/basics/#toc-lastevent-id
	lastEventID := req.Header.Get("Last-Event-ID")
	if lastEventID != "" {
		logrus.Debugf("Received `Last-Event-ID`: %s", lastEventID)
		cursor = lastEventID
	}

	// create a journal reader instance with required options.
	j, err := reader.NewReader(entryFormatter,
		reader.OptionMatch(matches),
		reader.OptionSeekCursor(cursor),
		reader.OptionLimit(limit),
		reader.OptionSkipNext(skipNext),
		reader.OptionSkipPrev(skipPrev))
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
	defer cancel()

	w.Header().Set("Content-Type", entryFormatter.GetContentType().String())

	// X-Journal-Skip-Next indicates how many entries we actually skipped forward from the current position.
	// X-Journal-Skip-Prev indicates how many entries we actually skipped backwards from the current position.
	// This feature can be used to tell whether we reached journal's top and/or bottom.
	w.Header().Set("X-Journal-Skip-Next", strconv.FormatUint(j.SkippedNext, 10))
	w.Header().Set("X-Journal-Skip-Prev", strconv.FormatUint(j.SkippedPrev, 10))
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
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	f := w.(http.Flusher)
	notify := w.(http.CloseNotifier).CloseNotify()

	f.Flush()
	for {
		select {
		case <-notify:
			{
				logrus.Debugf("Closing a client connecton.Request URI: %s", req.RequestURI)
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

// Streaming handlers
func streamingServerTextHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, true, reader.FormatText{})
}

func streamingServerJSONHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, true, reader.FormatJSON{})
}

func streamingServerSSEHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, true, reader.FormatSSE{UseCursorID: true})
}

func streamingServerStarHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, true, reader.FormatText{})
}

// Range handlers
func rangeServerTextHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, false, reader.FormatText{})
}

func rangeServerJSONHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, false, reader.FormatJSON{})
}

func rangeServerSSEHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, false, reader.FormatSSE{UseCursorID: true})
}

func rangeServerStarHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, false, reader.FormatText{})
}
