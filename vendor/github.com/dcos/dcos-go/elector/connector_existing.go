package elector

import "github.com/samuel/go-zookeeper/zk"

// ExistingConnection returns an existing connection.  Since it consumes
// from the zk.Event channel, it might be necessary for the client to fan out
// incoming events on the original channel to a new one so that events are not
// lost.
//
// The existing connection should have already established a session before
// calling this method
func ExistingConnection(conn Conn, events <-chan zk.Event) Connector {
	return &existingConnection{
		conn:   conn,
		events: events,
	}
}

type existingConnection struct {
	conn   Conn
	events <-chan zk.Event
}

func (e *existingConnection) Connect() (Conn, <-chan zk.Event, error) {
	return e.conn, e.events, nil
}

func (e *existingConnection) Close() error {
	// do nothing because it's not our connection
	return nil
}
