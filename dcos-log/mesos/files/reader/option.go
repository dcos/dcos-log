package reader

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Option is a functional parameters interface.
type Option func(*ReadManager) error

// OptLines limits a number of lines to be returned to a client.
func OptLines(n int) Option {
	return func(rm *ReadManager) error {
		rm.n = n
		return nil
	}
}

// OptSkip skips the number of lines.
func OptSkip(n int) Option {
	return func(rm *ReadManager) error {
		rm.skip = n
		return nil
	}
}

// OptFile sets the filename to read.
func OptFile(f string) Option {
	return func(rm *ReadManager) error {
		rm.File = f
		return nil
	}
}

// OptHeaders sets the optional request header.
func OptHeaders(h http.Header) Option {
	return func(rm *ReadManager) error {
		rm.header = h
		return nil
	}
}

// OptReadDirection sets the direction the journal must be read.
func OptReadDirection(r ReadDirection) Option {
	return func(rm *ReadManager) error {
		rm.readDirection = r
		return nil
	}
}

// OptStream sets the flag to stream the logs. (do not close the connection)
func OptStream(stream bool) Option {
	return func(rm *ReadManager) error {
		rm.stream = stream
		return nil
	}
}

// OptOffset sets the offset in the file.
func OptOffset(offset int) Option {
	return func(rm *ReadManager) error {
		if offset < 0 {
			return fmt.Errorf("invalid offset %d. Must be positive integer", offset)
		}
		rm.offset = offset
		return nil
	}
}

// OptReadFromEnd moves the cursor to the end of file.
func OptReadFromEnd() Option {
	return func(rm *ReadManager) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		offset, err := rm.fileLen(ctx)
		if err != nil {
			return err
		}

		return OptOffset(offset)(rm)
	}
}
