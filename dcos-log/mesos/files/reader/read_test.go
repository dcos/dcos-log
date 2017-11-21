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

func createHandler(data []byte, t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

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
	ts := httptest.NewServer(createHandler(data, t))
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
