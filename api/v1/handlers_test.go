package v1

import (
	"net/http"
	"testing"
)

func TestGetCursor(t *testing.T) {
	req, err := http.NewRequest("GET", "/?cursor=s%3Dcea8150abb0543deaab113ed2f39b014%3Bi%3D1%3Bb%3D2c357020b6e54863a5ac9dee71d5872c%3Bm%3D33ae8a1%3Bt%3D53e52ec99a798%3Bx%3Db3fe26128f768a49", nil)
	if err != nil {
		t.Fatal(err)
	}

	c, err := getCursor(req)
	if err != nil {
		t.Fatal(err)
	}

	if c != "s=cea8150abb0543deaab113ed2f39b014;i=1;b=2c357020b6e54863a5ac9dee71d5872c;m=33ae8a1;t=53e52ec99a798;x=b3fe26128f768a49" {
		t.Fatalf("Expecting cursor 123. Got: %s", c)
	}
}

func TestGetLimit(t *testing.T) {
	limits := []struct {
		uri     string
		expect  uint64
		stream  bool
		errorOk bool
	}{
		{
			uri:    "/?limit=10",
			expect: 10,
		},
		{
			uri:     "/?limit=-10",
			errorOk: true,
		},
		{
			uri: "?limit=0",
		},
	}

	for _, limit := range limits {
		r, err := http.NewRequest("GET", limit.uri, nil)
		if err != nil {
			t.Fatal(err)
		}
		l, err := getLimit(r, limit.stream)
		if limit.errorOk {
			if err == nil {
				t.Fatalf("Expecting error on input %s but no errors", limit.uri)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}

		if l != limit.expect {
			t.Fatalf("Expecting %d. Got %d", limit.expect, l)
		}
	}
}

func TestGetSkip(t *testing.T) {
	skipValues := []struct {
		uri                string
		skipNext, skipPrev uint64
		errorOk            bool
	}{
		{
			uri:      "/?skip_next=10",
			skipNext: 10,
		},
		{
			uri:      "/?skip_prev=10",
			skipPrev: 10,
		},
		{
			uri: "/?skip_next=0",
		},
		{
			uri: "/?skip_prev=0",
		},
		{
			// max uint64 + 1
			uri:     "/?skip_next=18446744073709551616",
			errorOk: true,
		},
		{
			// max uint64 + 1
			uri:     "/?skip_prev=18446744073709551616",
			errorOk: true,
		},
	}

	for _, skip := range skipValues {
		r, err := http.NewRequest("GET", skip.uri, nil)
		if err != nil {
			t.Fatal(err)
		}

		skipNext, skipPrev, err := getSkip(r)
		if skip.errorOk {
			if err == nil {
				t.Fatalf("Expecting error on input %s but no errors", skip.uri)
			}
			continue
		}

		if err != nil {
			t.Fatal(err)
		}

		if skipNext != skip.skipNext {
			t.Fatalf("Expecting skipNext %d. Got %d", skip.skipNext, skipNext)
		}

		if skipPrev != skip.skipPrev {
			t.Fatalf("Expecting skipPrev %d. Got %d", skip.skipPrev, skipPrev)
		}
	}
}

func TestGetMatches(t *testing.T) {
	r, err := http.NewRequest("GET", "?filter=hello:world&filter=foo:bar", nil)
	if err != nil {
		t.Fatal(err)
	}

	matches, err := getMatches(r)
	if err != nil {
		t.Fatal(err)
	}

	if len(matches) != 2 {
		t.Fatalf("Must have 2 matches got %d", len(matches))
	}

	if matches[0].Field != "HELLO" || matches[0].Value != "world" {
		t.Fatalf("Expecting HELLO=world match. Got %+v", matches[0])
	}

	if matches[1].Field != "FOO" || matches[1].Value != "bar" {
		t.Fatalf("Expecting FOO=bar match. Got %+v", matches[1])
	}
}
