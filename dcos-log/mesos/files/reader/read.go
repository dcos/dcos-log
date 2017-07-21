package reader

import (
	"net/url"
	"errors"
	"net/http"
	"context"
	"fmt"
	"bufio"
	"encoding/json"
	"strings"
	"strconv"
	"bytes"
	"io"
)


// NewLineReader is a LineReader constructor.
func NewLineReader(masterURL *url.URL, client *http.Client, file, agentID, frameworkID, executorID, containerID string,
	               h http.Header) (*LineReader, error) {
	if masterURL == nil {
		return nil, errors.New("master url cannot be nil")
	}

	return &LineReader{
		header: h,
		maxResponseSize: 1<<16,
		chunkIndex: 1,

		client: client,
		readEndpoint: &url.URL{
			Scheme: masterURL.Scheme,
			Host: masterURL.Host,
			Path: fmt.Sprintf("/agent/%s/files/read", agentID),
		},
		sandboxPath: fmt.Sprintf("/var/lib/mesos/slave/slaves/%s/frameworks/%s/executors/%s/runs/%s/",
			agentID, frameworkID, executorID, containerID),

		File: file,
		AgentID: agentID,
		FrameworkID: frameworkID,
		ExecutorID: executorID,
		ContainerID: containerID,
	}, nil
}

type response struct {
	Data   string `json:"data"`
	Offset int    `json:"deltaOffset"`
}

// LineReader implements a reader for mesos files API.
// http://mesos.apache.org/documentation/latest/endpoints/files/read/
type LineReader struct {
	currentOffset   int
	deltaOffset     int
	maxResponseSize int
	fileLength      int

	//chunks      int
	chunkIndex  int

	buf []string
	itemIndex int

	client       *http.Client
	readEndpoint *url.URL
	sandboxPath  string
	header       http.Header

	File        string
	AgentID     string
	FrameworkID string
	ExecutorID  string
	ContainerID string
}

func (r *LineReader) do(req *http.Request) (data *response, err error) {
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return
}

func (r *LineReader) fileLen(ctx context.Context, file string, header http.Header) (int, error) {
	v := url.Values{}
	v.Add("path", r.sandboxPath + file)
	v.Add("deltaOffset", "-1")
	newURL := *r.readEndpoint
	newURL.RawQuery = v.Encode()

	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return 0, err
	}

	req.Header = header

	resp, err := r.do(req.WithContext(ctx))
	if err != nil {
		return 0, err
	}

	return resp.Offset, nil
}

func (r *LineReader) read(ctx context.Context, file string, offset, length int, header http.Header) ([]string, int, int, error){
	v := url.Values{}
	v.Add("path", r.sandboxPath + file)
	v.Add("offset", strconv.Itoa(offset))
	v.Add("length", strconv.Itoa(length))

	newURL := *r.readEndpoint
	newURL.RawQuery = v.Encode()

	var short int

	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return nil, 0, 0, err
	}

	//fmt.Println(newURL.String())
	req.Header = header
	resp, err := r.do(req.WithContext(ctx))
	if err != nil {
		return nil, 0, 0, err
	}

	scanner := bufio.NewScanner(strings.NewReader(resp.Data))
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			// We have a full newline-terminated line.
			return i + 1, data[:i], nil
		}

		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			short = len(data)
			return short, nil, nil
		}
		// Request more data.
		return 0, nil, nil
	})

	var buf []string
	for scanner.Scan() {
		buf = append(buf, scanner.Text() + "\n")
	}

	return buf, len(resp.Data), short, nil
}

func (r *LineReader) Read(b []byte) (int, error) {
	var (
		err error
		offset int
		size int
	)

	if len(r.buf) == 0 {
		r.buf, size, offset, err = r.read(context.TODO(), r.File, r.currentOffset, r.maxResponseSize, r.header)
		if err != nil {
			panic(err)
			return 0, err
		}

		r.deltaOffset += offset
		r.currentOffset = (r.currentOffset + size) - r.deltaOffset
		//if len(r.buf) > 0 {
			//r.deltaOffset += offset
			//r.currentOffset += size - r.deltaOffset
		//}
	}

	if r.itemIndex < len(r.buf) {
		lineReader := strings.NewReader(r.buf[r.itemIndex])
		sz, err := lineReader.Read(b)
		r.itemIndex++

		if err == io.EOF {
			return sz, nil
		}

		if err != nil {
			return 0, err
		}
		return sz, nil
	}

	// clear buffer
	r.buf = nil
	r.itemIndex = 0
	return 0, io.EOF
}

//func (r *LineReader) ReadLines(ctx context.Context, file string, header http.Header) (chan []string, chan error) {
//	// assuming page size is 4096, files API allocate a buffer page*16
//	maxSize := 1<<16
//	linesChan := make(chan []string)
//	errChan := make(chan error)
//
//	//var deltaOffset int
//
//	go func() {
//		len, err := r.fileLen(ctx, file, header)
//		if err != nil {
//			errChan<- err
//			return
//		}
//
//		var sum int
//		for i := 0; i < (len / maxSize); i++ {
//			var lines []string
//			deltaOffset := (i * maxSize) - sum
//			lines, delta, err := r.read(ctx, file, deltaOffset, maxSize, header)
//			if err != nil {
//				errChan<- err
//				return
//			}
//
//			sum += delta
//
//			linesChan<- lines
//		}
//	}()
//
//
//	return linesChan, errChan
//}
