package reader

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
)

// ContentType is used in response header.
type ContentType string

var (
	// ContentTypePlainText is a ContentType header for plain text logs.
	ContentTypePlainText ContentType = "text/plain"

	// ContentTypeApplicationJSON is a ContentType header for json logs.
	ContentTypeApplicationJSON ContentType = "application/json"

	// ContentTypeEventStream is a ContentType header for event-stream logs.
	ContentTypeEventStream ContentType = "text/event-stream"
)

// NewEntryFormatter returns a new implementation of EntryFormatter corresponding to a given content type.
func NewEntryFormatter(s string, useCursorID bool) EntryFormatter {
	if s == ContentTypeApplicationJSON.String() {
		return &FormatJSON{}
	}

	if s == ContentTypeEventStream.String() {
		return &FormatSSE{
			UseCursorID: useCursorID,
		}
	}

	return &FormatText{}
}

// String returns a string representation of type "ContentType"
func (c ContentType) String() string {
	return string(c)
}

// EntryFormatter is an interface used by journal to write in a specific format.
type EntryFormatter interface {
	// GetContentType returns a content type for the entry formatter.
	GetContentType() ContentType

	// FormatEntry accepts `sdjournal.JournalEntry` and returns an array of bytes.
	FormatEntry(*sdjournal.JournalEntry) ([]byte, error)
}

// FormatText implements EntryFormatter for text logs.
type FormatText struct{}

// GetContentType returns "text/plain"
func (j FormatText) GetContentType() ContentType {
	return ContentTypePlainText
}

// FormatEntry formats sdjournal.JournalEntry to a text log line.
func (j FormatText) FormatEntry(entry *sdjournal.JournalEntry) ([]byte, error) {
	// return empty if field MESSAGE not found
	message, ok := entry.Fields["MESSAGE"]
	if !ok {
		return nil, nil
	}

	// entry.RealtimeTimestamp returns a unix time in microseconds
	// https://www.freedesktop.org/software/systemd/man/sd_journal_get_realtime_usec.html
	t := time.Unix(int64(entry.RealtimeTimestamp)/1000000, 0)
	line := []byte(fmt.Sprintf("%s: %s\n", t.Format("2006-01-02 15:04:05"), message))

	return line, nil
}

// FormatJSON implements EntryFormatter for json logs.
type FormatJSON struct{}

// GetContentType returns "application/json"
func (j FormatJSON) GetContentType() ContentType {
	return ContentTypeApplicationJSON
}

// FormatEntry formats sdjournal.JournalEntry to a json log entry.
func (j FormatJSON) FormatEntry(entry *sdjournal.JournalEntry) ([]byte, error) {
	entryBytes, err := marshalJournalEntry(entry)
	if err != nil {
		return entryBytes, err
	}

	entryPostfix := []byte("\n")
	return append(entryBytes, entryPostfix...), nil
}

// FormatSSE implements EntryFormatter for server sent event logs.
// Must be in the following format: data: {...}\n\n
type FormatSSE struct {
	UseCursorID bool
}

// GetContentType returns "text/event-stream"
func (j FormatSSE) GetContentType() ContentType {
	return ContentTypeEventStream
}

// FormatEntry formats sdjournal.JournalEntry to a server sent event log entry.
func (j FormatSSE) FormatEntry(entry *sdjournal.JournalEntry) ([]byte, error) {
	// Server sent events require \n\n at the end of the entry.
	entryBytes, err := marshalJournalEntry(entry)
	if err != nil {
		return entryBytes, err
	}

	entryPrefix := []byte("data: ")
	entryPostfix := []byte("\n\n")
	entryWithPostfix := append(entryBytes, entryPostfix...)
	entrySSE := append(entryPrefix, entryWithPostfix...)

	// if FormatSSE was initiated with useCursorID flag, then add id: cursor before the data.
	if j.UseCursorID {
		id := []byte(fmt.Sprintf("id: %s\n", entry.Cursor))
		entrySSE = append(id, entrySSE...)
	}
	return entrySSE, nil
}

func marshalJournalEntry(entry *sdjournal.JournalEntry) ([]byte, error) {
	formattedEntry := struct {
		Fields             map[string]string `json:"fields"`
		Cursor             string            `json:"cursor"`
		MonotonicTimestamp uint64            `json:"monotonic_timestamp"`
		RealtimeTimestamp  uint64            `json:"realtime_timestamp"`
	}{
		Fields:             entry.Fields,
		Cursor:             entry.Cursor,
		MonotonicTimestamp: entry.MonotonicTimestamp,
		RealtimeTimestamp:  entry.RealtimeTimestamp,
	}

	return json.Marshal(formattedEntry)
}
