package zkstore

import (
	"fmt"

	"github.com/pkg/errors"
)

// Location points to a particular item in the store
type Location struct {
	// Category is a base path for a particular category of data. For example,
	// this might be something like "widgets" or "widgets/2017" etc. This
	// field is required.
	Category string

	// Name is the name of the stored data. This field is required.
	Name string
}

func (l Location) String() string {
	return fmt.Sprintf("{cat=%v name=%v}", l.Category, l.Name)
}

// Validate performs validation on the Location
func (l Location) Validate() error {
	if err := ValidateNamed(l.Name, true); err != nil {
		return errors.Wrap(err, "invalid name")
	}
	if err := ValidateCategory(l.Category); err != nil {
		return errors.Wrap(err, "invalid category")
	}
	return nil
}
