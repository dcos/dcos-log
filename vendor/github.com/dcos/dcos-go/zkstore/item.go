package zkstore

import (
	"fmt"

	"github.com/pkg/errors"
)

// MaxDataSize represents the size of the largest data blob that a caller can store.
const MaxDataSize = 1024 * 1024

// Item represents the data of a particular item in the store
type Item struct {
	// Ident identifies an Item in the ZK backend.
	Ident

	// Data represents the bytes to be stored within the znode.
	Data []byte
}

// Validate performs validation on the Item
func (i Item) Validate() error {
	if err := i.Ident.Validate(); err != nil {
		return err
	}
	if len(i.Data) > MaxDataSize {
		return errors.New("data is greater than 1MB")
	}
	return nil
}

func (i Item) String() string {
	return fmt.Sprintf("{ident=%v datalen=%dB}", i.Ident, len(i.Data))
}
