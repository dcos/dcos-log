package zkstore

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha512"
	"fmt"
	"io"
	"sort"
	"testing"

	"github.com/dcos/dcos-go/testutils"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/require"
)

func TestExpectedBehavior(t *testing.T) {
	store, _, teardown := newStoreTest(t, OptBasePath("/storage"))
	defer teardown()
	require := require.New(t)

	locations, err := store.List("widgets")
	require.EqualValues(err, ErrNotFound)
	require.Nil(locations)

	item1 := Item{
		Ident: Ident{Location: Location{Category: "widgets", Name: "item1"}},
		Data:  []byte("item1"),
	}
	item2 := Item{
		Ident: Ident{Location: Location{Category: "widgets", Name: "item2"}},
		Data:  []byte("item2"),
	}

	_, err = store.Put(item1)
	require.NoError(err)
	_, err = store.Put(item2)
	require.NoError(err)

	locations, err = store.List("widgets")
	require.NoError(err)
	sort.Slice(LocationsByName(locations))
	require.EqualValues([]Location{
		{Category: "widgets", Name: "item1"},
		{Category: "widgets", Name: "item2"},
	}, locations)

	// set a version on item1
	_, err = store.Put(Item{
		Ident: Ident{Location: Location{Category: "widgets", Name: "item1"}, Variant: "v2"},
		Data:  []byte("item1v2"),
	})
	require.NoError(err)

	// locations should not include versions
	locations, err = store.List("widgets")
	require.NoError(err)
	sort.Slice(LocationsByName(locations))
	require.EqualValues([]Location{
		{Category: "widgets", Name: "item1"},
		{Category: "widgets", Name: "item2"},
	}, locations)

	// we can get the variants for an item if we ask for it specifically
	variants, err := store.Variants(Location{Category: "widgets", Name: "item1"})
	require.NoError(err)
	require.EqualValues([]string{"v2"}, variants)

	// fetch a particular version
	item, err := store.Get(Ident{Location: Location{Category: "widgets", Name: "item1"}, Variant: "v2"})
	require.NoError(err)
	require.EqualValues("item1v2", string(item.Data))

	// we should still be able to fetch the first one
	item, err = store.Get(Ident{Location: Location{Category: "widgets", Name: "item1"}})
	require.NoError(err)
	require.EqualValues("item1", string(item.Data))

	// try to delete a node that doesn't exist
	err = store.Delete(Ident{Location: Location{Category: "widgets", Name: "item-none"}})
	require.NoError(err)

	// try to delete a version that doesn't exist
	err = store.Delete(Ident{Location: Location{Category: "widgets", Name: "item-none"}, Variant: "v2"})
	require.NoError(err)

	// delete the node without any children
	err = store.Delete(Ident{Location: Location{Category: "widgets", Name: "item2"}})
	require.NoError(err)

	// try to query the node after you deleted it
	err = store.Delete(Ident{Location: Location{Category: "widgets", Name: "item2"}})
	require.NoError(err)

	// query the locations for the widgets category. we should only have one now.
	locations, err = store.List("widgets")
	require.NoError(err)
	require.EqualValues([]Location{{Category: "widgets", Name: "item1"}}, locations)

	// update the remaining item
	_, err = store.Put(Item{
		Ident: Ident{Location: Location{Category: "widgets", Name: "item1"}},
		Data:  []byte("item1updated"),
	})
	require.NoError(err)

	// verify the overwrite
	item, err = store.Get(Ident{Location: Location{Category: "widgets", Name: "item1"}})
	require.NoError(err)
	require.Equal("item1updated", string(item.Data))

	// delete the first item and all of its variants
	err = store.Delete(Ident{Location: Location{Category: "widgets", Name: "item1"}})
	require.NoError(err)

	// verify that it was deleted
	item, err = store.Get(Ident{Location: Location{Category: "widgets", Name: "item1"}})
	require.EqualValues(err, ErrNotFound)

	// verify that its version was also deleted
	item, err = store.Get(Ident{Location: Location{Category: "widgets", Name: "item1"}, Variant: "v2"})
	require.EqualValues(err, ErrNotFound)
}

