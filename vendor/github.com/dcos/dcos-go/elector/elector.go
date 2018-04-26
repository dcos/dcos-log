package elector

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"
)

// IElector is the interface to which the Elector must adhere. Clients may
// choose to use this, but the Start() method will return a concrete type,
// keeping in line with 'return concrete types, accept interfaces'.
type IElector interface {
	// LeaderIdent returns the current leader of the cluster, or "" if
	// the current leader is not known.
	LeaderIdent() string

	// Events returns a channel from which the client should consume events
	// from the elector.  The channel will be closed after an error event
	// is sent, as the elector is no longer usable from that point on.
	Events() <-chan Event

	// Close tidies up any applicable connection details to ZK. Clients
	// should call then when the elector is no longer needed
	Close() error
}

// ensure that Elector adheres to the IElector interface
var _ IElector = &Elector{}

// Elector handles leadership elections
type Elector struct {
	acl      []zk.ACL
	conn     Conn
	events   chan Event
	ident    string       // the ident of the elector
	basePath string       // where the elector nodes will be created
	closer   func() error // the connector shutdown func

	mut         sync.Mutex // mut guards the following mutable state:
	leaderIdent string     // the current leader's ident
	isLeader    bool       // whether or not the current elector is leader
}

var (
	// sequenceRe is a regexp that is used to extract sequence parts
	// from sequential znodes.
	sequenceRe = regexp.MustCompile(`.*-lock-(-?\d+)$`)
)

// Start builds a new elector and runs it in the background.
//
// The 'ident' parameter is the content that the elector will store inside of
// it's znode data.  This will typically be the IP address of the client of
// the elector.
//
// The 'basePath' parameter is the znode under which the leader election will
// happen.
//
// The 'acl' will be set on any nodes that must be created
func Start(ident string, basePath string, acl []zk.ACL, connector Connector) (*Elector, error) {
	if strings.TrimSpace(ident) == "" {
		return nil, errors.New("ident must not be blank")
	}
	if acl == nil {
		acl = zk.WorldACL(zk.PermAll)
	}
	conn, zkEvents, err := connector.Connect()
	if err != nil {
		return nil, err
	}
	elector := &Elector{
		acl:      acl,
		ident:    ident,
		conn:     conn,
		basePath: basePath,
		events:   make(chan Event),
		closer:   connector.Close,
	}
	go elector.start(zkEvents)
	return elector, nil
}

// LeaderIdent returns the current leader, or "" if no current leader is
// known yet.
func (e *Elector) LeaderIdent() string {
	e.mut.Lock()
	defer e.mut.Unlock()
	return e.leaderIdent
}

// Events returns a channel on which Events will be sent.
func (e *Elector) Events() <-chan Event {
	return e.events
}

// Close closes the underlying ZK connection. Clients should call Close() when
// abandoning elector efforts in order to quickly delete any ephemeral nodes
// that were created as a part of the election process.
func (e *Elector) Close() error {
	return e.closer()
}

// initialize sets up the basePath if necessary
func (e *Elector) initialize() error {
	exists, _, err := e.conn.Exists(e.basePath)
	if err != nil {
		return errors.Wrapf(err, "could not check if base path %s exists", e.basePath)
	}
	if exists {
		return nil
	}
	segments := strings.Split(e.basePath, "/")
	create := "/"
	for _, segment := range segments {
		create = path.Join(create, segment)
		exists, _, err := e.conn.Exists(create)
		if err != nil {
			return errors.Wrapf(err, "could not check path '%s'", create)
		}
		if exists {
			continue
		}
		_, err = e.conn.Create(create, []byte{}, 0, e.acl)
		if err != nil {
			return errors.Wrapf(err, "could not create path '%s'", create)
		}
	}
	return nil
}

