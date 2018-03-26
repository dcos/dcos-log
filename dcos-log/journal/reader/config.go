package reader

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/sirupsen/logrus"
)

var (
	// ErrCursorFormat is the error thrown by OptionSeekCursor if cursor string is invalid.
	ErrCursorFormat = errors.New("Incorrect cursor string")

	// ErrInvalidDuration is the error thrown by OptionSince if negative or zero duration used.
	ErrInvalidDuration = errors.New("Invalid duration parameter")
)

// Option is a functional option that configures a Reader.
type Option func(*Reader) error

// OptionReadReverse is a functional option sets a reverse direction to read the journal.
// By default we always read the journal up to down. If we use this option, we'll be reading the journal
// in reverse.
func OptionReadReverse(reverse bool) Option {
	return func(r *Reader) error {
		r.ReadReverse = reverse
		return nil
	}
}

// OptionLimit is a functional option sets a limit of entries to read from a journal.
func OptionLimit(n uint64) Option {
	return func(r *Reader) error {
		r.Limit = n
		r.UseLimit = n > 0
		return nil
	}
}

// OptionMatch is a functional option that filters entries based on []JournalEntryMatch.
func OptionMatch(m []JournalEntryMatch) Option {
	return func(r *Reader) error {
		if r.Journal == nil {
			return ErrUninitializedReader
		}

		fn := func(journal *sdjournal.Journal) {
			for _, match := range m {
				journal.AddMatch(match.String())
			}
		}

		// apply matches for current optional parameter
		fn(r.Journal)

		// store the function in case we need to re-apply the matches
		r.matchFns = append(r.matchFns, fn)

		return nil
	}
}

// OptionMatchOR is a functional option that filters entries and applies logical OR to user
// arguments []JournalEntryMatch.
func OptionMatchOR(m []JournalEntryMatch) Option {
	return func(r *Reader) error {
		if r.Journal == nil {
			return ErrUninitializedReader
		}

		fn := func(journal *sdjournal.Journal) {
			for _, match := range m {
				journal.AddMatch(match.String())
				journal.AddDisjunction()
				logrus.Infof("adding OR match %s", match)
			}
		}

		// apply matches for current optional parameter
		fn(r.Journal)

		// store the function in case we need to re-apply the matches
		r.matchFns = append(r.matchFns, fn)

		return nil
	}
}

// OptionSeekCursor is a functional option that seeks a cursor in the journal.
func OptionSeekCursor(c string) Option {
	return func(r *Reader) error {
		if c == "" {
			return nil
		}

		if err := validateCursor(c); err != nil {
			return err
		}

		r.Cursor = c
		return r.SeekCursor(c)
	}
}

// OptionSkipNext is a functional option that skips forward N journal entries from the current cursor position.
func OptionSkipNext(n uint64) Option {
	return func(r *Reader) error {
		if n > 0 {
			return r.SkipNext(n)
		}
		return nil
	}
}

// OptionSkipPrev is a functional option that skips backward N journal entries from the current cursor position.
func OptionSkipPrev(n uint64) Option {
	return func(r *Reader) error {
		if n > 0 {
			return r.SkipPrev(n)
		}
		return nil
	}
}

// OptionSince is a functional option that implements journalctl --since analogue.
func OptionSince(d time.Duration) Option {
	return func(r *Reader) error {
		if d <= 0 {
			return ErrInvalidDuration
		}
		start := time.Now().Add(-d)
		return r.Journal.SeekRealtimeUsec(uint64(start.UnixNano() / 1000))
	}
}

// JournalEntryMatch is a convenience wrapper to describe filters supplied to AddMatch.
type JournalEntryMatch struct {
	Field, Value string
}

// String returns a string representation of a Match suitable for use with AddMatch.
func (m *JournalEntryMatch) String() string {
	return m.Field + "=" + m.Value
}

func validateCursor(c string) error {
	parseKeyValueStr := func(s string) (string, string, error) {
		sArray := strings.Split(s, "=")
		if len(sArray) != 2 {
			return "", "", ErrCursorFormat
		}
		return sArray[0], sArray[1], nil
	}

	parseHexUint64 := func(s string) error {
		_, err := strconv.ParseUint(s, 16, 64)
		if err != nil {
			return ErrCursorFormat
		}
		return nil
	}

	validateString := func(s, k string) error {
		key, value, err := parseKeyValueStr(s)
		if err != nil {
			return err
		}

		if key != k {
			return ErrCursorFormat
		}

		// https://github.com/systemd/systemd/blob/master/src/journal/sd-journal.c#L920
		if len(value) > 33 {
			return ErrCursorFormat
		}
		return nil
	}

	validateHexUint64 := func(s, k string) error {
		key, value, err := parseKeyValueStr(s)
		if err != nil {
			return err
		}

		if key != k {
			return ErrCursorFormat
		}

		if err := parseHexUint64(value); err != nil {
			return ErrCursorFormat
		}

		return nil
	}

	// https://github.com/systemd/systemd/blob/master/src/journal/sd-journal.c#L937
	cursorFormat := []struct {
		fieldKey   string
		validateFn func(string, string) error
	}{
		{
			fieldKey:   "s",
			validateFn: validateString,
		},
		{
			fieldKey:   "i",
			validateFn: validateHexUint64,
		},
		{
			fieldKey:   "b",
			validateFn: validateString,
		},
		{
			fieldKey:   "m",
			validateFn: validateHexUint64,
		},
		{
			fieldKey:   "t",
			validateFn: validateHexUint64,
		},
		{
			fieldKey:   "x",
			validateFn: validateHexUint64,
		},
	}
	cursorSplit := strings.Split(c, ";")
	if len(cursorSplit) != len(cursorFormat) {
		return ErrCursorFormat
	}

	for index, cursorField := range cursorFormat {
		if err := cursorField.validateFn(cursorSplit[index], cursorField.fieldKey); err != nil {
			return err
		}
	}

	return nil
}
