package zkstore

import "github.com/samuel/go-zookeeper/zk"

// ExistingConnection returns an existing connection.
//
// The existing connection should have already established a session before
// calling this method
func ExistingConnection(conn *zk.Conn) Connector {
	return &existingConnection{
		conn: conn,
	}
}

type existingConnection struct {
	conn *zk.Conn
}

func (e *existingConnection) Connect() (*zk.Conn, error) {
	return e.conn, nil
}

func (e *existingConnection) Close() error {
	// do nothing because it's not our connection
	return nil
}
