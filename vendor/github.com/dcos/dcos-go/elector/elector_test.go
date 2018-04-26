package elector

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dcos/dcos-go/testutils"
	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/stretchr/testify/require"
)

// the base path to use for tests
var basePath = "/leader/election/lock"
var opts = ConnectionOpts{}

func TestZookeeperPartition(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Does not work on Windows yet")
	}
	require := require.New(t)
	zkCtl, err := testutils.StartZookeeper()
	require.NoError(err)
	defer zkCtl.TeardownPanic()

	e1, err := Start("foo", basePath, nil, NewConnection([]string{zkCtl.Addr()}, opts))
	require.NoError(err)
	defer e1.Close()

	require.NoError(electorBecomesLeader(e1, true))
	require.Equal("foo", e1.LeaderIdent())

	zkCtl.TeardownPanic()

	select {
	case e := <-e1.events:
		require.Error(e.Err)
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for an error")
	}
}

func TestExpectedBehavior(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Does not work on Windows yet")
	}
	require := require.New(t)
	zkCtl, err := testutils.StartZookeeper()
	require.NoError(err)
	defer zkCtl.TeardownPanic()

	e1, err := Start("e1", basePath, nil, NewConnection([]string{zkCtl.Addr()}, opts))
	require.NoError(err)
	defer e1.Close()

	require.NoError(electorBecomesLeader(e1, true))
	require.Equal("e1", e1.LeaderIdent())

	e2, err := Start("e2", basePath, nil, NewConnection([]string{zkCtl.Addr()}, opts))
	require.NoError(err)
	defer e2.Close()

	require.NoError(electorBecomesLeader(e2, false))
	require.Equal("e1", e1.LeaderIdent())
	require.Equal("e1", e2.LeaderIdent())

	// shut down e1
	require.NoError(e1.Close())

	// verify e2 becomes leader
	require.NoError(electorBecomesLeader(e2, true))
	require.Equal("e2", e2.LeaderIdent())
}

// leaderBecomes verifies that the elector emits a specific leader event
func electorBecomesLeader(e *Elector, leader bool) error {
	event, err := getEvent(e, 5*time.Second)
	if err != nil {
		return err
	}
	if event.Err != nil {
		return errors.Wrap(event.Err, "event error")
	}
	if event.Leader != leader {
		return errors.Errorf("expected %v but got %v", leader, event.Leader)
	}
	return nil
}

func getEvent(e *Elector, timeout time.Duration) (event Event, err error) {
	select {
	case event := <-e.Events():
		return event, nil
	case <-time.After(timeout):
		return event, errors.New("Timed out waiting for an event")
	}
}

func TestNodeIsLeader(t *testing.T) {
	type testCase struct {
		node        string
		children    []string
		isLeader    bool
		leaderIdent string
		err         error
	}
	lock1 := "/foo-lock-1"
	lock2 := "/foo-lock-2"
	lock3 := "/foo-lock-3"
	for _, test := range []testCase{
		{
			err: errors.New("no child nodes"),
		},
		{
			node: lock1,
			err:  errors.New("no child nodes"),
		},
		{
			node:     lock1,
			children: []string{},
			err:      errors.New("no child nodes"),
		},
		{
			children: []string{lock1, lock2, lock3},
			err:      errors.New("invalid owner node: node cannot be blank"),
		},
		{
			node:        lock1,
			children:    []string{lock2, lock3, lock1},
			isLeader:    true,
			leaderIdent: lock1,
		},
		{
			node:        lock2,
			children:    []string{lock3, lock1, lock2},
			isLeader:    false,
			leaderIdent: lock1,
		},
	} {
		isLeader, leaderIdent, err := determineLeader(test.node, test.children)
		if errMsg(err) != errMsg(test.err) || isLeader != test.isLeader ||
			leaderIdent != test.leaderIdent {
			t.Fatalf("test %+v got isLeader:%v leaderIdent:%v err:%v", test, isLeader, leaderIdent, err)
		}
	}
}

func TestSequencePart(t *testing.T) {
	type testCase struct {
		input string
		res   int
		err   error
	}
	for _, test := range []testCase{
		{
			input: "foo",
			err:   errors.New("invalid node: foo"),
		},
		{
			input: "foo/bar",
			err:   errors.New("invalid node: foo/bar"),
		},
		{
			input: "foo/bar-lock",
			err:   errors.New("invalid node: foo/bar-lock"),
		},
		{
			input: "foo/bar-lock-",
			err:   errors.New("invalid node: foo/bar-lock-"),
		},
		{
			input: "foo/bar-lock-001",
			res:   1,
		},
		{
			input: "foo/bar-lock--42",
			res:   -42,
		},
	} {
		res, err := sequencePart(test.input)
		if errMsg(err) != errMsg(test.err) || res != test.res {
			t.Fatalf("test %+v got res:%v err:%v", test, res, err)
		}
	}
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestStart_NoDeadlock(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)
	events := make(chan zk.Event)
	conn := ConnAdapter{
		ExistsF: func(p string) (bool, *zk.Stat, error) {
			defer wg.Done()
			t.Logf("exists: %q", p)
			// block until events is writable
			events <- zk.Event{State: zk.StateSaslAuthenticated}
			return true, nil, nil
		},
		CreateProtectedEphemeralSequentialF: func(string, []byte, []zk.ACL) (string, error) {
			events <- zk.Event{State: zk.StateSaslAuthenticated}
			return "protected", nil
		},
		ChildrenWF: func(string) ([]string, *zk.Stat, <-chan zk.Event, error) {
			events <- zk.Event{State: zk.StateSaslAuthenticated}
			return nil, nil, nil, nil
		},
	}
	ctor := ExistingConnection(conn, events)
	go func() {
		defer wg.Done()
		_, err := Start("id", "base", nil, ctor)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		wg.Wait()
	}()
	select {
	case <-ch:
		// we want this to happen because it means that
		// initialize() isn't blocking event chan processing
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start() to complete")
	}
}
