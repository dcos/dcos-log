package api

import (
	"testing"
	"net/http"
)

func TestParseUint64(t *testing.T) {

	testValues := []struct {
		input    string
		actual   uint64
		negative bool
		errorOk  bool
	} {
		{
			input: "123456789",
			actual: 123456789,
			negative: false,
		},
		{
			input: "-123456789",
			actual: 123456789,
			negative: true,
		},
		{
			// max uint64
			input: "18446744073709551615",
			actual: 18446744073709551615,
			negative: false,
		},
		{
			// max uint64 + 1
			input: "18446744073709551616",
			errorOk: true,
		},
		{
			input: "0",
		},
		{
			input: "",
			errorOk: true,
		},
	}

	for _, testValue := range testValues {
		negative, n, err := parseUint64(testValue.input)
		if testValue.errorOk {
			if err == nil {
				t.Fatalf("Expecting error on input %s but no errors", testValue.input)
			}
			// if error is ok, do no check the rest of the values
			continue
		}

		if !testValue.errorOk && err != nil {
			t.Fatalf("test value %s: %s", testValue.input, err)
		}

		if negative != testValue.negative {
			t.Fatalf("Input value: %s must not be negative", testValue.input)
		}

		if testValue.actual != n {
			t.Fatalf("Input value: %s must return %d. Got %d", testValue.input, testValue.actual, n)
		}
	}
}
