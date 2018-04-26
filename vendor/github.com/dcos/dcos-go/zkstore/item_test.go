package zkstore

import (
	"testing"
)

func TestItemValidate(t *testing.T) {
	type testCase struct {
		item   Item
		errMsg string
	}
	for _, test := range []testCase{
		{
			item:   Item{},
			errMsg: "invalid location: invalid name: cannot be blank",
		},
		{
			item:   Item{Ident: Ident{Location: Location{Name: "foo"}}},
			errMsg: "invalid location: invalid category: " + errBadCategory.Error(),
		},
		{
			item: Item{Ident: Ident{Location: Location{Name: "foo", Category: "widgets"}}},
		},
		{
			item: Item{Ident: Ident{Location: Location{Name: "foo", Category: "widgets"}},
				Data: make([]byte, 1024)},
		},
		{
			item: Item{Ident: Ident{Location: Location{Name: "foo", Category: "widgets"}},
				Data: make([]byte, 1024*1024)},
		},
		{
			item: Item{Ident: Ident{Location: Location{Name: "foo", Category: "widgets"}},
				Data: make([]byte, 1024*1024+1)},
			errMsg: "data is greater than 1MB",
		},
	} {
		err := test.item.Validate()
		if errMsg(err) != test.errMsg {
			t.Fatalf("%v err:%v", test.item, err)
		}
	}
}