func TestVersionParentNodeDataIsNotSetIfItAlreadyExists(t *testing.T) {
	store, conn, teardown := newStoreTest(t, fixedBucketFunc(42), OptBasePath("/storage"))
	defer teardown()
	require := require.New(t)

	_, err := store.Put(Item{
		Ident: Ident{
			Location: Location{
				Category: "/widgets",
				Name:     "foo",
			},
		},
		Data: []byte("parent"),
	})
	require.NoError(err)

	_, err = store.Put(Item{
		Ident: Ident{
			Location: Location{
				Category: "/widgets",
				Name:     "foo",
			},
			Variant: "my-version",
		},
		Data: []byte("child"),
	})
	require.NoError(err)

	// verify that the parent node exists with the data
	data, stat, err := conn.Get("/storage/widgets/buckets/42/foo")
	require.NoError(err)
	require.NotNil(stat)
	require.Equal("parent", string(data))

	// verify that the version node exists with the data
	data, stat, err = conn.Get("/storage/widgets/buckets/42/foo/my-version")
	require.NoError(err)
	require.NotNil(stat)
	require.Equal("child", string(data))
}

func TestVersionParentNodeIsSetIfItDoesNotExist(t *testing.T) {
	store, conn, teardown := newStoreTest(t, fixedBucketFunc(42), OptBasePath("/storage"))
	defer teardown()
	require := require.New(t)

	_, err := store.Put(Item{
		Ident: Ident{
			Location: Location{
				Category: "/widgets",
				Name:     "foo",
			},
			Variant: "my-version",
		},
		Data: []byte("hello"),
	})
	require.NoError(err)

	// verify that the parent node exists with the data
	data, stat, err := conn.Get("/storage/widgets/buckets/42/foo")
	require.NoError(err)
	require.NotNil(stat)
	require.Equal("hello", string(data))

	// verify that the version node exists with the data
	data, stat, err = conn.Get("/storage/widgets/buckets/42/foo/my-version")
	require.NoError(err)
	require.NotNil(stat)
	require.Equal("hello", string(data))
}

// If the Version is set on an Ident, we need to ensure that the underlying
// store respects that and rejects operations for which a version was
// different from that which was specified.
func TestVersion(t *testing.T) {
	store, _, teardown := newStoreTest(t, fixedBucketFunc(42), OptBasePath("/storage"))
	defer teardown()
	require := require.New(t)

	newItem := func() Item {
		return Item{
			Ident: Ident{
				Location: Location{
					Category: "/widgets",
					Name:     "foo",
				},
				Variant: "my-version",
			},
			Data: []byte("hello"),
		}
	}

	// this should fail because the item does not exist yet
	item := newItem()
	item.Ident.Version = NewVersion(42) // invalid
	ident, err := store.Put(item)
	require.Equal(ErrVersionConflict, err)

	item = newItem()
	ident, err = store.Put(item)
	require.NoError(err)
	require.EqualValues(0, ident.actualVersion())

	// put the same item again, verify its zkVersion==1
	item = newItem()
	ident, err = store.Put(item)
	require.NoError(err)
	require.EqualValues(1, ident.actualVersion())

	// put the same item, but set it to use zkversion=1
	item = newItem()
	item.Ident.Version = NewVersion(1)
	ident, err = store.Put(item)
	require.NoError(err)
	require.EqualValues(2, ident.actualVersion())

	// put the same item, but set a previous version
	item = newItem()
	item.Ident.Version = NewVersion(1)
	ident, err = store.Put(item)
	require.EqualValues(ErrVersionConflict, err)

	// at this point, our stored item is still at version 2
	// we should get a conflict if we try to delete version 1
	item = newItem()
	item.Ident.Version = NewVersion(1)
	err = store.Delete(item.Ident)
	require.EqualValues(ErrVersionConflict, err)
}

