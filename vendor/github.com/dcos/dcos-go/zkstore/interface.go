package zkstore

// IStore is the interface to which Store confirms. Documentation for these
// methods lives on the concrete Store type, as the NewStore method returns
// a concrete type, not this interface.  Clients may choose to use the IStore
// interface if they wish.
type IStore interface {
	Put(item Item) (Ident, error)
	Get(ident Ident) (item Item, err error)
	List(category string) (locations []Location, err error)
	Variants(location Location) (variants []string, err error)
	Delete(ident Ident) error
	Close() error
}

// ensure that Store confirms to the IStore interface.
var _ IStore = &Store{}
