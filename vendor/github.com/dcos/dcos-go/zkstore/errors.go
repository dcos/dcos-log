package zkstore

type internalError string

func (i internalError) Error() string { return string(i) }

var _ = error(internalError("")) // sanity check

const (
	// ErrIllegalOption is returned when a StoreOpt configuration is set w/ an illegal value.
	ErrIllegalOption = internalError("illegal option configuration")

	// ErrVersionConflict is returned when a specified ZKVersion is rejected by
	// ZK when performing a mutating operation on a znode.  Clients that receive
	// this can retry by re-reading the Item and then trying again.
	ErrVersionConflict = internalError("zk version conflict")

	// ErrNotFound is returned when an attempting to read a znode that does not exist.
	ErrNotFound = internalError("znode not found")

	errHashOverflow = internalError("hash value larger than 64 bits")

	errBadCategory = internalError("bad category name")
)