func (e *Elector) start(zkEvents <-chan zk.Event) {
	defer close(e.events)
	errch := make(chan error, 1)
	shouldQuit := make(chan struct{})

	var wg sync.WaitGroup
	var shouldQuitOnce sync.Once
	cancel := func() { shouldQuitOnce.Do(func() { close(shouldQuit) }) }

	// must spawn this before initialize() to avoid blocking
	wg.Add(1)
	go func() {
		defer cancel()
		defer wg.Done()
		for {
			select {
			case <-shouldQuit:
				return
			case zkEvent := <-zkEvents:
				if err := checkZKEvent(zkEvent); err != nil {
					select {
					case errch <- err:
					default:
					}
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer cancel()
		defer wg.Done()
		err := func() error {
			if err := e.initialize(); err != nil {
				return errors.Wrap(err, "elector initialization failed")
			}
			lockPath, err := e.conn.CreateProtectedEphemeralSequential(
				e.basePath+"/lock-",
				[]byte(e.ident),
				e.acl)
			if err != nil {
				return errors.Wrap(err, "could not create lock node")
			}

			firstLeaderUpdate := true
			updateFunc := func(children []string) error {
				isLeader, leaderNode, err := determineLeader(lockPath, children)
				if err != nil {
					return errors.Wrap(err, "could not determine leader")
				}
				leaderIdent, err := e.getIdentFromNode(leaderNode)
				if err != nil {
					return errors.Wrap(err, "could not get ident from node")
				}
				e.updateLeaderData(leaderIdent, isLeader, firstLeaderUpdate)
				firstLeaderUpdate = false
				return nil
			}

			children, _, childEvents, err := e.conn.ChildrenW(e.basePath)
			if err != nil {
				return errors.Wrap(err, "could not get children")
			}
			if err = updateFunc(children); err != nil {
				return err
			}
			for {
				select {
				case _, ok := <-childEvents:
					if !ok {
						return errors.New("child events stream terminated")
					}
					children, _, childEvents, err = e.conn.ChildrenW(e.basePath)
					if err != nil {
						return errors.Wrap(err, "could not get children")
					}
					if err = updateFunc(children); err != nil {
						return err
					}
				case <-shouldQuit:
					return nil
				}
			}
		}()
		select {
		case errch <- err:
		default:
		}
	}()

	wg.Wait()
	close(errch)

	// the elector errored out unexpectedly. send an error to the client.
	e.sendErr(<-errch)
}

func checkZKEvent(zkEvent zk.Event) error {
	if zkEvent.Err != nil {
		return zkEvent.Err
	}
	switch zkEvent.State {
	case zk.StateExpired, zk.StateAuthFailed, zk.StateDisconnected, zk.StateUnknown:
		return fmt.Errorf("invalid ZK state: %v", zkEvent.State)
	}
	return nil
}

// updateLeaderData updates the leadership information on the elector, and also
// sends a Leader event if the elector leadership transitioned.
func (e *Elector) updateLeaderData(leaderIdent string, leader bool, forceSend bool) {
	e.mut.Lock()
	prevLeader := e.isLeader // used later to determine if update necessary
	e.leaderIdent = leaderIdent
	e.isLeader = leader
	e.mut.Unlock()
	if prevLeader == leader && !forceSend {
		return
	}
	e.sendEvent(Event{Leader: leader})
}

// getIdentFromNode fetches the znode data from the specified node and returns
// it as a string
func (e *Elector) getIdentFromNode(node string) (ident string, err error) {
	nodePath := path.Join(e.basePath, node)
	b, _, err := e.conn.Get(nodePath)
	return string(b), err
}

// sendErr sends an error event on the events chan.
func (e *Elector) sendErr(err error) {
	e.sendEvent(Event{Err: err})
}

// sendEvent sends the specified event on the events channel
func (e *Elector) sendEvent(event Event) {
	e.events <- event
}

// sorted children sequences converts the children to sequence parts, and
// then returns the sorted sequences, along with a lookup map of sequence
// to nodes
func sortedChildrenSequences(children []string) (sorted []int, lookup map[int]string, err error) {
	sorted = make([]int, len(children))
	lookup = make(map[int]string)
	for i, child := range children {
		seq, err := sequencePart(child)
		if err != nil {
			return nil, nil, err
		}
		sorted[i] = seq
		lookup[seq] = child
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	return sorted, lookup, nil
}

// determineLeader takes the current node, and all of the children of the
// leader node, and then determines if the node is the leader, and also,
// which node is the leader.
func determineLeader(node string, children []string) (isLeader bool, leaderNode string, err error) {
	err = func() error {
		if len(children) == 0 {
			return errors.New("no child nodes")
		}
		sequences, lookup, err := sortedChildrenSequences(children)
		if err != nil {
			return err
		}
		mySeq, err := sequencePart(node)
		if err != nil {
			return errors.Wrap(err, "invalid owner node")
		}
		leaderSeq := sequences[0]
		isLeader = mySeq == leaderSeq
		leaderNode = lookup[leaderSeq]
		return nil
	}()
	return isLeader, leaderNode, err
}

// sequencePart extracts the trailing integer part of a zk sequential node
// into an int.
func sequencePart(node string) (int, error) {
	if node == "" {
		return 0, errors.New("node cannot be blank")
	}
	matches := sequenceRe.FindStringSubmatch(node)
	if len(matches) != 2 {
		return 0, fmt.Errorf("invalid node: %s", node)
	}
	res, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid sequence part: %s", matches[1])
	}
	return res, nil
}
