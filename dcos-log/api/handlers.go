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
	"github.com/dcos/dcos-go/dcos-log/journal/reader"
)

type rangeHeader struct {
	Cursor string
	Skip   int64
	Num    uint64
}

func (r *rangeHeader) validateCursor() error {
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

func parseRangeHeader(h string) (r rangeHeader, err error) {
	r = rangeHeader{}

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

	r.Skip, err = strconv.ParseInt(hArray[1], 10, 64)
	if err != nil {
		return r, err
	}

	r.Num, err = strconv.ParseUint(hArray[2], 10, 64)
	if err != nil {
		return r, err
	}

	return r, nil
}

func writeErrorResponse(w http.ResponseWriter, code int, msg string) {
	logrus.Error(msg)
	http.Error(w, msg, code)
}

func advanceCursor(rHeader rangeHeader, j *reader.Reader) error {
	// find the cursor position to start with considering how many entries to skip
	// negative value allowed
	if rHeader.Skip > 0 {
		if _, err := j.Journal.NextSkip(uint64(rHeader.Skip)); err != nil {
			return fmt.Errorf("Unable to advance cursor with NextSkip: %s", err)
		}
	} else if rHeader.Skip < 0 {
		// if no cursor passed, move cursor to the very end
		if rHeader.Cursor == "" {
			if err := j.Journal.SeekTail(); err != nil {
				return fmt.Errorf("Unable to advance cursor to a tail: %s", err)
			}
		}
		if _, err := j.Journal.PreviousSkip(uint64(-rHeader.Skip)); err != nil {
			return fmt.Errorf("Unable to advanec cursor with PreviousSkip: %s", err)
		}
	}
	return nil
}

func readJournalHandler(w http.ResponseWriter, req *http.Request, stream bool, contentType reader.ContentType) {
	var (
		rHeader rangeHeader
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

		if stream && rHeader.Num != 0 {
			writeErrorResponse(w, http.StatusBadRequest, "Cannot use num with stream")
			return
		}
	}

	// open journal reader
	var limit *reader.Num
	if rHeader.Num > 0 {
		limit = &reader.Num{
			Value: rHeader.Num,
		}
	}

	j, err := reader.NewReader(limit, contentType)
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

	// Parse form parameters
	if err := req.ParseForm(); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Could not parse request form")
		return
	}

	// Apply filters
	for matchKey, matchValues := range req.Form {
		for _, matchValue := range matchValues {
			match := fmt.Sprintf("%s=%s", matchKey, matchValue)
			logrus.Infof("Adding match: %s", match)
			if err := j.Journal.AddMatch(match); err != nil {
				e := fmt.Sprintf("Could not add match %s %s: %s", matchKey, matchValues, err)
				writeErrorResponse(w, http.StatusInternalServerError, e)
				return
			}
			if err := j.Journal.AddConjunction(); err != nil {
				e := fmt.Sprintf("Could not add conjunction %s %s: %s", matchKey, matchValues, err)
				writeErrorResponse(w, http.StatusInternalServerError, e)
				return
			}
		}
	}

	if rHeader.Cursor != "" {
		if err := j.Journal.SeekCursor(rHeader.Cursor); err != nil {
			e := fmt.Sprintf("Error seeking cursor %s: %s", rHeader.Cursor, err)
			writeErrorResponse(w, http.StatusInternalServerError, e)
			return
		}

		if _, err := j.Journal.Next(); err != nil {
			e := fmt.Sprintf("Could not advance the cursor: %s", err)
			writeErrorResponse(w, http.StatusInternalServerError, e)
			return
		}

		// Verify if we found a cursor
		if err = j.Journal.TestCursor(rHeader.Cursor); err != nil {
			e := fmt.Sprintf("Error seeking cursor: %s", err)
			writeErrorResponse(w, http.StatusInternalServerError, e)
			return
		}

		if err := advanceCursor(rHeader, j); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
	} else {
		if err := advanceCursor(rHeader, j); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		}
	}

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
