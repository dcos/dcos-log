package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/dcos/dcos-log/dcos-log/journal/reader"
)

// RangeHeader is a struct that describes a Range header.
type RangeHeader struct {
	Cursor                    string
	SkipNext, SkipPrev, Limit uint64
}

func (r *RangeHeader) validateCursor() error {
	// empty cursor allowed
	if r.Cursor == "" {
		return nil
	}

	// Cursor format https://github.com/systemd/systemd/blob/master/src/journal/sd-journal.c#L937
	cArray := strings.Split(r.Cursor, ";")
	if len(cArray) != 6 {
		return fmt.Errorf("Incorrect cursor format. Got %s", r.Cursor)
	}

	//TODO: add more checks
	return nil
}

func parseRangeHeader(h string) (r RangeHeader, err error) {
	r = RangeHeader{}

	// Cursor format https://github.com/systemd/systemd/blob/master/src/journal/sd-journal.c#L937
	// example entries=cursor[[:num_skip]:num_entries]
	hArray := strings.Split(h, ":")
	if len(hArray) != 3 {
		return r, fmt.Errorf("Unexpected header format. Got `%s`", h)
	}

	r.Cursor = strings.TrimPrefix(hArray[0], "entries=")
	if err := r.validateCursor(); err != nil {
		return r, err
	}

	skip, err := strconv.ParseInt(hArray[1], 10, 64)
	if err != nil {
		return r, err
	}

	if skip > 0 {
		r.SkipNext = uint64(skip)
	} else if skip < 0 {
		r.SkipPrev = uint64(-skip)
	}

	r.Limit, err = strconv.ParseUint(hArray[2], 10, 64)
	if err != nil {
		return r, err
	}

	return r, nil
}

func writeErrorResponse(w http.ResponseWriter, code int, msg string) {
	logrus.Error(msg)
	http.Error(w, msg, code)
}

func readJournalHandler(w http.ResponseWriter, req *http.Request, stream bool, contentType reader.ContentType) {
	var (
		rHeader RangeHeader
		err     error
	)
	ctx, cancel := context.WithCancel(context.Background())

	reqRange := req.Header.Get("Range")
	if reqRange != "" {
		rHeader, err = parseRangeHeader(reqRange)
		if err != nil {
			e := fmt.Sprintf("Error parsing header `Range`: %s", err)
			writeErrorResponse(w, http.StatusBadRequest, e)
			return
		}

		if stream && rHeader.Limit != 0 {
			writeErrorResponse(w, http.StatusBadRequest, "Unable to limit a number of log entries in following mode")
			return
		}
	}

	// Parse form parameters and apply matches
	var matches []reader.Match
	if err := req.ParseForm(); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Could not parse request form")
		return
	}

	for matchKey, matchValues := range req.Form {
		// keys starting with double underscores are ignored
		if strings.HasPrefix(matchKey, "__") {
			logrus.Debugf("Skipping key: %s", matchKey)
			continue
		}

		for _, matchValue := range matchValues {
			matches = append(matches, reader.Match{
				Field: matchKey,
				Value: matchValue,
			})
		}
	}

	// build a config for a specific request
	journalConfig := reader.JournalReaderConfig{
		Cursor:      rHeader.Cursor,
		ContentType: contentType,
		Limit:       rHeader.Limit,
		SkipNext:    rHeader.SkipNext,
		SkipPrev:    rHeader.SkipPrev,
		Matches:     matches,
	}

	j, err := reader.NewReader(journalConfig)
	if err != nil {
		e := fmt.Sprintf("Error opening journal reader: %s", err)
		writeErrorResponse(w, http.StatusInternalServerError, e)
		return
	}

	go func() {
		select {
		case <-ctx.Done():
			logrus.Info("Requests fulfilled, closing journal")
			j.Journal.Close()
		}
	}()
	defer cancel()

	w.Header().Set("Content-Type", string(contentType))
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
				logrus.Info("Closing a client connecton")
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
	readJournalHandler(w, req, true, reader.ContentTypeText)
}

func streamingServerJSONHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, true, reader.ContentTypeJSON)
}

func streamingServerSSEHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, true, reader.ContentTypeStream)
}

// Range handlers
func rangeServerSSEHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, false, reader.ContentTypeStream)
}

func rangeServerJSONHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, false, reader.ContentTypeJSON)
}

func rangeServerTextHandler(w http.ResponseWriter, req *http.Request) {
	readJournalHandler(w, req, false, reader.ContentTypeText)
}
