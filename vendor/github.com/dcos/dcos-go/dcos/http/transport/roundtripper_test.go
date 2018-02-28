// Copyright 2016 Mesosphere, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

var signedToken = "1234567890"

type fakeRoundTripper struct {
	reqHandler func(*http.Request) (*http.Response, error)
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f.reqHandler(req)
}

func bouncerToken(w http.ResponseWriter, r *http.Request) {
	token := struct {
		T string `json:"token"`
	}{
		T: signedToken,
	}

	b, _ := json.Marshal(token)

	fmt.Fprint(w, string(b))
}

// Test if we can generate token and add it to request headers.
func TestNewRoundTripper(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(bouncerToken))
	defer ts.Close()

	fr := &fakeRoundTripper{
		func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "http://127.0.0.1:8101/acs/api/v1/auth/login" {
				// validate the content-type
				if contentType := req.Header.Get("Content-type"); contentType != "application/json" {
					t.Fatalf("Expect request header `Content-type: application/json. Got: %s", contentType)
				}

				// validate POST params
				postParams := struct {
					UID   string `json:"uid"`
					Token string `json:"token"`
					Exp   int64  `json:"exp"`
				}{}

				decoder := json.NewDecoder(req.Body)
				if err := decoder.Decode(&postParams); err != nil {
					t.Fatal(err)
				}

				if postParams.UID != "test_user" {
					t.Fatalf("Expect POST parameter `uid` = test_user. Got: %s", postParams.UID)
				}

				if postParams.Token == "" {
					t.Fatal("Expect POST parameter `token`")
				}

				if postParams.Exp < 1 {
					t.Fatalf("Expect POST parameter `exp` positive more then zero. Got: %d", postParams.Exp)
				}

				return http.Get(ts.URL)
			}
			return http.DefaultTransport.RoundTrip(req)
		},
	}

	jwtTransport, err := NewRoundTripper(fr, OptionReadIAMConfig("./fixtures/test_service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	debug, err := DebugTransport(jwtTransport)
	if err != nil {
		t.Fatalf("DebugTransport detected incorrect transport: %s", err)
	}
	if debug.CurrentToken() != signedToken {
		t.Fatalf("Expect token %s. Got %s", signedToken, debug.CurrentToken())
	}

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// expect Authorization header with token
		auth := r.Header.Get("Authorization")
		if auth != "token="+signedToken {
			t.Fatalf("Expect request header: `Authorization: token=%s`.Got: %s", signedToken, auth)
		}
	}))
	defer ts2.Close()

	c := http.Client{
		Transport: jwtTransport,
	}

	resp, err := c.Get(ts2.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
}

// Test if we can regenerate token and retry http request if the first time we get 401.
func TestTokenUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(bouncerToken))
	defer ts.Close()

	fr := &fakeRoundTripper{
		func(req *http.Request) (*http.Response, error) {
			if req.URL.String() == "http://127.0.0.1:8101/acs/api/v1/auth/login" {
				return http.Get(ts.URL)
			}
			return http.DefaultTransport.RoundTrip(req)
		},
	}

	jwtTransport, err := NewRoundTripper(fr, OptionReadIAMConfig("./fixtures/test_service_account.json"))
	if err != nil {
		t.Fatal(err)
	}

	var cnt int32
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&cnt, 1)
		if cnt == 2 {
			return
		}
		http.Error(w, "", http.StatusUnauthorized)
	}))
	defer ts2.Close()

	c := http.Client{
		Transport: jwtTransport,
	}

	resp, err := c.Get(ts2.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expect http status 200. Got %d", resp.StatusCode)
	}
}

func TestWrongTransport(t *testing.T) {
	tr := &http.Transport{}
	// Expect to get incorrect transport type since we're debugging
	// for a implWithJWT type transport.
	if _, err := DebugTransport(tr); err != ErrWrongRoundTripperImpl {
		t.Fatalf("Expect: %s. Got %s", ErrWrongRoundTripperImpl, err)
	}
}

func TestOptionCredentials(t *testing.T) {
	_, err := NewRoundTripper(&http.Transport{}, OptionCredentials("", "", ""))
	if err != ErrInvalidCredentials {
		t.Fatalf("Expect: %s. Got %s", ErrInvalidCredentials, err)
	}
}

func TestOptionTokenExpire(t *testing.T) {
	_, err := NewRoundTripper(&http.Transport{}, OptionTokenExpire(0))
	if err != ErrInvalidExpireDuration {
		t.Fatalf("Expect: %s. Got %s", ErrInvalidExpireDuration, err)
	}
}
