package libatbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
)

func TestCreateConnectionReturnsConnectionForValidOwnerAndAddress(t *testing.T) {
	// Arrange
	node, ret := CreateNode(0x8234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	conn := CreateConnection(node, "ipv4://127.0.0.1:0", false)

	// Assert
	require.NotNil(t, conn)
	require.NotNil(t, conn.GetAddress())
	assert.Equal(t, "ipv4://127.0.0.1:0", conn.GetAddress().GetAddress())
	assert.Equal(t, "ipv4://127.0.0.1:0", conn.GetOriginAddress())
}

func TestCreateConnectionNilOwnerReturnsNil(t *testing.T) {
	// Arrange + Act
	conn := CreateConnection(nil, "ipv4://127.0.0.1:0", false)

	// Assert
	assert.Nil(t, conn)
}

func TestCreateConnectionLegacyWildcardListener(t *testing.T) {
	node, code := CreateNode(1, nil)
	require.Zero(t, code)
	t.Cleanup(func() { node.Reset() })
	conn := CreateConnection(node, "ipv4://0.0.0.0:0")
	require.NotNil(t, conn)
	assert.Equal(t, "0.0.0.0", conn.GetAddress().GetHost())
	require.Zero(t, conn.Listen())
	assert.Equal(t, "0.0.0.0", conn.GetAddress().GetHost())
	assert.Zero(t, node.GetConnectionTimerSize())
}

func TestCreateConnectionInvalidAddressReturnsNil(t *testing.T) {
	// Arrange
	node, ret := CreateNode(0x9234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	conn := CreateConnection(node, "not-a-valid-address", false)

	// Assert
	assert.Nil(t, conn)
}
