package reader

import (
	"net/http"
	"fmt"
)

// Option ...
type Option func(*ReadManager) error

func OptLines(n int) Option {
	return func(rm *ReadManager) error {
		rm.n = n
		return nil
	}
}

func OptFile(f string) Option {
	return func(rm *ReadManager) error {
		rm.File = f
		return nil
	}
}

func OptHeaders(h http.Header) Option {
	return func(rm *ReadManager) error {
		rm.header = h
		return nil
	}
}

func OptReadDirection(r ReadDirection) Option {
	return func(rm *ReadManager) error {
		rm.readDirection = r
		return nil
	}
}

func OptStream(stream bool) Option {
	return func(rm *ReadManager) error {
		rm.stream = stream
		return nil
	}
}

func OptOffset(offset int) Option {
	return func(rm *ReadManager) error {
		if offset < 0 {
			return fmt.Errorf("invalid offset %d. Must be positive integer", offset)
		}
		rm.offset = offset
		return nil
	}
}