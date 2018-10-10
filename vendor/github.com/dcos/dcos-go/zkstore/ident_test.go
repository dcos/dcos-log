package zkstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIdentValidate(t *testing.T) {
	type testCase struct {
		ident  Ident
		errMsg string
	}
	for _, test := range []testCase{
		{
			ident:  Ident{},
			errMsg: "invalid location: invalid name: cannot be blank",
		},
		{
			ident:  Ident{Location: Location{Name: "foo"}},
			errMsg: "invalid location: invalid category: " + errBadCategory.Error(),
		},
		{
			ident:  Ident{Location: Location{Name: "foo/bar"}},
			errMsg: "invalid location: invalid name: must match " + validNameRE.String(),
		},
		{
			ident:  Ident{Location: Location{Name: "foo", Category: "widgets"}, Variant: "invalid/version"},
			errMsg: "invalid variant: must match " + validNameRE.String(),
		},
		{
			ident: Ident{Location: Location{Name: "foo", Category: "widgets"}, Variant: "my-version"},
		},
		{
			ident: Ident{Location: Location{Name: "foo", Category: "widgets/2017"}, Variant: "my-version"},
		},
	} {
		err := test.ident.Validate()
		if errMsg(err) != test.errMsg {
			t.Fatalf("%v err:%v", test, err)
		}
	}
}

func TestActualVersion(t *testing.T) {
	require := require.New(t)

	ident := Ident{}
	require.EqualValues(-1, ident.actualVersion())
	require.False(ident.Version.hasVersion)

	ident.Version = NewVersion(0)
	require.EqualValues(0, ident.actualVersion())
	require.True(ident.Version.hasVersion)

	ident.Version = NewVersion(1)
	require.EqualValues(1, ident.actualVersion())
	require.True(ident.Version.hasVersion)

	ident.Version = NewVersion(-1)
	require.EqualValues(-1, ident.actualVersion())
	require.True(ident.Version.hasVersion)
}
