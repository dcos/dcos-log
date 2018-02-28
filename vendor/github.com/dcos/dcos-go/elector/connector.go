package elector

import (
	"github.com/samuel/go-zookeeper/zk"
)

// Connector specifies a way to connect to ZK
type Connector interface {
	// Connect returns a ZK connection and events channel
	Connect() (*zk.Conn, <-chan zk.Event, error)

	// Close should ensure the ZK connection is closed.
	Close() error
}
