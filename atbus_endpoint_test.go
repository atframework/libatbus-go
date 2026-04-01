package libatbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
)

func TestCreateEndpointReturnsOwnedEndpoint(t *testing.T) {
	// Arrange
	node, ret := CreateNode(0x6234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	ep := CreateEndpoint(node, 0x6235, "endpoint-host", 2468)

	// Assert
	require.NotNil(t, ep)
	assert.Equal(t, node, ep.GetOwner())
	assert.Equal(t, BusIdType(0x6235), ep.GetId())
	assert.Equal(t, int32(2468), ep.GetPid())
	assert.Equal(t, "endpoint-host", ep.GetHostname())
}

func TestCreateEndpointNilOwnerReturnsNil(t *testing.T) {
	// Arrange + Act
	ep := CreateEndpoint(nil, 0x7235, "endpoint-host", 2468)

	// Assert
	assert.Nil(t, ep)
}
