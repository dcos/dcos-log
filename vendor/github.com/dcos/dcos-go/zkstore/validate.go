package zkstore

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

var (
	validNameRE     = regexp.MustCompile(`^[\w\d_-]+$`)
	validCategoryRE = regexp.MustCompile(`^[/\w\d_-]+$`)
)

// ValidateNamed validates items that have a "name". Like an actual Name
// or perhaps a Version.  Since some names can be blank, we use the
// 'required' parameter to signify whether or not a name can be blank, and
// then after that we check against the regexp.
func ValidateNamed(name string, required bool) error {
	if strings.TrimSpace(name) != name {
		return errors.New("leader or trailing spaces not allowed")
	}
	if name == "" {
		if required {
			return errors.New("cannot be blank")
		}
		return nil
	}
	if !validNameRE.MatchString(name) {
		return fmt.Errorf("must match %s", validNameRE)
	}
	return nil
}

// ValidateCategory checks that a category is required, and can look like a path or not.
func ValidateCategory(name string) error {
	if strings.TrimSpace(name) == "" {
		return errBadCategory
	}
	if !validCategoryRE.MatchString(name) {
		return fmt.Errorf("must match %s", validCategoryRE)
	}
	cleaned := path.Clean(name)
	if cleaned != name {
		return errBadCategory
	}
	return nil
}
