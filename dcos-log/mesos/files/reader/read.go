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
)

const (
	chunkSize = 4096
)

type response struct {
	Data   string `json:"data"`
	Offset int    `json:"offset"`
}

var ErrEmptyParam = errors.New("args cannot be empty")

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
//func NewLineReader(masterURL *url.URL, client *http.Client, file, agentID, frameworkID, executorID, containerID string,
//	h http.Header, n int) (*ReadManager, error) {
func NewLineReader(client *http.Client, masterURL *url.URL, agentID, frameworkID, executorID, containerID string, opts ...Option) (*ReadManager, error) {

	// make sure the required parameters are set properly
	if err := notEmpty([]string{agentID, frameworkID, executorID, containerID}); err != nil {
		return nil, err
	}

	masterURLCopy := *masterURL
	masterURLCopy.Path = fmt.Sprintf("/agent/%s/files/read", agentID)

	rm := &ReadManager{
		client: client,

		// use default `stdout` file
		File:         "stdout",
		readEndpoint: &masterURLCopy,
		sandboxPath: fmt.Sprintf("/var/lib/mesos/slave/slaves/%s/frameworks/%s/executors/%s/runs/%s/",
			agentID, frameworkID, executorID, containerID),
	}

	for _, opt := range opts {
		if opt != nil {
			if err := opt(rm); err != nil {
				return nil, err
			}
		}
	}

	if rm.readDirection == BottomToTop {
		size, err := rm.fileLen(context.TODO())
		if err != nil {
			return nil, err
		}

		rm.offset = size
		foundLines := 0
		offset := size - chunkSize
		for {
			// if the offset is 0 or negative value, the means we reached the top of the file.
			// we can just set the offset to 0 and read the entire file
			if offset < 1 {
				rm.offset = 0
				break
			}

			lines, delta, err := rm.read(context.TODO(), offset, chunkSize, false, reverse)
			if err != nil {
				return nil, err
			}

			// if the required number of lines found, we need to calculate an offset
			if foundLines+len(lines) >= rm.n {
				diff := rm.n - foundLines
				for i := len(lines) - diff; i < len(lines); i++ {
					rm.offset -= len(lines[i]) + 1
				}
				break
			} else {
				// if the current chunk contains less then requested lines, then add to a counter
				// and continue search.
				foundLines += len(lines)
			}

			offset -= chunkSize - delta + 7
			rm.offset = offset
		}
	}

	return rm, nil
}

type ReadDirection int

const (
	TopToBottom ReadDirection = iota
	BottomToTop
)

// ReadManager ...
// http://mesos.apache.org/documentation/latest/endpoints/files/read/
type ReadManager struct {
	client       *http.Client
	readEndpoint *url.URL
	sandboxPath  string
	header       http.Header

	readDirection ReadDirection
	n      int
	File string

	size int
	offset int
	lines []string

	readLines int
	ignoreDelta bool
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
	newURL := *rm.readEndpoint
	newURL.RawQuery = v.Encode()

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

type Callback func(l *Line)
type Modifier func(s string) string

func (rm *ReadManager) read(ctx context.Context, offset, length int, ignoreDelta bool, modifier Modifier) ([]string, int, error) {
	v := url.Values{}
	v.Add("path", rm.sandboxPath+rm.File)
	v.Add("offset", strconv.Itoa(offset))
	v.Add("length", strconv.Itoa(length))

	if modifier == nil {
		modifier = func(s string) string { return s }
	}

	newURL := *rm.readEndpoint
	newURL.RawQuery = v.Encode()

	//fmt.Println(newURL.String())

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

	if ignoreDelta {
		return lines, 0, nil
	}

	delta := 0
	if len(lines) > 2 {
		delta = len(lines[len(lines)-1])
		lines = lines[:len(lines)-1]
	}

	return lines, delta, nil
}

func (r *ReadManager) Prepand(s string) {
	old := r.lines
	r.lines = append([]string{s}, old...)
}

func (r *ReadManager) Pop() string {
	old := *r
	n := len(old.lines)
	if n == 0 {
		return ""
	}

	x := old.lines[n-1]
	r.lines = old.lines[0 : n-1]
	return x
}

func (r *ReadManager) Read(b []byte) (int, error) {
	if r.n > 0 && r.readLines == r.n {
		return 0, io.EOF
	}

	if len(r.lines) == 0 {
		lines, delta, err := r.read(context.TODO(), r.offset, chunkSize, r.ignoreDelta, nil)
		if err != nil {
			return 0, err
		}

		if len(lines) > 1 {
			linesLen := 0
			for _, line := range lines {
				r.Prepand(line)
				linesLen += len(line) + 1
			}

			if linesLen < chunkSize {
				r.offset = r.offset + linesLen - 1
				r.ignoreDelta = true
			} else {
				r.offset = (r.offset + chunkSize) - delta
			}
		}
	}

	line := r.Pop()
	if line == "" {
		return 0, io.EOF
	}

	r.readLines++
	return strings.NewReader(line + "\n").Read(b)
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
