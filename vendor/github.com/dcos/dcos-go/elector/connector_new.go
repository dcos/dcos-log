package elector

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"
)

const (
	defaultConnectTimeout        = 5 * time.Second
	defaultInitialSessionTimeout = 5 * time.Second
)

// NewConnection returns a Connector that creates a new ZK connection
func NewConnection(addrs []string, opts ConnectionOpts) Connector {
	return &newConnection{
		addrs: addrs,
		opts:  opts,
	}
}

// ConnectionOpts are used when creating a new Zk connection
type ConnectionOpts struct {
	// ConnectTimeout is the timeout to make the initial connection to ZK.
	ConnectTimeout time.Duration

	// InitialSessionTimeout is how long to wait for a valid session to
	// be established once the connection happens.
	InitialSessionTimeout time.Duration

	// Auth represents authentication details. If left alone, no auth will
	// be performed
	Auth struct {
		Schema string
		Secret []byte
	}
}

type newConnection struct {
	addrs []string
	opts  ConnectionOpts
	conn  *zk.Conn
	once  sync.Once
}

func (c *newConnection) Connect() (Conn, <-chan zk.Event, error) {
	connectTimeout := durationOrDefault(c.opts.ConnectTimeout, defaultConnectTimeout)
	conn, zkEvents, err := zk.Connect(c.addrs, connectTimeout)
	if err != nil {
		return nil, nil, errors.Wrap(err, "connection failed")
	}
	if c.opts.Auth.Schema != "" || len(c.opts.Auth.Schema) > 0 {
		if err := conn.AddAuth(c.opts.Auth.Schema, c.opts.Auth.Secret); err != nil {
			return nil, nil, errors.Wrap(err, "authentication failed")
		}
	}
	c.conn = conn
	sessionWaitTimeout := durationOrDefault(c.opts.InitialSessionTimeout, defaultInitialSessionTimeout)
	if err := waitForSession(zkEvents, sessionWaitTimeout); err != nil {
		return nil, nil, errors.Wrap(err, "session could not be established")
	}
	return conn, zkEvents, nil
}

func (c *newConnection) Close() error {
	c.once.Do(func() {
		c.conn.Close()
	})
	return nil
}

// durationOrDefault returns the first duration unless it is the zero value,
// in which case it will return the defaultDuration.
func durationOrDefault(duration time.Duration, defaultDuration time.Duration) time.Duration {
	if duration != 0 {
		return duration
	}
	return defaultDuration
}

// waitForSession waits for a session to be established. if it times out
// an error will be returned.
func waitForSession(zkEvents <-chan zk.Event, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case e := <-zkEvents:
			if e.State == zk.StateHasSession {
				return nil
			}
		case <-deadline.C:
			return errors.New("timed out")
		}
	}
}
