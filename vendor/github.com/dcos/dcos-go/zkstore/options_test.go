package zkstore

import (
	"crypto/md5"
	"crypto/sha1"
	"testing"

	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/require"
)

func TestOptBasePath(t *testing.T) {
	require := require.New(t)
	store := &Store{}
	require.EqualError(OptBasePath("foo")(store), ErrIllegalOption.Error())
	require.NoError(OptBasePath("/foo")(store))
}

func TestOptHashBuckets(t *testing.T) {
	require := require.New(t)
	store := &Store{}
	require.NoError(OptNumHashBuckets(0).Apply(store))
	require.EqualError(OptNumHashBuckets(-1).Apply(store), ErrIllegalOption.Error())
	require.NoError(OptNumHashBuckets(1).Apply(store))
}

func TestOptACL(t *testing.T) {
	require := require.New(t)
	store := &Store{}
	require.NoError(OptACL(nil).Apply(store))
	require.NoError(OptACL([]zk.ACL{}).Apply(store))
	require.NoError(OptACL(zk.WorldACL(zk.PermAll)).Apply(store))
}

func TestOptHashProviderFunc(t *testing.T) {
	require := require.New(t)
	store := &Store{}
	require.NoError(OptHashProviderFunc(nil).Apply(store))
	require.NoError(OptHashProviderFunc(HashProvider(md5.New)).Apply(store))
	require.NoError(OptHashProviderFunc(HashProvider(sha1.New)).Apply(store))
}