func TestVersionSetOnCreate(t *testing.T) {
	store, _, teardown := newStoreTest(t, fixedBucketFunc(42), OptBasePath("/storage"))
	defer teardown()
	require := require.New(t)

	newItem := func() Item {
		return Item{
			Ident: Ident{
				Location: Location{
					Category: "/widgets/awesome/two",
					Name:     "foo",
				},
				Variant: "my-version",
			},
			Data: []byte("hello"),
		}
	}

	// this should fail because the item does not exist yet and version > 0
	item := newItem()
	item.Ident.Version = NewVersion(42) // invalid
	ident, err := store.Put(item)
	require.Equal(ErrVersionConflict, err)

	// this should succeed because the item does not exist yet and version == NoPriorVersion
	item = newItem()
	item.Ident.Version = NewVersion(NoPriorVersion)
	ident, err = store.Put(item)
	require.NoError(err)
	require.EqualValues(0, ident.actualVersion())

	// this should fail because the item was already created.
	item = newItem()
	item.Ident.Version = NewVersion(NoPriorVersion)
	ident, err = store.Put(item)
	require.Equal(ErrVersionConflict, err)

	// this should succeed because the item already exists and has not been
	// subsequently updated which means currently its version==0.
	item = newItem()
	item.Ident.Version = NewVersion(0)
	ident, err = store.Put(item)
	require.NoError(err)
	require.EqualValues(1, ident.actualVersion())

	// this should fail because the item has since been updated from version==0 to version==1
	item = newItem()
	item.Ident.Version = NewVersion(0)
	ident, err = store.Put(item)
	require.Equal(ErrVersionConflict, err)

	// put the same item again, verify its version==2
	item = newItem()
	ident, err = store.Put(item)
	require.NoError(err)
	require.EqualValues(2, ident.actualVersion())

	// put the same item, but set it to use zkversion=2
	item = newItem()
	item.Ident.Version = NewVersion(2)
	ident, err = store.Put(item)
	require.NoError(err)
	require.EqualValues(3, ident.actualVersion())

	// put the same item, but fail when trying to set a previous version
	item = newItem()
	item.Ident.Version = NewVersion(2)
	ident, err = store.Put(item)
	require.EqualValues(ErrVersionConflict, err)

	// at this point, our stored item is still at version 3
	// we should get a conflict if we try to delete version 2
	item = newItem()
	item.Ident.Version = NewVersion(2)
	err = store.Delete(item.Ident)
	require.EqualValues(ErrVersionConflict, err)
}

func TestVersionIsIncrementedOnPut(t *testing.T) {
	store, _, teardown := newStoreTest(t, fixedBucketFunc(42), OptBasePath("/storage"))
	defer teardown()
	require := require.New(t)

	// test zk versions for item nodes
	for i := 0; i < 5; i++ {
		ident, err := store.Put(Item{
			Ident: Ident{
				Location: Location{
					Category: "/widgets",
					Name:     "foo",
				},
			},
			Data: []byte("hello"),
		})
		require.NoError(err)
		clone := ident
		clone.Version = NewVersion(int32(i))
		require.EqualValues(clone, ident)
	}

	// test zk versions for version nodes
	for i := 0; i < 5; i++ {
		ident, err := store.Put(Item{
			Ident: Ident{
				Location: Location{
					Category: "/widgets",
					Name:     "foo",
				},
				Variant: "my-version",
			},
			Data: []byte("hello"),
		})
		require.NoError(err)
		clone := ident
		clone.Version = NewVersion(int32(i))
		require.EqualValues(clone, ident)
	}
}

func TestListLocations(t *testing.T) {
	store, _, teardown := newStoreTest(t, fixedBucketFunc(42), OptBasePath("/storage"))
	defer teardown()
	require := require.New(t)

	_, err := store.Put(Item{
		Ident: Ident{
			Location: Location{Category: "widgets/2017", Name: "foo"},
			Variant:  "my-version",
		},
		Data: []byte("hello"),
	})
	require.NoError(err)
	_, err = store.Put(Item{
		Ident: Ident{
			Location: Location{Category: "widgets/2017", Name: "bar"},
			Variant:  "my-version",
		},
		Data: []byte("hello"),
	})
	require.NoError(err)

	locations, err := store.List("/widgets/2017")
	require.NoError(err)
	require.EqualValues([]Location{
		{Name: "bar", Category: "/widgets/2017"},
		{Name: "foo", Category: "/widgets/2017"},
	}, locations)
	require.Len(locations, 2)
}

// ensure a reasonable distribution of buckets for a range of hash functions.
//
// NB: i could not get the fnv hash to pass this test
func TestBucketDistribution(t *testing.T) {
	type hashTest struct {
		name             string
		hashProviderFunc HashProviderFunc
	}
	for _, test := range []hashTest{
		{"md5", HashProvider(md5.New)},
		{"sha", HashProvider(sha1.New)},
		{"sha512", HashProvider(sha512.New)},
	} {
		require := require.New(t)
		numBuckets := 1024
		numNames := 1024 * 1024
		bf := bucketFunc(numBuckets, test.hashProviderFunc)
		hits := make(map[int]int)
		for i := 0; i < numBuckets; i++ {
			hits[i] = 0
		}
		for i := 0; i < numNames; i++ {
			b, err := bf(fmt.Sprintf("name-%d", i))
			require.NoError(err)
			if b < 0 {
				t.Fatalf("hashFunc=%v got negative bucket value", test.name)
			}
			hits[b]++
		}
		for b, num := range hits {
			if num == 0 {
				t.Fatalf("hashFunc=%v bucket %v did not have any hits", test.name, b)
			}
		}
	}
}

