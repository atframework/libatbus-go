// Package libatbus_impl provides internal implementation details for libatbus.
//
// This file contains Go equivalents of atbus_node_setup_test.cpp:
//   - override_listen_path
//   - crypto_algorithms
//   - compression_algorithms

package libatbus_impl

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

// TestNodeSetupParity_OverrideListenPath mirrors C++ atbus_node_setup::override_listen_path.
// On Unix it tests that overwrite_listen_path controls whether a second node
// can take over a Unix-socket listen path. On Windows, Unix sockets may not be
// fully supported so we skip.
func TestNodeSetupParity_OverrideListenPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket listen-path override test is not supported on Windows")
	}

	sockPath := "/tmp/atbus-go-unit-test-overwrite-unix.sock"
	t.Cleanup(func() {
		// Best-effort cleanup of leftover lock files.
		// The node Reset/Close should handle this, but be defensive.
	})

	// -- node1: overwrite_listen_path = false, listen on sockPath -- should succeed
	var conf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf1)
	conf1.OverwriteListenPath = false

	var node1 Node
	ret := node1.Init(0x12345678, &conf1)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node1.Listen("unix://" + sockPath)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"node1 should successfully listen on the socket path")

	// -- node2: overwrite_listen_path = false, same path -- should fail (path locked)
	var conf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf2)
	conf2.OverwriteListenPath = false

	var node2 Node
	ret = node2.Init(0x12356789, &conf2)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node2.Listen("unix://" + sockPath)
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"node2 should fail to listen on a locked path")

	// -- node3: overwrite_listen_path = true, same path -- should succeed
	var conf3 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf3)
	conf3.OverwriteListenPath = true

	var node3 Node
	ret = node3.Init(0x12367890, &conf3)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node3.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node3.Listen("unix://" + sockPath)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"node3 with overwrite_listen_path=true should take over the socket path")

	// Cleanup nodes
	node1.Reset()
	node2.Reset()
	node3.Reset()
}

// TestNodeSetupParity_CryptoAlgorithms mirrors C++ atbus_node_setup::crypto_algorithms.
// Go does not have parse_crypto_algorithm_name (enums are used directly);
// instead we verify the enum-to-string mapping and key/IV size helpers are
// consistent across all supported algorithms.
func TestNodeSetupParity_CryptoAlgorithms(t *testing.T) {
	algorithms := []struct {
		name    string
		enumVal protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		keySize int
		ivSize  int
		isAEAD  bool
		tagSize int
	}{
		{"XXTEA", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA, 16, 0, false, 0},
		{"CHACHA20", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20, 32, 12, false, 0},
		{"CHACHA20-POLY1305", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, 32, 12, true, 16},
		{"XCHACHA20-POLY1305", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF, 32, 24, true, 16},
		{"AES-128-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, 16, 16, false, 0},
		{"AES-128-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, 16, 12, true, 16},
		{"AES-192-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC, 24, 16, false, 0},
		{"AES-192-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, 24, 12, true, 16},
		{"AES-256-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, 32, 16, false, 0},
		{"AES-256-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, 32, 12, true, 16},
	}

	count := 0
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			// Verify string representation
			str := cryptoAlgorithmString(algo.enumVal)
			assert.NotEmpty(t, str)
			assert.NotEqual(t, "NONE", str, "Algorithm %s should have a non-NONE string representation", algo.name)

			// Verify key size
			assert.Equal(t, algo.keySize, cryptoAlgorithmKeySize(algo.enumVal),
				"Key size mismatch for %s", algo.name)

			// Verify IV size
			assert.Equal(t, algo.ivSize, cryptoAlgorithmIVSize(algo.enumVal),
				"IV size mismatch for %s", algo.name)

			// Verify AEAD property
			assert.Equal(t, algo.isAEAD, cryptoAlgorithmIsAEAD(algo.enumVal),
				"AEAD property mismatch for %s", algo.name)

			// Verify tag size
			assert.Equal(t, algo.tagSize, cryptoAlgorithmTagSize(algo.enumVal),
				"Tag size mismatch for %s", algo.name)
		})
		count++
	}

	assert.Greater(t, count, 0, "At least one crypto algorithm must be tested")
}

// TestNodeSetupParity_CompressionAlgorithms mirrors C++ atbus_node_setup::compression_algorithms.
// Go does not have parse_compression_algorithm_name; instead we verify the
// enum-to-string mapping and support-check for each known compression.
func TestNodeSetupParity_CompressionAlgorithms(t *testing.T) {
	algorithms := []struct {
		name    string
		enumVal protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
	}{
		{"ZSTD", protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD},
		{"LZ4", protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4},
		{"SNAPPY", protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY},
		{"ZLIB", protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB},
	}

	count := 0
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			// Verify string representation
			str := compressionAlgorithmString(algo.enumVal)
			assert.NotEmpty(t, str)
			assert.NotEqual(t, "NONE", str, "Compression %s should have a non-NONE string representation", algo.name)

			// Verify that the compression algorithm is recognized by the support check delegate
			assert.True(t, types.IsCompressionAlgorithmSupported(algo.enumVal),
				"Compression algorithm %s should be supported", algo.name)
		})
		count++
	}

	assert.Greater(t, count, 0, "At least one compression algorithm must be tested")

	// Note: In Go, NONE compression returns true from the support check delegate
	// (the underlying compression library treats NONE as a pass-through).
	// This is not a divergence from C++ behavior for practical purposes.
}

// TestNodeSetupParity_KeyExchangeAlgorithms verifies the key exchange algorithm
// string mapping and curve availability. This extends the C++ crypto_algorithms
// test coverage to key exchange types.
func TestNodeSetupParity_KeyExchangeAlgorithms(t *testing.T) {
	algorithms := []struct {
		name    string
		enumVal protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
	}{
		{"X25519", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519},
		{"SECP256R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1},
		{"SECP384R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1},
		{"SECP521R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1},
	}

	count := 0
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			str := keyExchangeString(algo.enumVal)
			assert.NotEmpty(t, str)
			assert.NotEqual(t, "NONE", str)

			curve := keyExchangeCurve(algo.enumVal)
			assert.NotNil(t, curve, "Key exchange %s should map to a valid ECDH curve", algo.name)
		})
		count++
	}

	assert.Greater(t, count, 0)

	// NONE should not map to any curve
	assert.Nil(t, keyExchangeCurve(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE))
}

// TestNodeSetupParity_ReloadCryptoAppliesConfig verifies ReloadCrypto correctly
// applies key exchange and allowed algorithm configuration.
func TestNodeSetupParity_ReloadCryptoAppliesConfig(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Default should be NONE
	assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE,
		n.GetCryptoKeyExchangeType())

	// Reload with X25519
	algs := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
	}
	ret = n.ReloadCrypto(
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		0,
		algs,
	)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		n.GetCryptoKeyExchangeType())

	// Reload with invalid type should fall back to NONE
	ret = n.ReloadCrypto(
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE(9999),
		0,
		algs,
	)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE,
		n.GetCryptoKeyExchangeType())
}
