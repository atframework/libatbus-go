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
// CreateNode. An explicit mode normalizes the address immediately (true for
// listening). Without a mode, Listen or Connect selects the address.
func CreateConnection(owner Node, addr string, listenMode ...bool) Connection {
	implOwner := unwrapNode(owner)
	if implOwner == nil {
		return nil
	}

	conn := libatbus_impl.CreateConnection(implOwner, addr, listenMode...)
	if conn == nil {
		return nil
	}
	return conn
}
