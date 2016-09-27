package reader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
)

// ContentType defines the format journal.reader will use in response.
type ContentType string

const (
	// ContentTypeText is used if the client requests journal entries in text/plain or text/html.
	ContentTypeText ContentType = "text/plain"

	// ContentTypeJSON is used if the client requests journal entries in JSON.
	ContentTypeJSON ContentType = "application/json"

	// ContentTypeStream is used if the client requests journal entries in text/event-stream
	ContentTypeStream ContentType = "text/event-stream"
)

// Num defines a limit of journal entries to read.
type Num struct {
	Value uint64
}

// NewReader returns a new instance of journal reader.
func NewReader(limit *Num, contentType ContentType) (journalReader *Reader, err error) {
	journalReader = &Reader{
		Limit: limit,
	}
	journalReader.Journal, err = sdjournal.NewJournal()
	if err != nil {
		return journalReader, err
	}
	switch contentType {
	case ContentTypeText:
		journalReader.getDataFn = journalReader.GetTextEntry
	case ContentTypeJSON:
		journalReader.getDataFn = journalReader.GetJSONEntry
	case ContentTypeStream:
		journalReader.getDataFn = journalReader.GetSSEEntry
	default:
		return journalReader, fmt.Errorf("Incorrect content type used: %s", contentType)
	}

	return journalReader, nil
}

// Reader is the main Journal Reader structure. It implements Reader interface.
type Reader struct {
	Journal *sdjournal.Journal
	Limit   *Num

	msgReader *bytes.Reader
	getDataFn func() ([]byte, error)
}

// Read is implementation of Reader interface.
// Most of the code was taken from https://github.com/coreos/go-systemd/blob/master/sdjournal/read.go
func (j *Reader) Read(b []byte) (int, error) {
	if j.msgReader == nil {
		// check if we reached the limit.
		if j.Limit != nil && j.Limit.Value == 0 {
			return 0, io.EOF
		}

		// advance the journal cursor. It has to be called at least one time
		// before reading
		c, err := j.Journal.Next()
		if err != nil {
			return 0, err
		}

		// EOF detection
		if c == 0 {
			return 0, io.EOF
		}

		// make sure we initialized the getDataFn
		if j.getDataFn == nil {
			return 0, fmt.Errorf("Uninitialized getDataFn. NewReader() must be called before using reader")
		}

		entry, err := j.getDataFn()
		if err != nil {
			return 0, err
		}

		// make a trick and put the entry in array of bytes.
		j.msgReader = bytes.NewReader(entry)

		// if we are using a limited number of entries, decrement a counter.
		if j.Limit != nil && j.Limit.Value > 0 {
			j.Limit.Value--
		}
	}

	var sz int
	sz, err := j.msgReader.Read(b)
	if err == io.EOF {
		j.msgReader = nil
		return sz, nil
	}

	if err != nil {
		return sz, err
	}

	if j.msgReader.Len() == 0 {
		j.msgReader = nil
	}

	return sz, nil
}

// GetSSEEntry returns a log entry in SSE format.
// data: {"key1": "value1"}\n\n
func (j *Reader) GetSSEEntry() ([]byte, error) {
	entry, err := j.GetJSONEntry()
	if err != nil {
		return []byte(""), err
	}

	// Server sent events require \n\n at the end of the entry.
	fullEntry := "data: " + string(entry) + "\n"
	fullEntryBytes := []byte(fullEntry)
	return fullEntryBytes, nil
}

// GetJSONEntry returns logs entries in JSON format.
func (j *Reader) GetJSONEntry() (entryBytes []byte, err error) {
	entry, err := j.Journal.GetEntry()
	if err != nil {
		return entryBytes, err
	}

	entryBytes, err = json.Marshal(entry)
	if err != nil {
		return entryBytes, err
	}

	entryPostfix := "\n"
	entryBytes = append(entryBytes, []byte(entryPostfix)...)
	return entryBytes, nil

}

// GetTextEntry returns log entries in plain text
func (j *Reader) GetTextEntry() (textEntry []byte, err error) {
	// text format: "date _HOSTNAME SYSLOG_IDENTIFIER[_PID]: MESSAGE
	entry, err := j.Journal.GetEntry()
	if err != nil {
		return textEntry, err
	}

	// entry.RealtimeTimestamp returns a unix time in microseconds
	// https://www.freedesktop.org/software/systemd/man/sd_journal_get_realtime_usec.html
	t := time.Unix(int64(entry.RealtimeTimestamp)/1000000, 0)
	date := t.Format(time.ANSIC)
	hostname := entry.Fields["_HOSTNAME"]
	syslogID := entry.Fields["SYSLOG_IDENTIFIER"]
	pid := entry.Fields["_PID"]
	message := entry.Fields["MESSAGE"]
	textEntry = []byte(fmt.Sprintf("%s %s %s[%s]: %s\n", date, hostname, syslogID, pid, message))
	return
}
