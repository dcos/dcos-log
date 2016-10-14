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
	getParamLimit  getParam = "limit"
	getParamSkip   getParam = "skip"
	getParamFilter getParam = "filter"
	getParamCursor getParam = "cursor"
)

func httpError(w http.ResponseWriter, msg string, code int, req *http.Request) {
	debugString := fmt.Sprintf("Message: %s [request URI: %s; remote address: %s; Accept: %s; Proto: %s]", msg,
		req.RequestURI, req.RemoteAddr, req.Header.Get("Accept"), req.Proto)
	logrus.Error(debugString)
	http.Error(w, debugString, code)
}

// parseUint64 takes a string and tried to convert it into uint64. API allows to use negative values which indicates
// we should move backwards from the current cursor position. If a string starts with `-`, we remove the negative
// sign and return value negative true.
func parseUint64(s string) (negative bool, n uint64, err error) {
	if s == "" {
		return negative, n, errors.New("Input string cannot be empty")
	}

	if strings.HasPrefix(s, "-") {
		s = s[1:]
		negative = true
	}

	n, err = strconv.ParseUint(s, 10, 64)
	return negative, n, err
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

	negative, limit, err := parseUint64(limitParam)
	if err != nil {
		return 0, fmt.Errorf("Error parsing paramter `limit`: %s", err)
	}

	if stream && limit > 0 {
		return 0, errors.New("Unable to stream events with `limit` parameter")
	}

	if negative {
		return 0, errors.New("Number of entries cannot be negative value")
	}

	return limit, nil
}

// GET parameter `skip` is a string, which may start with `-` followed by uint64. This function returns 3 values:
// 1. Number of log entries to skip from the current cursor position forward (uint64).
// 2. Number of log entries to skip from the current cursor position backward (uint64).
// 3. Error.
func getSkip(req *http.Request) (uint64, uint64, error) {
	skipParam := req.URL.Query().Get(getParamSkip.String())
	if skipParam == "" {
		return 0, 0, nil
	}

	negative, limit, err := parseUint64(skipParam)
	if err != nil {
		return 0, 0, fmt.Errorf("Error parsing parameter `skip`: %s", err)
	}

	if negative {
		return 0, limit, nil
	}

	return limit, 0, nil
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
