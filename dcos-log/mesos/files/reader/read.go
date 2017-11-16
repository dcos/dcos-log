package reader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	chunkSize = 1 << 16
)

var (
	// ErrEmptyParam ...
	ErrEmptyParam = errors.New("args cannot be empty")

	// ErrNoData is an error returned by Read(). It indicates that the buffer is empty
	// and we need to request more data from mesos files API.
	ErrNoData = errors.New("new data needed")
)

type response struct {
	Data   string `json:"data"`
	Offset int    `json:"offset"`
}

type modifier func(s string) string

func notEmpty(args []string) error {
	if len(args) == 0 {
		return ErrEmptyParam
	}

	for _, arg := range args {
		if arg == "" {
			return ErrEmptyParam
		}
	}

	return nil
}

// NewLineReader is a ReadManager constructor.
func NewLineReader(client *http.Client, masterURL url.URL, agentID, frameworkID, executorID, containerID, taskPath, file string,
	format Formatter, opts ...Option) (*ReadManager, error) {

	// make sure the required parameters are set properly
	if err := notEmpty([]string{agentID, frameworkID, executorID, containerID}); err != nil {
		return nil, err
	}

	sandboxPath := fmt.Sprintf("/var/lib/mesos/slave/slaves/%s/frameworks/%s/executors/%s/runs/%s/", agentID, frameworkID, executorID, containerID)
	if taskPath != "" {
		sandboxPath += fmt.Sprintf("tasks/%s/", taskPath)
	}

	rm := &ReadManager{
		client: client,

		File:         file,
		readEndpoint: masterURL,
		sandboxPath:  sandboxPath,
		formatFn:     format,
	}

	for _, opt := range opts {
		if opt != nil {
			if err := opt(rm); err != nil {
				return nil, err
			}
		}
	}

	if rm.readDirection == BottomToTop {
		foundLines := 0
		offset := rm.offset - chunkSize
		for {
			// if the offset is 0 or negative value, the means we reached the top of the file.
			// we can just set the offset to 0 and read the entire file
			if offset < 1 {
				offset = 0
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)

			lines, delta, err := rm.read(ctx, offset, chunkSize, reverse)
			if err != nil {
				cancel()
				return nil, err
			}
			cancel()

			skip := rm.skip

			// make skip a positive number
			if skip < 0 {
				skip = rm.skip * -1
			}

			// if the required number of lines found, we need to calculate an offset
			if foundLines+len(lines) >= skip {
				diff := skip - foundLines
				for i := 0; i <= diff; i++ {
					rm.offset -= len(lines[i].Message) + 1
				}
				break
			} else {
				// if the current chunk contains less then requested lines, then add to a counter
				// and continue search.
				foundLines += len(lines)
			}

			offset -= chunkSize - delta
			rm.offset = offset
		}
	}

	// guard against negative offset
	if rm.offset < 0 {
		rm.offset = 0
	}

	return rm, nil
}

// ReadDirection specifies the direction files API will be read.
type ReadDirection int

const (
	// TopToBottom reads files API from top to bottom.
	TopToBottom ReadDirection = iota

	// BottomToTop reads files API from bottom to top.
	BottomToTop
)

// ReadManager is a mesos files API reader. It reads builds the correct sandbox path to files
// and implements io.Reader.
// http://mesos.apache.org/documentation/latest/endpoints/files/read/
type ReadManager struct {
	client       *http.Client
	readEndpoint url.URL
	sandboxPath  string
	header       http.Header

	readDirection ReadDirection
	n             int
	skip          int
	skipped       int
	File          string

	size   int
	offset int
	lines  []Line

	readLines int
	stream    bool

	formatFn Formatter
}

func (rm *ReadManager) do(req *http.Request) (*response, error) {
	resp, err := rm.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status %d", resp.StatusCode)
	}

	data := &response{}
	if err := json.NewDecoder(resp.Body).Decode(data); err != nil {
		return nil, err
	}

	return data, nil
}

func (rm *ReadManager) fileLen(ctx context.Context) (int, error) {
	v := url.Values{}
	v.Add("path", rm.sandboxPath+rm.File)
	v.Add("offset", "-1")
	newURL := rm.readEndpoint
	newURL.RawQuery = v.Encode()

	logrus.Info(newURL.String())
	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return 0, err
	}
	req.Header = rm.header

	resp, err := rm.do(req.WithContext(ctx))
	if err != nil {
		return 0, err
	}

	return resp.Offset, nil
}

func (rm *ReadManager) read(ctx context.Context, offset, length int, modifier modifier) ([]Line, int, error) {
	v := url.Values{}
	v.Add("path", rm.sandboxPath+rm.File)
	v.Add("offset", strconv.Itoa(offset))
	v.Add("length", strconv.Itoa(length))

	if modifier == nil {
		modifier = func(s string) string { return s }
	}

	newURL := rm.readEndpoint
	newURL.RawQuery = v.Encode()

	logrus.Info(newURL.String())

	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return nil, 0, err
	}

	req.Header = rm.header
	resp, err := rm.do(req.WithContext(ctx))
	if err != nil {
		return nil, 0, err
	}
	lines := strings.Split(modifier(resp.Data), "\n")

	delta := 0
	if len(lines) > 1 {
		delta = len(lines[len(lines)-1])
		lines = lines[:len(lines)-1]
	}

	linesWithOffset := make([]Line, len(lines))
	// accumulates the position of the line + \n
	accumulator := 0
	for i := 0; i < len(lines); i++ {
		linesWithOffset[i] = Line{
			Message: lines[i],
			Offset:  offset + accumulator,
			Size:    len(lines[i]),
		}
		accumulator += len(lines[i]) + 1
	}

	return linesWithOffset, delta, nil
}

// Prepand the lines to a buffer.
func (rm *ReadManager) Prepand(s Line) {
	if s.Message == "" {
		return
	}
	old := rm.lines
	rm.lines = append([]Line{s}, old...)
}

// Pop returns a line from the end of the buffer.
func (rm *ReadManager) Pop() *Line {
	old := *rm
	n := len(old.lines)
	if n == 0 {
		return nil
	}

	x := old.lines[n-1]
	rm.lines = old.lines[0 : n-1]
	return &x
}

// Read implements io.Reader interface.
func (rm *ReadManager) Read(b []byte) (int, error) {
start:
	if !rm.stream && rm.n > 0 && rm.readLines == rm.n {
		return 0, io.EOF
	}

	if len(rm.lines) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		lines, delta, err := rm.read(ctx, rm.offset, chunkSize, nil)
		if err != nil {
			return 0, err
		}

		if len(lines) > 1 {
			linesLen := 0
			for _, line := range lines {
				rm.Prepand(line)
				linesLen += len(line.Message) + 1
			}

			if linesLen < chunkSize {
				rm.offset = rm.offset + linesLen - 1
			} else {
				rm.offset = (rm.offset + chunkSize) - delta - 1
			}
		} else {
			return 0, io.EOF
		}
	}

	line := rm.Pop()
	if line == nil {
		return 0, ErrNoData
	}

	if rm.skip > 0 && rm.skipped < rm.skip {
		rm.skipped++
		goto start
	}

	rm.readLines++
	return strings.NewReader(rm.formatFn(*line)).Read(b)
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
