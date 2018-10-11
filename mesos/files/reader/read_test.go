package reader

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

var (
	data = []byte(`one
two
three
four
five
`)
)

func createHandler(data []byte, forceSendResponse bool, t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if !forceSendResponse {
			io.Copy(w, bytes.NewReader(data))
			return
		}

		d := data
		// check the correct offset
		offsetStr := r.URL.Query().Get("offset")
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			t.Fatal(err)
		}

		// handle the file len request
		if offset == -1 {
			offset = len(data)
			d = []byte{}
		} else if offset >= len(data) {
			d = []byte{}
		} else {
			d = data[offset:]
		}

		resp := &response{
			Data:   string(d),
			Offset: offset,
		}

		marshaledResp, err := json.Marshal(resp)
		if err != nil {
			t.Fatal(err)
		}

		io.Copy(w, bytes.NewReader(marshaledResp))

	}
}

func doRead(t *testing.T, data []byte, opts ...Option) []byte {
	ts := httptest.NewServer(createHandler(data, true, t))
	defer ts.Close()

	client := &http.Client{}

	masterURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewLineReader(client, *masterURL, "1", "2", "3", "4", "",
		"stdout", LineFormat, opts...)
	if err != nil {
		t.Fatal(err)
	}

	buf, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	return buf
}

func TestRangeRead(t *testing.T) {
	buf := doRead(t, data)
	if bytes.Compare(buf, data) != 0 {
		t.Fatalf("expect %s. Got %s", data, buf)
	}
}

func TestSkip(t *testing.T) {
	expectedResponse := []byte(`two
three
four
five
`)
	buf := doRead(t, data, OptSkip(1))
	if bytes.Compare(buf, expectedResponse) != 0 {
		t.Fatalf("expect response %s. Got %s", expectedResponse, buf)
	}
}

func TestLast2Lines(t *testing.T) {
	expectedResponse := []byte(`four
five
`)

	buf := doRead(t, data, OptReadFromEnd(), OptSkip(-2), OptReadDirection(BottomToTop))
	if bytes.Compare(buf, expectedResponse) != 0 {
		t.Fatalf("expect response %s. Got %s", expectedResponse, buf)
	}
}

func TestLimit(t *testing.T) {
	expectedResponse := []byte(`one
two
`)
	buf := doRead(t, data, OptLines(2))
	if bytes.Compare(buf, expectedResponse) != 0 {
		t.Fatalf("expect %s. Got %s", expectedResponse, buf)
	}
}

func TestCursor(t *testing.T) {
	expectedResponse := []byte(`four
five
`)
	buf := doRead(t, data, OptOffset(14))
	if bytes.Compare(buf, expectedResponse) != 0 {
		t.Fatalf("expect %s. Got %s", expectedResponse, buf)
	}
}

func TestLimitAndSkip(t *testing.T) {
	expectedResponse := []byte(`three
four
`)

	buf := doRead(t, data, OptSkip(2), OptLines(2))
	if bytes.Compare(buf, expectedResponse) != 0 {
		t.Fatalf("expect %s. Got %s", expectedResponse, buf)
	}
}

func TestOneLine(t *testing.T) {
	testData := []byte(`hello
`)

	buf := doRead(t, testData)
	if bytes.Compare(buf, testData) != 0 {
		t.Fatalf("expect %s. Got %s", testData, buf)
	}
}

func TestEmptyData(t *testing.T) {
	testData := []byte("")

	buf := doRead(t, testData)
	if len(buf) > 0 {
		t.Fatalf("must be empty. Got %s", buf)
	}

	testData = []byte("\n")
	buf = doRead(t, testData)
	if len(buf) > 0 {
		t.Fatalf("must be empty. Got %s", buf)
	}
}

func TestBrowseSandbox(t *testing.T) {
	sandboxResponse := []byte(`[{
"gid":"root",
"mode":"-rw-r--r--",
"mtime":1513020278.0,
"nlink":1,
"path":"\/var\/lib\/mesos\/slave\/slaves\/f7df47cd-c82b-470d-b84b-4fc8611a3976-S1\/frameworks\/f7df47cd-c82b-470d-b84b-4fc8611a3976-0001\/executors\/instance-parent-pod.ed021ce3-dea8-11e7-9679-bef2db080897\/runs\/34ddc098-3310-4a9b-9f07-3d42e4c334bc\/tasks\/parent-pod.instance-ed021ce3-dea8-11e7-9679-bef2db080897.container-1\/one",
"size":10,
"uid":"root"
}]`)

	ts := httptest.NewServer(createHandler(sandboxResponse, false, t))
	defer ts.Close()

	client := http.DefaultClient

	masterURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewLineReader(client, *masterURL, "1", "2", "3", "4", "",
		"stdout", LineFormat)
	if err != nil {
		t.Fatal(err)
	}

	files, err := r.BrowseSandbox()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expecting one item. Got %d", len(files))
	}

	item := files[0]
	if item.MTime != 1513020278 {
		t.Fatalf("expect mtime 1513020278. Got %d", item.MTime)
	}

	expectedPath := "/var/lib/mesos/slave/slaves/f7df47cd-c82b-470d-b84b-4fc8611a3976-S1/frameworks/f7df47cd-c82b-470d-b84b-4fc8611a3976-0001/executors/instance-parent-pod.ed021ce3-dea8-11e7-9679-bef2db080897/runs/34ddc098-3310-4a9b-9f07-3d42e4c334bc/tasks/parent-pod.instance-ed021ce3-dea8-11e7-9679-bef2db080897.container-1/one"
	if item.Path != expectedPath {
		t.Fatalf("expect path %s. Got %s", expectedPath, item.Path)
	}

	if item.GID != "root" {
		t.Fatalf("expect gid root. Got %s", item.GID)
	}

	if item.Mode != "-rw-r--r--" {
		t.Fatalf("invalid mode %s", item.Mode)
	}

	if item.Size != 10 {
		t.Fatalf("invalid size %d", item.Size)
	}
}

func TestDownload(t *testing.T) {
	body := []byte("one two three")
	ts := httptest.NewServer(createHandler(body, false, t))
	defer ts.Close()

	client := &http.Client{}

	masterURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewLineReader(client, *masterURL, "1", "2", "3", "4", "",
		"stdout", LineFormat)
	if err != nil {
		t.Fatal(err)
	}

	dl, err := r.Download()
	if err != nil {
		t.Fatal(err)
	}
	defer dl.Body.Close()

	buf, err := ioutil.ReadAll(dl.Body)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(body, buf) != 0 {
		t.Fatalf("expect %s. Got %s", body, buf)
	}
}

func TestHeaderSet(t *testing.T) {
	h := http.Header{}
	h.Add("foo", "bar")

	opts := []Option{OptHeaders(h)}

	r, err := NewLineReader(http.DefaultClient, url.URL{}, "1", "2", "3", "4",
		"", "stdout", LineFormat, opts...)
	if err != nil {
		t.Fatal(err)
	}

	if r.header.Get("foo") != "bar" {
		t.Fatalf("expected header foo with value bar. Got %+v", r.header)
	}
}

func TestSkipBoundary(t *testing.T) {
	// Test the values from -100 to 100 are acceptable and not causing panic
	for i := -100; i < 100; i++ {
		doRead(t, data, OptReadDirection(BottomToTop), OptSkip(i))
	}
}
