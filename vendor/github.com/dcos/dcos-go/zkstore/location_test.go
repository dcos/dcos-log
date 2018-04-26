package zkstore

import (
	"testing"
)

func TestLocationValidate(t *testing.T) {
	type testCase struct {
		location Location
		errMsg   string
	}
	for _, test := range []testCase{
		{
			location: Location{},
			errMsg:   "invalid name: cannot be blank",
		},
		{
			location: Location{Name: "foo/bar"},
			errMsg:   "invalid name: must match " + validNameRE.String(),
		},
		{
			location: Location{Name: "foo"},
			errMsg:   "invalid category: " + errBadCategory.Error(),
		},
		{
			location: Location{Name: "foo", Category: "invalid category"},
			errMsg:   "invalid category: must match " + validCategoryRE.String(),
		},
		{
			location: Location{Name: "foo", Category: "invalid//category"},
			errMsg:   "invalid category: " + errBadCategory.Error(),
		},
		{
			location: Location{Name: "foo", Category: "/my-category"},
		},
		{
			location: Location{Name: "foo", Category: "simple-category"},
		},
	} {
		err := test.location.Validate()
		if errMsg(err) != test.errMsg {
			t.Fatalf("%#v: err:%v", test, err)
		}
	}
}
