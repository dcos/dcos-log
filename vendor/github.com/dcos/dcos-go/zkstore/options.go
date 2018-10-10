package zkstore

import (
	"path"
	"strings"

	"github.com/samuel/go-zookeeper/zk"
)

// StoreOpt allows a Store to be configured.
// Returns ErrIllegalOption if the option configuration cannot be applied to the store.
type StoreOpt func(store *Store) error

// Apply is a convenience method that handles nil StoreOpt funcs w/ aplomb: it is perfectly legal to invoke StoreOpt(nil).Apply(someStore).
func (f StoreOpt) Apply(store *Store) error {
	if f != nil {
		return f(store)
	}
	return nil
}

// OptBasePath specifies a root path that will be prepended to all paths written to
// or read from.
// An empty path will not change the store configuration.
// The specified path must begin with "/" and be 'clean' (see path.Clean) otherwise
// ErrIllegalOption is returned.
func OptBasePath(basePath string) StoreOpt {
	if basePath == "" {
		return nil
	}
	if !strings.HasPrefix(basePath, "/") {
		return optError
	}
	if cleaned := path.Clean(basePath); cleaned != basePath {
		return optError
	}
	return func(store *Store) error {
		store.basePath = basePath
		return nil
	}
}

// OptNumHashBuckets specifies the number of hash buckets that will be created under
// a store path for each content type when data is being written or read.
//
// If this value is changed after data is written, previously written data may
// not be able to be found later.
// If the bucket count is zero then the store configuration is not altered.
// If the bucket count is negative then ErrIllegalOption is returned.
func OptNumHashBuckets(numBuckets int) StoreOpt {
	if numBuckets == 0 {
		return nil
	}
	if numBuckets < 0 {
		return optError
	}
	return func(store *Store) error {
		store.hashBuckets = numBuckets
		return optBucketFunc(bucketFunc(numBuckets, store.hashProviderFunc)).Apply(store)

	}
}

// OptACL configures the store to use a particular ACL when creating nodes.
// A nil or empty ACL list does not alter the store configuration.
func OptACL(acl []zk.ACL) StoreOpt {
	if len(acl) == 0 {
		return nil // use default instead
	}
	return func(store *Store) error {
		store.acls = acl
		return nil
	}
}

// OptHashProviderFunc allows the client to configure which hasher to use to map
// item names to buckets.
// A nil hash func does not alter the store configuration.
func OptHashProviderFunc(hashProviderFunc HashProviderFunc) StoreOpt {
	if hashProviderFunc == nil {
		return nil // use default instead
	}
	return func(store *Store) error {
		store.hashProviderFunc = hashProviderFunc
		return optBucketFunc(bucketFunc(store.hashBuckets, hashProviderFunc)).Apply(store)
	}
}

// OptBucketsZnodeName allows the client to configure the znode name that will
// contain the numerically-named bucket nodes.
// Returns ErrIllegalOption when the specifeid znode name is invalid.
func OptBucketsZnodeName(name string) StoreOpt {
	if err := ValidateNamed(name, true); err != nil {
		return optError
	}
	return func(store *Store) error {
		store.bucketsZnodeName = name
		return nil
	}
}

func optBucketFunc(f func(string) (int, error)) StoreOpt {
	if f == nil {
		return nil
	}
	return func(store *Store) error {
		store.bucketFunc = f
		return nil
	}
}

func optError(*Store) error { return ErrIllegalOption }
