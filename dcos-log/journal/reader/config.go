package reader

import (
	"errors"
	"fmt"
)

// EntriesLimit defines a number of entries to read.
type EntriesLimit uint64

// JournalReaderConfig is a structure that defines journal reader options.
type JournalReaderConfig struct {
	// Cursor defines a cursor position in the journal.
	Cursor string

	//ContentType defines a response format.
	ContentType ContentType

	// Limit sets a limited number of entries to display.
	// If not set, do not use any limitation, show all entries until we hit io.EOF
	Limit uint64

	UseLimit bool

	// SkipNext skips number of entries from the current cursor position forward.
	SkipNext uint64

	// SkipPrev skips number of entries from the current cursor position backward.
	SkipPrev uint64

	// Matches is an array of filters to match.
	Matches []Match
}

// Validate makes sure the config has a valid options.
func (j *JournalReaderConfig) Validate() error {
	if j.SkipNext != 0 && j.SkipPrev != 0 {
		return errors.New("Cannot have SkipNext and SkipPrev at the same time")
	}

	switch j.ContentType {
	case ContentTypeText:
	case ContentTypeJSON:
	case ContentTypeStream:
	default:
		return fmt.Errorf("Incorrect content type used: %s", j.ContentType)
	}

	return nil
}

// Match is a convenience wrapper to describe filters supplied to AddMatch.
type Match struct {
	Field, Value string
}

// String returns a string representation of a Match suitable for use with AddMatch.
func (m *Match) String() string {
	return m.Field + "=" + m.Value
}