func TestHashProducesSameValues(t *testing.T) {
	require := require.New(t)
	for i := 0; i < 1024; i++ {
		bf := bucketFunc(64, DefaultHashProviderFunc)
		name := fmt.Sprintf("name-%d", i)
		b1, err := bf(name)
		require.NoError(err)
		b2, err := bf(name)
		require.NoError(err)
		require.Equal(b1, b2)
	}
}

// TestIdentPath ensures that our path generation routines are working as
// expected.
func TestIdentPath(t *testing.T) {
	isBadCategoryName := func(err error) bool { return err == errBadCategory }
	type testCase struct {
		ident      Ident
		basePath   string
		path       string
		wantsError func(error) bool
	}
	for _, test := range []testCase{
		{
			ident:      Ident{Location: Location{Name: "my-name", Category: "buckets"}},
			wantsError: isBadCategoryName,
		},
		{
			ident:      Ident{Location: Location{Name: "my-name", Category: "/buckets"}},
			wantsError: isBadCategoryName,
		},
		{
			ident:      Ident{Location: Location{Name: "my-name", Category: "/buckets/"}},
			wantsError: isBadCategoryName,
		},
		{
			ident:      Ident{Location: Location{Name: "my-name", Category: "foo/buckets"}},
			wantsError: isBadCategoryName,
		},
		{
			ident:      Ident{Location: Location{Name: "my-name", Category: "/foo/buckets/"}},
			wantsError: isBadCategoryName,
		},
		{
			ident: Ident{Location: Location{Name: "my-name", Category: "widgets"}},
			path:  "/widgets/buckets/42/my-name",
		},
		{
			ident: Ident{Location: Location{Name: "my-name", Category: "widgets/2017"}},
			path:  "/widgets/2017/buckets/42/my-name",
		},
		{
			basePath: "/storage",
			ident:    Ident{Location: Location{Name: "my-name", Category: "widgets"}},
			path:     "/storage/widgets/buckets/42/my-name",
		},
		{
			basePath: "/storage/things",
			ident:    Ident{Location: Location{Name: "my-name", Category: "widgets"}},
			path:     "/storage/things/widgets/buckets/42/my-name",
		},
		{
			basePath: "/storage/things",
			ident:    Ident{Location: Location{Name: "my-name", Category: "widgets/2017"}},
			path:     "/storage/things/widgets/2017/buckets/42/my-name",
		},
	} {
		store, err := NewStore(noConn(), OptBasePath(test.basePath), fixedBucketFunc(42))
		if err != nil {
			t.Fatalf("%#v produced err:%v", test, err)
		}
		identPath, err := store.identPath(test.ident)
		if (test.wantsError != nil) != (err != nil) {
			if err != nil {
				t.Fatalf("unexpected error for test case %#v: %v", test, err)
			} else {
				t.Fatalf("expected error for test case %#v", test)
			}
		}
		if test.wantsError != nil && !test.wantsError(err) {
			t.Fatalf("expected error other than %v for test case %#v", err, test)
		}
		if identPath != test.path {
			t.Fatalf("%#v produced path:%v", test, identPath)
		}
	}
}

func newStoreTest(t *testing.T, storeOpts ...StoreOpt) (store *Store, zkConn *zk.Conn, teardown func()) {
	zkCtl, err := testutils.StartZookeeper()
	if err != nil {
		t.Fatal(err)
	}
	connector := NewConnection([]string{zkCtl.Addr()}, ConnectionOpts{})
	conn, err := connector.Connect()
	if err != nil {
		t.Fatal(err)
	}
	store, err = NewStore(ExistingConnection(conn), storeOpts...)
	if err != nil {
		t.Fatal(err)
	}
	return store, conn, func() {
		closePanic(store)
		conn.Close()
		zkCtl.TeardownPanic()
	}
}

func closePanic(closer io.Closer) {
	if err := closer.Close(); err != nil {
		panic(err)
	}
}

// noConn returns a connector that does nothing, for tests on the Store which
// do not require zookeeper.
func noConn() Connector {
	return ExistingConnection(nil)
}

// fixedBucketFunc configures a store to always use a single bucket number. This
// is used to make verification easy in tests.
func fixedBucketFunc(bucket int) StoreOpt {
	return optBucketFunc(func(_ string) (int, error) { return bucket, nil })
}
