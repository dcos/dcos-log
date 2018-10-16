package v2

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/dcos/dcos-log/mesos/files/reader"
)

type filesAPIResponse struct {
	Offset int    `json:"offset"`
	Data   string `json:"data"`
}

func newFakeFilesAPIServer(t *testing.T) *httptest.Server {
	serverResponse := "one\ntwo\nthree\nfour\nfive\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		data := serverResponse
		offsetStr := r.URL.Query().Get("offset")
		if offsetStr != "" {
			offset, err := strconv.Atoi(offsetStr)
			if err != nil {
				t.Fatal(err)
			}

			if offset > 0 {
				data = data[offset:]
			}
		}
		body, err := json.Marshal(&filesAPIResponse{
			Offset: 0,
			Data:   data,
		})
		if err != nil {
			t.Fatal(err)
		}

		fmt.Fprint(w, string(body))
	}))
	return ts
}

func makeRequest(req *http.Request, t *testing.T) string {
	opts, err := buildOpts(req)
	if err != nil {
		t.Fatal(err)
	}

	ts := newFakeFilesAPIServer(t)
	defer ts.Close()

	testURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	r, err := reader.NewLineReader(&http.Client{}, *testURL, "a", "b", "c", "d", "f",
		"stdout", reader.LineFormat, opts...)
	if err != nil {
		t.Fatal(err)
	}

	body, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}

func TestBuildOpts(t *testing.T) {
	req, err := http.NewRequest("GET", "/?limit=2&skip=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedResponse := "two\nthree\n"
	resp := makeRequest(req, t)
	if resp != expectedResponse {
		t.Fatalf("expect %s. Got %s", expectedResponse, resp)
	}
}

func TestBuildOptsWithLastEventID(t *testing.T) {
	req, err := http.NewRequest("GET", "/?limit=2&skip=1&cursor=10", nil)
	if err != nil {
		t.Fatal(err)
	}

	// 18 offset stands for "five\n"
	req.Header.Set("Last-Event-ID", "18")

	expectedResponse := "five\n"
	resp := makeRequest(req, t)
	if resp != expectedResponse {
		t.Fatalf("expect %s. Got %s", expectedResponse, resp)
	}
}

func TestBuildOptsCursor(t *testing.T) {
	// cursor 18 stands for the last line "five\n"
	req, err := http.NewRequest("GET", "/?cursor=18", nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedResponse := "five\n"
	resp := makeRequest(req, t)
	if resp != expectedResponse {
		t.Fatalf("expect %s. Got %s", expectedResponse, resp)
	}
}

func TestBuildOptsLimit(t *testing.T) {
	req, err := http.NewRequest("GET", "/?limit=2", nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedResponse := "one\ntwo\n"
	resp := makeRequest(req, t)
	if resp != expectedResponse {
		t.Fatalf("expect %s. Got %s", expectedResponse, resp)
	}
}

func TestBuildOptsSkip(t *testing.T) {
	req, err := http.NewRequest("GET", "/?skip=3", nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedResponse := "four\nfive\n"
	resp := makeRequest(req, t)
	if resp != expectedResponse {
		t.Fatalf("expect %s. Got %s", expectedResponse, resp)
	}
}
