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

func readJournalHandler(w http.ResponseWriter, req *http.Request, stream bool, entryFormatter reader.EntryFormatter) {
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
	var matches []reader.JournalEntryMatch
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
			matches = append(matches, reader.JournalEntryMatch{
				Field: matchKey,
				Value: matchValue,
			})
		}
	}

	// create a journal reader instance with required options.
	j, err := reader.NewReader(entryFormatter,
		reader.OptionSeekCursor(rHeader.Cursor),
		reader.OptionLimit(rHeader.Limit),
		reader.OptionSkipNext(rHeader.SkipNext),
		reader.OptionSkipPrev(rHeader.SkipPrev),
		reader.OptionMatch(matches))
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
