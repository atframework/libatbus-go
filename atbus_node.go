package libatbus

import (
	libatbus_impl "github.com/atframework/libatbus-go/impl"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

type (
	BusIdType     = types.BusIdType
	NodeConfigure = types.NodeConfigure
	Node          = types.Node
)

func unwrapNode(owner Node) *libatbus_impl.Node {
	if owner == nil {
		return nil
	}

	implOwner, ok := owner.(*libatbus_impl.Node)
	if !ok {
		return nil
	}

	return implOwner
}

// NewNode creates a node instance using the default libatbus implementation.
// Call Init on the returned node before use.
func NewNode() Node {
	return &libatbus_impl.Node{}
}

// CreateNode creates and initializes a node using the default libatbus implementation.
func CreateNode(id BusIdType, conf *NodeConfigure) (Node, ErrorType) {
	ret := &libatbus_impl.Node{}
	errCode := ret.Init(id, conf)
	if errCode != 0 {
		return nil, errCode
	}

	return ret, errCode
}

// ParseCryptoAlgorithmName maps a libatbus C++ cipher name to the matching protocol enum.
func ParseCryptoAlgorithmName(name string) protocol.ATBUS_CRYPTO_ALGORITHM_TYPE {
	return types.ParseCryptoAlgorithmName(name)
}

// ParseCompressionAlgorithmName maps a libatbus C++ compression name to the matching protocol enum.
func ParseCompressionAlgorithmName(name string) protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE {
	return types.ParseCompressionAlgorithmName(name)
}
