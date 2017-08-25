package reader

import (
	"net/url"
	"errors"
	"net/http"
	"context"
	"bufio"
	"encoding/json"
	"strings"
	"strconv"
	"io"
	"fmt"
)


type response struct {
	Data   string    `json:"data"`
	Offset int       `json:"offset"`
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
		File: "stdout",
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

	return rm, nil
}

//	&ReadManager{
//		header: h,
//		n: n,
//
//		client: client,
//		readEndpoint: &url.URL{
//			Scheme: masterURL.Scheme,
//			Host: masterURL.Host,
//			Path: fmt.Sprintf("/agent/%s/files/read", agentID),
//		},
//		sandboxPath: fmt.Sprintf("/var/lib/mesos/slave/slaves/%s/frameworks/%s/executors/%s/runs/%s/",
//			agentID, frameworkID, executorID, containerID),
//
//		File: file,
//		AgentID: agentID,
//		FrameworkID: frameworkID,
//		ExecutorID: executorID,
//		ContainerID: containerID,
//	}, nil
//}


// ReadManager ...
// http://mesos.apache.org/documentation/latest/endpoints/files/read/
type ReadManager struct {
	client       *http.Client
	readEndpoint *url.URL
	sandboxPath  string
	header       http.Header

	offset      uint64
	n           int

	File        string

	//AgentID     string
	//FrameworkID string
	//ExecutorID  string
	//ContainerID string
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
		fmt.Println(resp.Body)
		return nil, err
	}

	return data, nil
}

func (rm *ReadManager) fileLen(ctx context.Context) (int, error) {
	v := url.Values{}
	v.Add("path", rm.sandboxPath + rm.File)
	v.Add("offset", "-1")
	newURL := *rm.readEndpoint
	newURL.RawQuery = v.Encode()

	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return 0, err
	}
	//fmt.Println(newURL.String())

	req.Header = rm.header

	resp, err := rm.do(req.WithContext(ctx))
	if err != nil {
		return 0, err
	}

	return resp.Offset, nil
}

type Callback func(l *Line)
type Modifier func(s string) string

func (rm *ReadManager) read(ctx context.Context, offset, length int, modifier Modifier, cbk Callback) error {
	v := url.Values{}
	v.Add("path", rm.sandboxPath + rm.File)
	v.Add("offset", strconv.Itoa(offset))
	v.Add("length", strconv.Itoa(length))

	newURL := *rm.readEndpoint
	newURL.RawQuery = v.Encode()

	//var short int

	req, err := http.NewRequest("GET", newURL.String(), nil)
	if err != nil {
		return err
	}

	fmt.Println(newURL.String())
	req.Header = rm.header
	resp, err := rm.do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	var pos int
	scanner := bufio.NewScanner(strings.NewReader(modifier(resp.Data)))
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = bufio.ScanLines(data, atEOF)
		pos += advance
		return
	})

	for scanner.Scan() {
		text := scanner.Text()
		cbk(&Line{
			Message:text,
			Offset: pos - len(text) - 1 + resp.Offset,
		})
	}

	return nil
}

type counter struct {
	items []int
}

func (c *counter) increment(l *Line) {
	c.items = append(c.items, l.Offset)
}

func (c *counter) len() int {
	return len(c.items)
}

func (rm *ReadManager) Range(ctx context.Context, lines int) (io.Reader, <-chan struct{}, error) {
	q := &LinesReader{
		nLines: lines,
	}
	done := make(chan struct{})

	var chunk = 4096

	size, err := rm.fileLen(ctx)
	if err != nil {
		return nil, nil, err
	}
	//fmt.Printf("size is %d\n", size)

	cnt := &counter{}

	//fmt.Println("START find offset")
	var attempt int = 1
	for cnt.len() < lines {
		//fmt.Printf("called: %d\n", cnt.len())
		var offset int
		if size > chunk {
			offset = size-(chunk*attempt)
			if offset < 0 {
				offset = 0
			}
		}

		err = rm.read(ctx, offset, chunk, reverse, cnt.increment)
		if err != nil {
			return nil, nil, err
		}
		attempt++
	}
	//fmt.Printf("DONE find offset, total attempts: %d\n", attempt)

	startOffset := cnt.items[cnt.len()-(lines+1)]
	//fmt.Printf("startOffset: %d\n", startOffset)

	//go func() {
		//defer close(done)
	var offset = startOffset
		//for i := 1; i < attempt; i ++ {

			fn := func(s string) string {
				return s
			}

			err = rm.read(ctx, offset, -1, fn, q.Prepand)
				if err != nil {
					return nil, nil, err
				}
			offset += chunk
		//}
	//}()

	//fmt.Printf("%+v\n", cnt)
	return q, done, nil
}

func (rm *ReadManager) Stream(ctx context.Context) io.Reader {
	return nil
}

func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// func (rm *ReadManager) Read(b []byte) (int, error) {
	//l, err := rm.fileLen(context.TODO())
	//if err != nil {
	//	panic(err)
	//}
	//
	//// if file is empty, return EOF
	//if l == 0 {
	//	return 0, io.EOF
	//}
	//
	//// file did not change, return EOF
	//if l == rm.offset {
	//	return 0, io.EOF
	//}

	//var chunkSize uint64 = 4096
	//lines, _, err := rm.read(context.TODO(), rm.offset, chunkSize)
	//if err != nil {
	//	panic(err)
	//}
	//
	//if rm.n > len(lines) {
	//		lines = lines[:rm.n]
	//} else {
	//	newOffset := lines[len(lines)-1].Offset
	//	if newOffset > rm.offset {
	//		rm.offset = newOffset
	//	}
	//}
	//
	//body, err := json.Marshal(lines)
	//if err != nil {
	//	panic(err)
	//}

	// return bytes.NewReader(body).Read(b)
// }

//func (rm *ReadManager) ReadLines(ctx context.Context, file string, header http.Header) {
//	// assuming page size is 4096, files API allocate a buffer page*16
//	maxSize := 1<<16
//
//	//var deltaOffset int
//
//
//	len, err := rm.fileLen(ctx, file, header)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Printf("Len= %d\n", len)
//	if len < maxSize {
//		lines, _, _, err := rm.read(ctx, file, 0, len, header)
//		if err != nil {
//			panic(err)
//		}
//		fmt.Println(lines)
//		return
//	}
//
//	//var sum int
//	//for i := 0; i < (len / maxSize); i++ {
//	//	var lines []string
//	//	deltaOffset := (i * maxSize) - sum
//	//	lines, _, delta, err := rm.read(ctx, file, deltaOffset, maxSize, header)
//	//	if err != nil {
//	//		panic(err)
//	//	}
//	//	sum += delta
//	//
//	//	fmt.Println(lines)
//	//}
//	//fmt.Println("After")
//}
