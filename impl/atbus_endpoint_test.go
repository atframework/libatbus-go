// Package libatbus_impl provides internal implementation details for libatbus.
//
// Traceability for C++ atbus_endpoint_test.cpp:
//   - connection_basic / endpoint_basic / get_connection: covered here.
//   - is_child and related topology assertions: covered in atbus_topology_test.go.
//   - address parsing/schema handling: covered in channel/utility/channel_utility_test.go.
package libatbus_impl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

func TestCreateEndpointInitializesOwnerAndMetadata(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1001, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	ep := CreateEndpoint(&n, 0x2001, 3456, "endpoint-host")

	// Assert
	require.NotNil(t, ep)
	assert.Equal(t, types.Node(&n), ep.GetOwner())
	assert.Equal(t, types.BusIdType(0x2001), ep.GetId())
	assert.Equal(t, int32(3456), ep.GetPid())
	assert.Equal(t, "endpoint-host", ep.GetHostname())
}

func TestEndpointAddConnectionBindsAsControlConnection(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1002, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, 0x2002, 4567, "endpoint-host")
	require.NotNil(t, ep)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connected)

	// Act
	added := ep.AddConnection(conn, false)

	// Assert
	assert.True(t, added)
	assert.Equal(t, ep, conn.GetBinding())
	assert.Equal(t, conn, ep.ctrlConn)
}

func TestEndpointGetDataConnectionFallsBackToControlWhenEnabled(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1003, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	target := CreateEndpoint(&n, 0x2003, 5678, "endpoint-host")
	require.NotNil(t, target)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connected)
	require.True(t, target.AddConnection(conn, false))

	// Act
	result := n.self.GetDataConnection(target, true)

	// Assert
	assert.Equal(t, conn, result)
}
