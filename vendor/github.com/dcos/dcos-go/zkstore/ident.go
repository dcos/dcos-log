package zkstore

import (
	"fmt"

	"github.com/pkg/errors"
)

// Version represents an optional ZK version number.
type Version struct {
	int32
	hasVersion bool
}

// Value returns true with a real ZK version number, false when there is no value.
func (v Version) Value() (int32, bool) { return v.int32, v.hasVersion }

// Clear erases the real ZK version number. Future calls to Version will return false.
func (v *Version) Clear() { v.hasVersion = false }

// NewVersion returns a ZK version w/ the given value. Future calls to Version will return true.
func NewVersion(v int32) Version { return Version{hasVersion: true, int32: v} }

// Ident specifies the location of a stored item
type Ident struct {
	// Location points to where an item lives in the store.
	Location

	// Variant, if specified, specifies a named version of the data.
	Variant string

	// Version specifies the ZK version of the data.  This will be
	// used to prevent accidental overwrites.  If Version is nil,
	// then the version will not be considered for operations.
	Version Version
}

func (i Ident) String() string {
	return fmt.Sprintf("{loc=%v var=%v ver=%v}", i.Location, i.Variant, i.Version)
}

// Validate performs validation on the Ident
func (i Ident) Validate() error {
	if err := i.Location.Validate(); err != nil {
		return errors.Wrap(err, "invalid location")
	}
	if err := ValidateNamed(i.Variant, false); err != nil {
		return errors.Wrap(err, "invalid variant")
	}
	return nil
}

// actualVersion converts the *int32 into a version recognized by ZK.  ZK
// expects a -1 if the version should be ignored.
func (i Ident) actualVersion() int32 {
	v, ok := i.Version.Value()
	if !ok {
		return -1
	}
	return v
}
