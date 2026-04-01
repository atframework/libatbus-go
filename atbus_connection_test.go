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
	conn := CreateConnection(node, "ipv4://127.0.0.1:0")

	// Assert
	require.NotNil(t, conn)
	require.NotNil(t, conn.GetAddress())
	assert.Equal(t, "ipv4://127.0.0.1:0", conn.GetAddress().GetAddress())
}

func TestCreateConnectionNilOwnerReturnsNil(t *testing.T) {
	// Arrange + Act
	conn := CreateConnection(nil, "ipv4://127.0.0.1:0")

	// Assert
	assert.Nil(t, conn)
}

func TestCreateConnectionInvalidAddressReturnsNil(t *testing.T) {
	// Arrange
	node, ret := CreateNode(0x9234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	conn := CreateConnection(node, "not-a-valid-address")

	// Assert
	assert.Nil(t, conn)
}
