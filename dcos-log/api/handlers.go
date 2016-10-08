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

func writeErrorResponse(w http.ResponseWriter, code int, msg string) {
	logrus.Error(msg)
	http.Error(w, msg, code)
}

func parseUINT64(s string) (bool, uint64, error) {
	var negative bool

	if s == "" {
		return negative, 0, errors.New("Input string cannot be empty")
	}

	if strings.HasPrefix(s, "-") {
		s = s[1:]
		negative = true
	}

	n, err := strconv.ParseUint(s, 10, 64)
	return negative, n, err
}

func getCursor(req *http.Request) (string, error) {
	cursor := req.URL.Query().Get("cursor")
	if cursor == "" {
		return cursor, nil
	}

	cursor, err := url.QueryUnescape(cursor)
	if err != nil {
		return cursor, fmt.Errorf("Unable to unescape cursor parameter: %s", err)
	}
	return cursor, nil
}

func getLimit(req *http.Request, stream bool) (uint64, error) {
	limitParam := req.URL.Query().Get("limit")
	if limitParam == "" {
		return 0, nil
	}

	negative, limit, err := parseUINT64(limitParam)
	if err != nil {
		return 0, fmt.Errorf("Error parsing paramter `limit`: %s", err)
	}

	if stream {
		return 0, errors.New("Unable to stream events with `limit` parameter")
	}

	if negative {
		return 0, errors.New("Number of entries cannot be negative value")
	}

	return limit, nil
}

func getSkip(req *http.Request) (uint64, uint64, error) {
	skipParam := req.URL.Query().Get("skip")
	if skipParam == "" {
		return 0, 0, nil
	}

	negative, limit, err := parseUINT64(skipParam)
	if err != nil {
		return 0, 0, fmt.Errorf("Error parsing parameter `skip`: %s", err)
	}

	if negative {
		return 0, limit, nil
	}

	return limit, 0, nil
}

func getMatches(req *http.Request) ([]reader.JournalEntryMatch, error) {
	var matches []reader.JournalEntryMatch
	for _, filter := range req.URL.Query()["filter"] {
		filterArray := strings.Split(filter, ":")
		if len(filterArray) != 2 {
			return matches, fmt.Errorf("Incorrect filter parameter format, must be ?filer=key:value. Got %s", filter)
		}

		matches = append(matches, reader.JournalEntryMatch{
			Field: filterArray[0],
			Value: filterArray[1],
		})
	}

	return matches, nil
}

func readJournalHandler(w http.ResponseWriter, req *http.Request, stream bool, entryFormatter reader.EntryFormatter) {
	ctx, cancel := context.WithCancel(context.Background())

	// Read `filter` parameters.
	matches, err := getMatches(req)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Read `cursor` parameter.
	cursor, err := getCursor(req)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Read `limit` parameter.
	limit, err := getLimit(req, stream)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// Read `skip` parameter.
	skipNext, skipPrev, err := getSkip(req)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	// create a journal reader instance with required options.
	j, err := reader.NewReader(entryFormatter,
		reader.OptionMatch(matches),
		reader.OptionSeekCursor(cursor),
		reader.OptionLimit(limit),
		reader.OptionSkipNext(skipNext),
		reader.OptionSkipPrev(skipPrev))
	if err != nil {
		e := fmt.Sprintf("Error opening journal reader: %s", err)
		writeErrorResponse(w, http.StatusInternalServerError, e)
		return
	}

	go func() {
		select {
		case <-ctx.Done():
			logrus.Debug("Requests fulfilled, closing journal")
			j.Journal.Close()
		}
	}()
	defer cancel()

	w.Header().Set("Content-Type", entryFormatter.GetContentType())
	if !stream {
		b, err := io.Copy(w, j)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if b == 0 {
			writeErrorResponse(w, http.StatusNoContent, "No match found")
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
				logrus.Debug("Closing a client connecton")
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
	readJournalHandler(w, req, true, reader.FormatSSE{})
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
	readJournalHandler(w, req, false, reader.FormatSSE{})
}

func rangeServerStarHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, false, reader.FormatText{})
}
