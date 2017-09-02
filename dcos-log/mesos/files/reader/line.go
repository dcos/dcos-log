package reader

import (
	"bytes"
	"encoding/json"
	"io"
)

// Line is a structure for a line message with offset.
type Line struct {
	Message string
	Offset  int
}

// LinesReader is structure that implements a simple queue and a reader. When the reader reads a line
// it is being removed from the queue.
type LinesReader struct {
	nLines int
	lines  []*Line

	read int
}

func (lr LinesReader) Len() int {
	return len(lr.lines)
}

func (lr *LinesReader) Read(b []byte) (int, error) {
	if lr.read == lr.nLines {
		return 0, io.EOF
	}

	line := lr.Pop()
	if line == nil {
		return 0, io.EOF
	}

	body, err := json.Marshal(line.Message)
	if err != nil {
		return 0, err
	}

	body = append(body, '\n')
	lr.read++
	return bytes.NewReader(body).Read(b)
}

func (lr *LinesReader) Prepand(l *Line) {
	old := lr.lines
	lr.lines = append([]*Line{l}, old...)
}

func (lr *LinesReader) Pop() *Line {
	old := *lr
	n := len(old.lines)
	if n == 0 {
		return nil
	}

	x := old.lines[n-1]
	lr.lines = old.lines[0 : n-1]
	return x
}
