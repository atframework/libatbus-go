package libatbus

import (
	libatbus_impl "github.com/atframework/libatbus-go/impl"
	types "github.com/atframework/libatbus-go/types"
)

type (
	Connection        = types.Connection
	ConnectionContext = types.ConnectionContext
)

// CreateConnection creates a connection owned by the given node using the
// default libatbus implementation. The owner should be created by NewNode or
// CreateNode.
func CreateConnection(owner Node, addr string) Connection {
	implOwner := unwrapNode(owner)
	if implOwner == nil {
		return nil
	}

	return libatbus_impl.CreateConnection(implOwner, addr)
}
