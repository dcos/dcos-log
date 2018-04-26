package zkstore

import (
	"crypto/md5"
	"encoding/binary"
	"hash"
)

// HashProviderFunc is a factory for hashers.
type HashProviderFunc func(string) (uint64, error)

// DefaultHashProviderFunc is the default provider of hash functions
// unless overriden with OptHashProviderFunc when building the Store.
var DefaultHashProviderFunc = HashProvider(md5.New)

// hashBytesToBucket produces a positive int from a hashed byte slice
func hashBytesToBucket(bucketCount int, res uint64) int {
	mod := res % uint64(bucketCount)
	return int(mod)
}

// HashProvider adapts a golang stdlib hasher into a hash provider func for use by this package.
func HashProvider(f func() hash.Hash) HashProviderFunc {
	if f == nil {
		return nil
	}
	return func(s string) (uint64, error) {
		hasher := f()
		_, err := hasher.Write([]byte(s))
		if err != nil {
			return 0, err
		}
		hash := hasher.Sum(nil)
		if x := len(hash); x > 8 {
			hash = hash[:8]
		}
		y, z := binary.Uvarint(hash)
		if z < 0 {
			// should never happen since we cap the length of the buffer before decoding
			return 0, errHashOverflow
		}
		return y, nil
	}
}
