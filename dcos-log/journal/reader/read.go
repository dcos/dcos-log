package reader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Sirupsen/logrus"
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

// NewReader returns a new instance of journal reader.
func NewReader(config JournalReaderConfig) (journalReader *Reader, err error) {
	if err := config.Validate(); err != nil {
		return journalReader, fmt.Errorf("Invalid config: %s", err)
	}

	journalReader = &Reader{
		Limit:    config.Limit,
		UseLimit: config.Limit > 0,
		Cursor:   config.Cursor,
	}

	journalReader.Journal, err = sdjournal.NewJournal()
	if err != nil {
		return journalReader, err
	}

	// Add any supplied matches
	for _, m := range config.Matches {
		journalReader.Journal.AddMatch(m.String())
	}

	// move cursor to a specific location
	if err := journalReader.SeekCursor(config.Cursor); err != nil {
		return journalReader, err
	}

	// move cursor forward if available
	if err := journalReader.NextSkip(config.SkipNext); err != nil {
		return journalReader, err
	}

	// move cursor backwards if available
	if err := journalReader.PreviousSkip(config.SkipPrev); err != nil {
		return journalReader, err
	}

	switch config.ContentType {
	case ContentTypeText:
		journalReader.getDataFn = journalReader.GetTextEntry
	case ContentTypeJSON:
		journalReader.getDataFn = journalReader.GetJSONEntry
	case ContentTypeStream:
		journalReader.getDataFn = journalReader.GetSSEEntry
	default:
		return journalReader, fmt.Errorf("Incorrect content type used: %s", config.ContentType)
	}

	return journalReader, nil
}

// Reader is the main Journal Reader structure. It implements Reader interface.
type Reader struct {
	Journal  *sdjournal.Journal
	Cursor   string
	Limit    uint64
	UseLimit bool

	msgReader *bytes.Reader
	getDataFn func() ([]byte, error)
}

// NextSkip skips a journal by n entries forward.
func (r *Reader) NextSkip(n uint64) error {
	if n == 0 {
		logrus.Debug("Skipping `NextSkip`")
		return nil
	}
	_, err := r.Journal.NextSkip(n)
	return err
}

// PreviousSkip skips a journal by n entries backward.
func (r *Reader) PreviousSkip(n uint64) error {
	if n == 0 {
		logrus.Debug("Skipping `PreviousSkip`")
		return nil
	}

	// if Cursor was not specified, move to the tail first
	if r.Cursor == "" {
		if err := r.Journal.SeekTail(); err != nil {
			return fmt.Errorf("Could not move to the end if the journal: %s", err)
		}
	}
	_, err := r.Journal.PreviousSkip(n)
	return err
}

// SeekCursor looks for a specific cursor in the journal and moves to it.
// Function returns an error if cursor not found.
func (r *Reader) SeekCursor(c string) error {
	// return if no cursor passed
	if c == "" {
		return nil
	}

	if err := r.Journal.SeekCursor(c); err != nil {
		return err
	}

	// Advance cursor
	if _, err := r.Journal.Next(); err != nil {
		return err
	}

	// Verify we got moved the cursor to the desired position
	if err := r.Journal.TestCursor(c); err != nil {
		return fmt.Errorf("Cursor %s not found: %s", c, err)
	}

	// now if we found the cursor, let's move it a step back to the original position
	if _, err := r.Journal.Previous(); err != nil {
		return err
	}
	return nil
}

// Read is implementation of Reader interface.
// Most of the code was taken from https://github.com/coreos/go-systemd/blob/master/sdjournal/read.go
func (r *Reader) Read(b []byte) (int, error) {
	if r.msgReader == nil {
		// check if we reached the limit.
		if r.UseLimit && r.Limit == 0 {
			return 0, io.EOF
		}

		// advance the journal cursor. It has to be called at least one time
		// before reading
		c, err := r.Journal.Next()
		if err != nil {
			return 0, err
		}

		// EOF detection
		if c == 0 {
			return 0, io.EOF
		}

		// make sure we initialized the getDataFn
		if r.getDataFn == nil {
			return 0, fmt.Errorf("Uninitialized getDataFn. NewReader() must be called before using reader")
		}

		entry, err := r.getDataFn()
		if err != nil {
			return 0, err
		}

		// make a trick and put the entry in array of bytes.
		r.msgReader = bytes.NewReader(entry)

		// if we are using a limited number of entries, decrement a counter.
		if r.UseLimit && r.Limit > 0 {
			r.Limit--
		}
	}

	var sz int
	sz, err := r.msgReader.Read(b)
	if err == io.EOF {
		r.msgReader = nil
		return sz, nil
	}

	if err != nil {
		return sz, err
	}

	if r.msgReader.Len() == 0 {
		r.msgReader = nil
	}

	return sz, nil
}

// GetSSEEntry returns a log entry in SSE format.
// data: {"key1": "value1"}\n\n
func (r *Reader) GetSSEEntry() ([]byte, error) {
	entry, err := r.GetJSONEntry()
	if err != nil {
		return []byte(""), err
	}

	// Server sent events require \n\n at the end of the entry.
	fullEntry := "data: " + string(entry) + "\n"
	fullEntryBytes := []byte(fullEntry)
	return fullEntryBytes, nil
}

// GetJSONEntry returns logs entries in JSON format.
func (r *Reader) GetJSONEntry() (entryBytes []byte, err error) {
	entry, err := r.Journal.GetEntry()
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
func (r *Reader) GetTextEntry() (textEntry []byte, err error) {
	// text format: "date _HOSTNAME SYSLOG_IDENTIFIER[_PID]: MESSAGE
	entry, err := r.Journal.GetEntry()
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
