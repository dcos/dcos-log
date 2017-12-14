package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	chunkSize = 1 << 16
)

const (
	pathParam   = "path"
	offsetParam = "offset"
	lengthParam = "length"
)

// ReadDirection specifies the direction files API will be read.
type ReadDirection int

// BottomToTop reads files API from bottom to top.
const BottomToTop ReadDirection = 1

var (
	// ErrNoData is an error returned by Read(). It indicates that the buffer is empty
	// and we need to request more data from mesos files API.
	ErrNoData = errors.New("new data needed")

	// ErrFileNotFound is raised if the request file is not found in mesos files API.
	ErrFileNotFound = errors.New("file not found")
)

type response struct {
	Data   string `json:"data"`
	Offset int    `json:"offset"`
}

type modifier func(s string) string

func notEmpty(args map[string]string) error {
	if len(args) == 0 {
		return fmt.Errorf("parameters cannot be empty")
	}

	for name, arg := range args {
		if arg == "" {
			return fmt.Errorf("parameter %s cannot be empty", name)
		}
	}

	return nil
}

// NewLineReader is a ReadManager constructor.
func NewLineReader(client *http.Client, masterURL url.URL, agentID, frameworkID, executorID, containerID, taskPath, file string,
	format Formatter, opts ...Option) (*ReadManager, error) {

	// make sure the required parameters are set properly
	if err := notEmpty(map[string]string{"agentID": agentID, "frameworkID": frameworkID, "executorID": executorID,
		"containerID": containerID}); err != nil {
		return nil, err
	}

	sandboxPath := path.Join("/var/lib/mesos/slave/slaves", agentID, "/frameworks", frameworkID, "/executors", executorID, "/runs", containerID)
	if taskPath != "" {
		sandboxPath = path.Join(sandboxPath, path.Join("tasks", taskPath))
	}

	rm := &ReadManager{
		client: client,

		file:         file,
		readEndpoint: masterURL,
		sandboxPath:  sandboxPath,
		formatFn:     format,

		agentID:     agentID,
		frameworkID: frameworkID,
		executorID:  executorID,
		containerID: containerID,
	}

	for _, opt := range opts {
		if opt != nil {
			if err := opt(rm); err != nil {
				return nil, err
			}
		}
	}

	if rm.readDirection == BottomToTop && rm.skip != 0 {
		var (
			offset int
			length int
		)

		if rm.offset > chunkSize {
			offset = rm.offset - chunkSize
			length = chunkSize
		} else {
			// offset 0
			length = rm.offset
		}

		err := calcOffset(offset, length, rm)
		if err != nil && err != io.EOF {
			return nil, err
		}
	}

	// guard against negative offset
	if rm.offset < 0 {
		rm.offset = 0
	}

	return rm, nil
}

func calcOffset(offset, length int, rm *ReadManager) error {
	var foundLines int

	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		lines, delta, err := rm.read(ctx, offset, length, reverse)
		if err != nil {
			cancel()
			return err
		}

		cancel()

		skip := rm.skip

		// make skip a positive number
		if skip < 0 {
			skip = rm.skip * -1
		}

		// if the required number of lines found, we need to calculate an offset
		if foundLines+len(lines) >= skip {
			for i, skipped := 0, 0; skipped < skip; i++ {
				if lines[i].Message != "" {
					skipped++
				}
				rm.offset -= len(lines[i].Message) + 1
			}
			return nil
		}

		// if the current chunk contains less then requested lines, then add to a counter
		// and continue search.
		foundLines += len(lines)

		length = chunkSize
		offset -= chunkSize - delta
		rm.offset = offset

		// if the offset is 0 or negative value, the means we reached the top of the file.
		// we can just set the offset to 0 and read the entire file
		if offset < 0 {
			rm.offset = 0
			return nil
		}
	}
}

// ReadManager is a mesos files API reader. It builds the correct sandbox path to files
// and implements io.Reader.
// http://mesos.apache.org/documentation/latest/endpoints/files/read/
type ReadManager struct {
	client       *http.Client
	readEndpoint url.URL
	sandboxPath  string
	header       http.Header

	readDirection ReadDirection
	readLimit     int
	skip          int
	skipped       int
	file          string

	size   int
	offset int
	lines  []Line

	readLines int
	stream    bool

	formatFn Formatter

	agentID     string
	frameworkID string
	executorID  string
	containerID string
	taskPath    string
}

