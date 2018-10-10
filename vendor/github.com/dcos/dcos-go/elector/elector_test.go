package elector

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dcos/dcos-go/testutils"
	"github.com/fortytw2/leaktest"
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

type eventsCloser struct {
	Connector
	writeLock *sync.Mutex
	events    chan zk.Event
}

func (e *eventsCloser) Close() error {
	func() {
		e.writeLock.Lock()
		defer e.writeLock.Unlock()
		close(e.events)
	}()
	return e.Connector.Close()
}

func TestClose_NoGoroutineLeak(t *testing.T) {
	// See notes in the subtest func about the need to run
	// multiple times to guard against flakes.
	for i := 0; i < 100 && !t.Failed(); i++ {
		testCloseNoGoroutineLeak(t)
	}
}

func testCloseNoGoroutineLeak(t *testing.T) {
	defer leaktest.Check(t)()

	events := make(chan zk.Event)
	childevents := make(chan zk.Event)
	writeLock := new(sync.Mutex) // guard against concurrent chan send and close ops
	childrenWFCount := 0
	conn := ConnAdapter{
		ExistsF: func(p string) (bool, *zk.Stat, error) {
			t.Logf("exists: %q", p)
			// block until events is writable
			writeLock.Lock()
			defer writeLock.Unlock()
			events <- zk.Event{State: zk.StateSaslAuthenticated}
			return true, nil, nil
		},
		CreateProtectedEphemeralSequentialF: func(string, []byte, []zk.ACL) (string, error) {
			writeLock.Lock()
			defer writeLock.Unlock()
			events <- zk.Event{State: zk.StateSaslAuthenticated}
			return "whatever-lock-123", nil
		},
		ChildrenWF: func(string) ([]string, *zk.Stat, <-chan zk.Event, error) {
			writeLock.Lock()
			defer writeLock.Unlock()
			if childrenWFCount > 0 {
				return nil, nil, nil, fmt.Errorf("ZK closed")
			}
			childrenWFCount++
			events <- zk.Event{State: zk.StateSaslAuthenticated}
			return []string{"whatever-lock-123"}, nil, childevents, nil
		},
		GetF: func(s string) ([]byte, *zk.Stat, error) {
			return ([]byte)("foo"), nil, nil
		},
	}
	ctor := &eventsCloser{
		Connector: ExistingConnection(conn, events),
		writeLock: writeLock,
		events:    events,
	}
	el, err := Start("id", "base", nil, ctor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ch := make(chan error, 1)
	go func() {
		defer close(ch)
		select {
		// initial leader event
		case ev := <-el.Events():
			if ev.Err != nil {
				ch <- ev.Err
			}
		case <-time.After(5 * time.Second):
			ch <- fmt.Errorf("timed out waiting for initial leader event")
		}
	}()

	err = <-ch
	if err != nil {
		t.Fatal(err)
	}

	// No one is reading elector events anymore because the
	// system is going down. Child events close to indicate that
	// it is shutting down. The elector attempts to re-list the
	// children and ZK generates an error.
	// The result is that the goroutine will block forever while
	// trying to send an error to the elector client that is no
	// longer listening.
	// The error handling code in the elector will race so this
	// test should be run multiple times to verify that it's not
	// going to flake in production.
	close(childevents)
	el.Close()
}
