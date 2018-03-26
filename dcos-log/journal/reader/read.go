package reader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/coreos/go-systemd/sdjournal"
	"github.com/sirupsen/logrus"
)

// ErrUninitializedReader is the error returned by Reader is contentFormatter wasn't initialized.
// An instance of Reader must always be obtained by calling `NewReader` constructor function.
var ErrUninitializedReader = errors.New("NewReader() must be called before using journal reader")

// NewReader returns a new instance of journal reader.
func NewReader(contentFormatter EntryFormatter, options ...Option) (r *Reader, err error) {
	// if contentFormatter is not set, use FormatText by default.
	if contentFormatter == nil {
		contentFormatter = FormatText{}
	}

	r = &Reader{
		contentFormatter: contentFormatter,
	}

	r.Journal, err = sdjournal.NewJournal()
	if err != nil {
		return r, err
	}

	// apply options
	for _, opt := range options {
		if opt != nil {
			if err := opt(r); err != nil {
				return r, err
			}
		}
	}

	return r, nil
}

// Reader is the main Journal Reader structure. It implements Reader interface.
type Reader struct {
	Journal                  *sdjournal.Journal
	Cursor                   string
	Limit                    uint64
	UseLimit                 bool
	SkippedNext, SkippedPrev uint64
	ReadReverse              bool

	eofTime          time.Time
	msgReader        *bytes.Reader
	contentFormatter EntryFormatter
	// n represents the number of logs read.
	n uint64

	// matchFns contains a list of match functions the user used in the original constructor.
	// this is useful to re-apply matches in some cases (for instance journald rotation)
	matchFns []func(journal *sdjournal.Journal)
}

// SkipNext skips a journal by n entries forward.
func (r *Reader) SkipNext(n uint64) error {
	var err error
	r.SkippedNext, err = r.Journal.NextSkip(n)
	return err
}

// SkipPrev skips a journal by n entries backwards.
func (r *Reader) SkipPrev(n uint64) error {
	// if Cursor was not specified, move to the tail first
	if r.Cursor == "" {
		if err := r.Journal.SeekTail(); err != nil {
			return fmt.Errorf("Could not move to the end if the journal: %s", err)
		}
	}

	var err error
	r.SkippedPrev, err = r.Journal.PreviousSkip(n)
	return err
}

// SeekCursor looks for a specific cursor in the journal and moves to it.
// Function returns an error if cursor not found.
func (r *Reader) SeekCursor(c string) error {
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

		var (
			c        uint64
			err      error
			skipRead bool
		)
		// The problem here is the following. When we read the journal for the first time we have to advance
		// the cursor to read the very first entry. However when we move the cursor backwards with skip option
		// `OptionSkipPrev` the cursor will be pointing to an actual entry which we want to read. In this case
		// we have to be aware how many entries we already read and whether we can read the current cursor.

		// only check if we need to move the cursor for the first time.
		// if user used a specific cursor in the request we should check if we are pointing to it.
		// if we are, we should not read the same entry and move to the next one.
		if r.n == 0 {
			// if we can read the cursor without errors we should NOT advance the cursor for the first time.
			// However, if the user provided a cursor in the request, we should not read, we have to move on
			// to the next.
			if cursor, err := r.Journal.GetCursor(); err == nil {
				if cursor != r.Cursor {
					skipRead = true
				}
			}
		}

		if !skipRead {
			if r.ReadReverse {
				c, err = r.Journal.Previous()
			} else {
				c, err = r.Journal.Next()
			}
			if err != nil {
				return 0, err
			}

			// EOF detection
			if c == 0 {
				// for server sent events content type some proxies may close connection
				// after a short timeout. We are going to send a ping comment every 15 seconds
				// if no data available. This will ensure the connection is kept alive and
				// nginx will not drop it with `Connection timed out` error.
				// https://html.spec.whatwg.org/multipage/comms.html
				if r.contentFormatter.GetContentType() == ContentTypeEventStream {
					if time.Since(r.eofTime) < time.Duration(time.Second*15) {
						return 0, io.EOF
					}

					r.msgReader = bytes.NewReader([]byte(": ping\n\n"))
					r.eofTime = time.Now()
					goto reader
				}
				return 0, io.EOF
			}
		}
		// update the timer indicating we are not idling
		r.eofTime = time.Now()

		if r.contentFormatter == nil {
			return 0, ErrUninitializedReader
		}

		entry, err := r.Journal.GetEntry()
		if err != nil {
			return 0, err
		}

		entryBytes, err := r.contentFormatter.FormatEntry(entry)
		if err != nil {
			return 0, err
		}

		// make a trick and put the entry in array of bytes.
		r.msgReader = bytes.NewReader(entryBytes)

		// if we are using a limited number of entries, decrement a counter.
		if r.UseLimit && r.Limit > 0 {
			r.Limit--
		}

		r.n++
	}

reader:
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

// Close is a function to close the journal. Along with Read() function it implements io.ReadCloser
func (r *Reader) Close() error {
	if r.Journal == nil {
		return ErrUninitializedReader
	}
	return r.Journal.Close()
}

// Follow is a wrapper function, which can be called multiple times to mimic a journal tailing.
func (r *Reader) Follow(wait time.Duration, writer io.Writer) error {
	n, err := io.Copy(writer, r)
	if err != nil && err != io.EOF {
		return err
	}

	// if the number of read lines more then 0, we did not reach the journald bottom and can exit early
	if n > 0 {
		return nil
	}

	// if we reached the journald bottom, we'll have to wait and learn the current status of journald
	// SD_JOURNAL_INVALIDATE indicates that the journald files were removed from the filesystem and now we need to close
	// the opened files handlers and reopened with original user parameters.
	// https://www.freedesktop.org/software/systemd/man/sd_journal_get_fd.html#Return%20Value
	if r.Journal.Wait(wait) == sdjournal.SD_JOURNAL_INVALIDATE {
		logrus.Infof("SD_JOURNAL_INVALIDATE, reopened journal")

		cursor, err := r.Journal.GetCursor()
		if err != nil {
			return fmt.Errorf("unable to get current cursor: %s", err)
		}

		// close journal to release the file handler
		err = r.Journal.Close()
		if err != nil {
			return fmt.Errorf("unable to close current instance of journald: %s", err)
		}

		// open a new journald
		newJournal, err := sdjournal.NewJournal()
		if err != nil {
			return fmt.Errorf("unable to open a new instance of journald: %s", err)
		}

		// apply the original matches to a new instance of journal
		// we only need to apply the matches since all other user parameters live in the Reader structure which
		// was not changed.
		for _, fn := range r.matchFns {
			fn(newJournal)
		}

		// update the journal instance
		r.Journal = newJournal

		// systemd bug has a weird bug in versions < v236 (a fix for the bug https://github.com/systemd/systemd/pull/5580)
		// it's quite possible to execute the lines in this block, even if the journald files were not rotated.
		// So we need to know, if we are in the old journald log or a new one. The easiest method would be
		// to search for the original journald cursor. If we found the cursor, we are in the same log, otherwise
		// journald was rotated and we are in a brand new log file and we have to read from the beginning.

		// we want to intentionally ignore the error message, since it would indicate rotated systemd file
		if err := r.SeekCursor(cursor); err != nil {
			logrus.Errorf("error search cursor %s. %s", cursor, err)
		}
	}

	// other possible statues are
	// SD_JOURNAL_NOP - means that the journal did not change since the last invocation and we can just exit without
	// errors.
	// SD_JOURNAL_APPEND - means that new entries were appended to the end of the journal and next time the client
	// runs the Follow() function again, they would be displayed. But for now, we can exit without errors.

	return nil
}
