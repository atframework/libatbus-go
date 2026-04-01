package libatbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
)

func TestNewNodeCanBeInitialized(t *testing.T) {
	// Arrange
	node := NewNode()
	require.NotNil(t, node)

	// Act
	ret := node.Init(0x1234, nil)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, BusIdType(0x1234), node.GetId())
	assert.NotNil(t, node.GetSelfEndpoint())
}

func TestCreateNodeInitializesNode(t *testing.T) {
	// Arrange + Act
	node, ret := CreateNode(0x2234, nil)

	// Assert
	require.NotNil(t, node)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, BusIdType(0x2234), node.GetId())
	assert.NotNil(t, node.GetSelfEndpoint())
}

func TestRemoveEndpointByIDRemovesExistingEndpoint(t *testing.T) {
	// Arrange
	node, ret := CreateNode(0x3234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(node, 0x3235, "test-host", 4321)
	require.NotNil(t, ep)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.AddEndpoint(ep))

	// Act
	ret = node.RemoveEndpointByID(ep.GetId())

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Nil(t, node.GetEndpoint(ep.GetId()))
}

func TestRemoveEndpointByIDReturnsNotFoundWhenMissing(t *testing.T) {
	// Arrange
	node, ret := CreateNode(0x4234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	ret = node.RemoveEndpointByID(0xFFFF)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_NOT_FOUND, ret)
}

func TestRemoveEndpointByIDRejectsSelfID(t *testing.T) {
	// Arrange
	node, ret := CreateNode(0x5234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	ret = node.RemoveEndpointByID(node.GetId())

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, ret)
}
