package elector

import "fmt"

// Event is sent on the Elector's Events() channel.
type Event struct {
	// Leader is true if the elector that produced it is the leader
	Leader bool

	// Err represents an error event. If this is non-nil, the other fields
	// in the event must be ignored, and most clients will want to
	// shut down if Err is non-nil, since leadership cannot be guaranteed
	// in that case.
	//
	// When an err is sent, the Elector should no longer be considered
	// usable.
	Err error
}

func (e Event) String() string {
	return fmt.Sprintf("{leader:%v err:%s}", e.Leader, e.Err)
}