func (rm *ReadManager) do(req *http.Request) (*response, error) {
	resp, err := rm.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNotFound:
		return nil, ErrFileNotFound
	default:
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
	v.Add(pathParam, filepath.Join(rm.sandboxPath, rm.file))
	v.Add(offsetParam, "-1")
	newURL := rm.readEndpoint
	newURL.RawQuery = v.Encode()

	logrus.Debugf("fileLen %s", newURL)
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
	v.Add(pathParam, filepath.Join(rm.sandboxPath, rm.file))
	v.Add(offsetParam, strconv.Itoa(offset))
	v.Add(lengthParam, strconv.Itoa(length))

	if modifier == nil {
		modifier = func(s string) string { return s }
	}

	newURL := rm.readEndpoint
	newURL.RawQuery = v.Encode()

	logrus.Debugf("read %s", newURL)

	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return nil, 0, err
	}

	req.Header = rm.header
	resp, err := rm.do(req.WithContext(ctx))
	if err != nil {
		return nil, 0, err
	}

	if resp.Data == "" || resp.Data == "\n" {
		return nil, 0, io.EOF
	}

	lines := strings.Split(modifier(resp.Data), "\n")

	delta := 0
	if len(lines) > 1 {
		delta = len(lines[len(lines)-1])
		lines = lines[:len(lines)-1]
	}

	linesWithOffset := make([]Line, len(lines))
	// accumulates the position of the line + \readLimit
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

// Prepend the lines to a buffer.
func (rm *ReadManager) Prepend(s Line) {
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
	if !rm.stream && rm.readLimit > 0 && rm.readLines == rm.readLimit {
		return 0, io.EOF
	}

	if len(rm.lines) == 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		lines, delta, err := rm.read(ctx, rm.offset, chunkSize, nil)
		if err != nil {
			return 0, err
		}

		if len(lines) > 0 {
			linesLen := 0
			for _, line := range lines {
				rm.Prepend(line)
				linesLen += len(line.Message) + 1
			}

			if linesLen < chunkSize {
				rm.offset = rm.offset + linesLen - 1
			} else {
				rm.offset = (rm.offset + chunkSize) - delta - 1
			}
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
	return strings.NewReader(rm.formatFn(*line, rm)).Read(b)
}

// SandboxFile represents a file object located in mesos sandbox.
type SandboxFile struct {
	GID   string `json:"gid"`
	Mode  string `json:"mode"`
	MTime mTime  `json:"mtime"`
	NLink uint   `json:"nlink"`
	Path  string `json:"path"`
	Size  uint64 `json:"size"`
	UID   string `json:"uid"`
}

type mTime uint64

// mtime field has a weird format with trailing .0
// in order to unmarshal this value into json, we need to cut off the suffix .0 if it exists
func (mt *mTime) UnmarshalJSON(data []byte) error {
	sanitizedData := bytes.TrimSuffix(data, []byte(".0"))
	v, err := strconv.ParseUint(string(sanitizedData), 10, 64)
	if err != nil {
		return err
	}
	*mt = mTime(v)
	return nil
}

// BrowseSandbox returns a url to browse files in the sandbox.
func (rm ReadManager) BrowseSandbox() ([]SandboxFile, error) {
	v := url.Values{}
	v.Add(pathParam, rm.sandboxPath)

	newURL := rm.readEndpoint
	newURL.RawQuery = v.Encode()

	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header = rm.header

	resp, err := rm.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to make a GET request: %s. URL %s", err, newURL.String())
	}
	defer resp.Body.Close()

	var files []SandboxFile

	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("unable to decode files API response: %s. URL %s", err, newURL.String())
	}

	return files, nil
}

// Download makes a request to download endpoint and returns a raw http.Response for client to read and close.
func (rm ReadManager) Download() (*http.Response, error) {
	v := url.Values{}
	v.Add(pathParam, filepath.Join(rm.sandboxPath, rm.file))

	newURL := rm.readEndpoint
	newURL.RawQuery = v.Encode()

	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create a new request to %s: %s", newURL.String(), err)
	}

	req.Header = rm.header

	return rm.client.Do(req)
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
