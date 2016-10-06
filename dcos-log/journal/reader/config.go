package reader

// Option is a functional option that configures a Reader.
type Option func(*Reader) error

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

		for _, match := range m {
			r.Journal.AddMatch(match.String())
		}

		return nil
	}
}

// OptionSeekCursor is a functional option that seeks a cursor in the journal.
func OptionSeekCursor(c string) Option {
	return func(r *Reader) error {
		if c != "" {
			r.Cursor = c
			return r.SeekCursor(c)
		}
		return nil
	}
}

// OptionSkipNext is a functional option that skips forward N journal entries from the current cursor position.
func OptionSkipNext(n uint64) Option {
	return func(r *Reader) error {
		if n != 0 {
			return r.SkipNext(n)
		}
		return nil
	}
}

// OptionSkipPrev is a functional option that skips backward N journal entries from the current cursor position.
func OptionSkipPrev(n uint64) Option {
	return func(r *Reader) error {
		if n != 0 {
			return r.SkipPrev(n)
		}
		return nil
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
