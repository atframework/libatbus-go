package libatbus

import (
	types "github.com/atframework/libatbus-go/types"
)

type Endpoint = types.Endpoint

// CreateEndpoint creates an endpoint owned by the given node.
func CreateEndpoint(owner Node, id BusIdType, hostName string, pid int) Endpoint {
	if owner == nil {
		return nil
	}

	return owner.CreateEndpoint(id, hostName, pid)
}
