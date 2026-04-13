package libatbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
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

func TestParseCryptoAlgorithmNameMatchesCxxMappings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	}{
		{name: "xxtea", input: "xxtea", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA},
		{name: "chacha20", input: "chacha20", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20},
		{name: "chacha20 poly1305", input: "chacha20-poly1305-ietf", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF},
		{name: "xchacha20 poly1305", input: "xchacha20-poly1305-ietf", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF},
		{name: "aes 128 cbc", input: "aes-128-cbc", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC},
		{name: "aes 128 gcm", input: "aes-128-gcm", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM},
		{name: "aes 192 cbc", input: "aes-192-cbc", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC},
		{name: "aes 192 gcm", input: "aes-192-gcm", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM},
		{name: "aes 256 cbc", input: "aes-256-cbc", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC},
		{name: "aes 256 gcm", input: "aes-256-gcm", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM},
		{name: "case insensitive", input: "AES-256-GCM", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM},
		{name: "no whitespace trimming prefix", input: " aes-256-gcm", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE},
		{name: "no whitespace trimming suffix", input: "aes-256-gcm ", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE},
		{name: "empty", input: "", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE},
		{name: "unknown", input: "aes256gcm", expected: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ParseCryptoAlgorithmName(tc.input))
		})
	}
}

func TestParseCompressionAlgorithmNameMatchesCxxMappings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
	}{
		{name: "zstd", input: "zstd", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD},
		{name: "lz4", input: "lz4", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4},
		{name: "snappy", input: "snappy", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY},
		{name: "zlib", input: "zlib", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB},
		{name: "case insensitive", input: "SNAPPY", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY},
		{name: "no whitespace trimming prefix", input: " snappy", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE},
		{name: "no whitespace trimming suffix", input: "snappy ", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE},
		{name: "empty", input: "", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE},
		{name: "unknown", input: "gzip", expected: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, ParseCompressionAlgorithmName(tc.input))
		})
	}
}
