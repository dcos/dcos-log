package zkstore

import (
	"github.com/samuel/go-zookeeper/zk"
)

// Connector specifies a way to connect to ZK
type Connector interface {
	// Connect returns a ZK connection
	Connect() (*zk.Conn, error)

	// Close should ensure the ZK connection is closed.
	Close() error
}
