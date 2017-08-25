package reader

import "net/http"

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
