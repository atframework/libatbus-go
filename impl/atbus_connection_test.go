package libatbus_impl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

func TestCreateConnectionInitializesAddressAndContext(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3001, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")

	// Assert
	require.NotNil(t, conn)
	require.NotNil(t, conn.GetAddress())
	assert.Equal(t, "ipv4://127.0.0.1:0", conn.GetAddress().GetAddress())
	assert.Equal(t, types.ConnectionState_Disconnected, conn.GetStatus())
	assert.NotNil(t, conn.GetConnectionContext())
}

func TestConnectionSetTemporarySetsTemporaryFlag(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3002, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	// Act
	conn.SetTemporary()

	// Assert
	assert.True(t, conn.CheckFlag(types.ConnectionFlag_Temporary))
}

func TestConnectionIsRunningReflectsStatus(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3003, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	// Act + Assert
	conn.setStatus(types.ConnectionState_Disconnected)
	assert.False(t, conn.IsRunning())

	conn.setStatus(types.ConnectionState_Connecting)
	assert.True(t, conn.IsRunning())

	conn.setStatus(types.ConnectionState_Handshaking)
	assert.True(t, conn.IsRunning())

	conn.setStatus(types.ConnectionState_Connected)
	assert.True(t, conn.IsRunning())

	conn.setStatus(types.ConnectionState_Disconnecting)
	assert.False(t, conn.IsRunning())
}
