package elector

import (
	"github.com/samuel/go-zookeeper/zk"
)

// Connector specifies a way to connect to ZK.
type Connector interface {
	// Connect returns a ZK connection and events channel
	Connect() (Conn, <-chan zk.Event, error)

	// Close should ensure the ZK connection is closed.
	Close() error
}

// Conn represents a connection to ZK.
type Conn interface {
	Get(path string) ([]byte, *zk.Stat, error)
	Exists(path string) (bool, *zk.Stat, error)
	Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error)
	CreateProtectedEphemeralSequential(path string, data []byte, acl []zk.ACL) (string, error)
	ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error)
}

// ConnAdapter represents a connection to ZK.
type ConnAdapter struct {
	GetF                                func(path string) ([]byte, *zk.Stat, error)
	ExistsF                             func(path string) (bool, *zk.Stat, error)
	CreateF                             func(path string, data []byte, flags int32, acl []zk.ACL) (string, error)
	CreateProtectedEphemeralSequentialF func(path string, data []byte, acl []zk.ACL) (string, error)
	ChildrenWF                          func(path string) ([]string, *zk.Stat, <-chan zk.Event, error)
}

// Get implements Conn.
func (c ConnAdapter) Get(path string) ([]byte, *zk.Stat, error) {
	return c.GetF(path)
}

// Exists implements Conn.
func (c ConnAdapter) Exists(path string) (bool, *zk.Stat, error) {
	return c.ExistsF(path)
}

// Create implements Conn.
func (c ConnAdapter) Create(path string, data []byte, flags int32, acl []zk.ACL) (string, error) {
	return c.CreateF(path, data, flags, acl)
}

// CreateProtectedEphemeralSequential implements Conn.
func (c ConnAdapter) CreateProtectedEphemeralSequential(path string, data []byte, acl []zk.ACL) (string, error) {
	return c.CreateProtectedEphemeralSequentialF(path, data, acl)
}

// ChildrenW implements Conn.
func (c ConnAdapter) ChildrenW(path string) ([]string, *zk.Stat, <-chan zk.Event, error) {
	return c.ChildrenWF(path)
}
