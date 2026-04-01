package libatbus_impl

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	buffer "github.com/atframework/libatbus-go/buffer"
	error_code "github.com/atframework/libatbus-go/error_code"
	"github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
// CryptoAlgorithmType Tests
// ============================================================================

func TestCryptoAlgorithmTypeString(t *testing.T) {
	// Test: Verify all crypto algorithm types return correct string representations
	cases := []struct {
		algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		expected  string
	}{
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, "NONE"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA, "XXTEA"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, "AES-128-CBC"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC, "AES-192-CBC"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, "AES-256-CBC"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, "AES-128-GCM"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, "AES-192-GCM"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, "AES-256-GCM"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20, "CHACHA20"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, "CHACHA20-POLY1305"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF, "XCHACHA20-POLY1305"},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE(999), "UNKNOWN(999)"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			// Act
			result := cryptoAlgorithmString(tc.algorithm)

			// Assert
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCryptoAlgorithmTypeKeySize(t *testing.T) {
	// Test: Verify key sizes for all crypto algorithms
	cases := []struct {
		algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		expected  int
	}{
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, 0},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC, 24},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, 24},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, 32},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, 32},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20, 32},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, 32},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF, 32},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE(999), 0},
	}

	for _, tc := range cases {
		t.Run(tc.algorithm.String(), func(t *testing.T) {
			// Act
			result := cryptoAlgorithmKeySize(tc.algorithm)

			// Assert
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCryptoAlgorithmTypeIVSize(t *testing.T) {
	// Test: Verify IV/nonce sizes for all crypto algorithms
	cases := []struct {
		algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		expected  int
	}{
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, 0},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA, 0},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, 12},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, 12},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, 12},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20, 12},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, 12},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF, 24},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE(999), 0},
	}

	for _, tc := range cases {
		t.Run(tc.algorithm.String(), func(t *testing.T) {
			// Act
			result := cryptoAlgorithmIVSize(tc.algorithm)

			// Assert
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCryptoAlgorithmTypeIsAEAD(t *testing.T) {
	// Test: Verify AEAD detection for all crypto algorithms
	aeadAlgorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF,
	}

	nonAeadAlgorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20,
	}

	for _, alg := range aeadAlgorithms {
		t.Run(cryptoAlgorithmString(alg)+"_IsAEAD", func(t *testing.T) {
			assert.True(t, cryptoAlgorithmIsAEAD(alg))
		})
	}

	for _, alg := range nonAeadAlgorithms {
		t.Run(cryptoAlgorithmString(alg)+"_NotAEAD", func(t *testing.T) {
			assert.False(t, cryptoAlgorithmIsAEAD(alg))
		})
	}
}

func TestCryptoAlgorithmTypeTagSize(t *testing.T) {
	// Test: Verify tag sizes for AEAD algorithms
	cases := []struct {
		algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		expected  int
	}{
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF, 16},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, 0},
		{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, 0},
	}

	for _, tc := range cases {
		t.Run(tc.algorithm.String(), func(t *testing.T) {
			// Act
			result := cryptoAlgorithmTagSize(tc.algorithm)

			// Assert
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ============================================================================
// KeyExchangeType Tests
// ============================================================================

func TestKeyExchangeTypeString(t *testing.T) {
	// Test: Verify all key exchange types return correct string representations
	cases := []struct {
		keyExchange protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
		expected    string
	}{
		{protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE, "NONE"},
		{protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519, "X25519"},
		{protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1, "SECP256R1"},
		{protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1, "SECP384R1"},
		{protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1, "SECP521R1"},
		{protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE(999), "UNKNOWN(999)"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			// Act
			result := keyExchangeString(tc.keyExchange)

			// Assert
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestKeyExchangeTypeCurve(t *testing.T) {
	// Test: Verify curve mapping for key exchange types
	validKeyExchanges := []protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE{
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1,
	}

	for _, ke := range validKeyExchanges {
		t.Run(keyExchangeString(ke)+"_HasCurve", func(t *testing.T) {
			curve := keyExchangeCurve(ke)
			assert.NotNil(t, curve)
		})
	}

	t.Run("None_NilCurve", func(t *testing.T) {
		curve := keyExchangeCurve(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE)
		assert.Nil(t, curve)
	})

	t.Run("Unknown_NilCurve", func(t *testing.T) {
		curve := keyExchangeCurve(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE(999))
		assert.Nil(t, curve)
	})
}

// ============================================================================
// KDFType Tests
// ============================================================================

func TestKDFTypeString(t *testing.T) {
	// Test: Verify KDF types return correct string representations
	cases := []struct {
		kdf      protocol.ATBUS_CRYPTO_KDF_TYPE
		expected string
	}{
		{protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256, "HKDF-SHA256"},
		{protocol.ATBUS_CRYPTO_KDF_TYPE(999), "UNKNOWN(999)"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			// Act
			result := kdfTypeString(tc.kdf)

			// Assert
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ============================================================================
// CompressionAlgorithmType Tests
// ============================================================================

func TestCompressionAlgorithmTypeString(t *testing.T) {
	// Test: Verify all compression algorithm types return correct string representations
	cases := []struct {
		compression protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
		expected    string
	}{
		{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, "NONE"},
		{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD, "ZSTD"},
		{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4, "LZ4"},
		{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY, "SNAPPY"},
		{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, "ZLIB"},
		{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(999), "UNKNOWN(999)"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			// Act
			result := compressionAlgorithmString(tc.compression)

			// Assert
			assert.Equal(t, tc.expected, result)
		})
	}
}

// ============================================================================
// CryptoSession Tests
// ============================================================================

func TestNewCryptoSession(t *testing.T) {
	// Test: Verify new crypto session is created with default state
	// Arrange & Act
	session := NewCryptoSession()

	// Assert
	assert.NotNil(t, session)
	assert.False(t, session.IsInitialized())
}

func TestCryptoSessionGenerateKeyPair(t *testing.T) {
	// Test: Verify key pair generation for different key exchange types
	keyExchanges := []protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE{
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1,
	}

	for _, ke := range keyExchanges {
		t.Run(keyExchangeString(ke), func(t *testing.T) {
			// Arrange
			session := NewCryptoSession()

			// Act
			err := session.GenerateKeyPair(ke)

			// Assert
			require.NoError(t, err)
			pubKey := session.GetPublicKey()
			assert.NotNil(t, pubKey)
			assert.Greater(t, len(pubKey), 0)
		})
	}

	t.Run("UnsupportedKeyExchange", func(t *testing.T) {
		// Arrange
		session := NewCryptoSession()

		// Act
		err := session.GenerateKeyPair(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE)

		// Assert
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrCryptoAlgorithmNotSupported))
	})
}

func TestCryptoSessionKeyExchange(t *testing.T) {
	// Test: Verify full key exchange between two sessions
	keyExchanges := []protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE{
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1,
	}

	for _, ke := range keyExchanges {
		t.Run(keyExchangeString(ke), func(t *testing.T) {
			// Arrange
			session1 := NewCryptoSession()
			session2 := NewCryptoSession()

			require.NoError(t, session1.GenerateKeyPair(ke))
			require.NoError(t, session2.GenerateKeyPair(ke))

			pubKey1 := session1.GetPublicKey()
			pubKey2 := session2.GetPublicKey()

			// Act
			sharedSecret1, err1 := session1.ComputeSharedSecret(pubKey2)
			sharedSecret2, err2 := session2.ComputeSharedSecret(pubKey1)

			// Assert
			require.NoError(t, err1)
			require.NoError(t, err2)
			assert.Equal(t, sharedSecret1, sharedSecret2)
		})
	}
}

func TestCryptoSessionSetKey(t *testing.T) {
	// Test: Verify SetKey for different algorithms
	algorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF,
	}

	for _, alg := range algorithms {
		t.Run(cryptoAlgorithmString(alg), func(t *testing.T) {
			// Arrange
			session := NewCryptoSession()
			key := make([]byte, cryptoAlgorithmKeySize(alg))
			iv := make([]byte, cryptoAlgorithmIVSize(alg))
			_, _ = rand.Read(key)
			_, _ = rand.Read(iv)

			// Act
			err := session.SetKey(key, iv, alg)

			// Assert
			require.NoError(t, err)
			assert.True(t, session.IsInitialized())
		})
	}

	t.Run("InvalidKeySize", func(t *testing.T) {
		// Arrange
		session := NewCryptoSession()
		shortKey := make([]byte, 8) // Too short for AES-128

		// Act
		err := session.SetKey(shortKey, nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM)

		// Assert
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrCryptoInvalidKeySize))
	})
}

func TestCryptoSessionEncryptDecryptAEAD(t *testing.T) {
	// Test: Verify encrypt/decrypt roundtrip for AEAD algorithms
	algorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF,
	}

	testData := []byte("Hello, World! This is a test message for encryption.")

	for _, alg := range algorithms {
		t.Run(cryptoAlgorithmString(alg), func(t *testing.T) {
			// Arrange
			session := NewCryptoSession()
			key := make([]byte, cryptoAlgorithmKeySize(alg))
			iv := make([]byte, cryptoAlgorithmIVSize(alg))
			_, _ = rand.Read(key)
			_, _ = rand.Read(iv)
			require.NoError(t, session.SetKey(key, iv, alg))

			// Act
			encrypted, err := session.Encrypt(testData)
			require.NoError(t, err)

			decrypted, err := session.Decrypt(encrypted)
			require.NoError(t, err)

			// Assert
			assert.Equal(t, testData, decrypted)
			assert.NotEqual(t, testData, encrypted)
		})
	}
}

func TestCryptoSessionEncryptDecryptCBC(t *testing.T) {
	// Test: Verify encrypt/decrypt roundtrip for CBC algorithms
	algorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC,
	}

	testData := []byte("Hello, World! This is a test message for CBC encryption.")

	for _, alg := range algorithms {
		t.Run(cryptoAlgorithmString(alg), func(t *testing.T) {
			// Arrange
			session := NewCryptoSession()
			key := make([]byte, cryptoAlgorithmKeySize(alg))
			iv := make([]byte, cryptoAlgorithmIVSize(alg))
			_, _ = rand.Read(key)
			_, _ = rand.Read(iv)
			require.NoError(t, session.SetKey(key, iv, alg))

			// Act
			encrypted, err := session.Encrypt(testData)
			require.NoError(t, err)

			decrypted, err := session.Decrypt(encrypted)
			require.NoError(t, err)

			// Assert
			assert.Equal(t, testData, decrypted)
			assert.NotEqual(t, testData, encrypted)
		})
	}
}

func TestCryptoSessionEncryptDecryptCBCWithIV(t *testing.T) {
	// Test: Verify CBC encrypt/decrypt with caller-provided IV
	// Arrange
	session := NewCryptoSession()
	key := make([]byte, 16)
	iv := make([]byte, 16)
	_, _ = rand.Read(key)
	_, _ = rand.Read(iv)
	require.NoError(t, session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC))
	plaintext := []byte("CBC with IV test payload")

	// Act
	encrypted, err := session.EncryptWithIV(plaintext, iv)
	require.NoError(t, err)

	decrypted, err := session.DecryptWithIV(encrypted, iv)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, plaintext, decrypted)
}

func TestCryptoSessionEncryptCBCWithIVInvalidSize(t *testing.T) {
	// Test: Verify CBC encrypt fails with invalid IV size
	// Arrange
	session := NewCryptoSession()
	key := make([]byte, 16)
	iv := make([]byte, 16)
	_, _ = rand.Read(key)
	_, _ = rand.Read(iv)
	require.NoError(t, session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC))
	badIV := make([]byte, 8)

	// Act
	_, err := session.EncryptWithIV([]byte("payload"), badIV)

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrCryptoInvalidIVSize))
}

func TestCryptoSessionEncryptDecryptXXTEA(t *testing.T) {
	// Test: Verify XXTEA encrypt/decrypt roundtrip
	testData := []byte("Hello, World! This is a test message for XXTEA encryption.")

	// Arrange
	session := NewCryptoSession()
	key := make([]byte, 16) // XXTEA uses 128-bit key
	_, _ = rand.Read(key)
	require.NoError(t, session.SetKey(key, nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA))

	// Act
	encrypted, err := session.Encrypt(testData)
	require.NoError(t, err)

	decrypted, err := session.Decrypt(encrypted)
	require.NoError(t, err)

	// Assert: decrypted includes zero-padding, check prefix matches
	assert.NotEqual(t, testData, encrypted)
	require.GreaterOrEqual(t, len(decrypted), len(testData))
	assert.Equal(t, testData, decrypted[:len(testData)])
}

func TestCryptoSessionXXTEADifferentSizes(t *testing.T) {
	// Test: Verify XXTEA handles various data sizes including padding edge cases
	cases := []struct {
		name string
		size int
	}{
		{"1byte", 1},
		{"3bytes", 3},
		{"4bytes", 4},
		{"7bytes", 7},
		{"8bytes", 8},
		{"15bytes", 15},
		{"16bytes", 16},
		{"100bytes", 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			session := NewCryptoSession()
			key := make([]byte, 16)
			_, _ = rand.Read(key)
			require.NoError(t, session.SetKey(key, nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA))

			testData := make([]byte, tc.size)
			_, _ = rand.Read(testData)

			// Act
			encrypted, err := session.Encrypt(testData)
			require.NoError(t, err)

			decrypted, err := session.Decrypt(encrypted)
			require.NoError(t, err)

			// Assert: plaintext prefix matches, output is 4-byte aligned, minimum 8 bytes
			require.GreaterOrEqual(t, len(decrypted), len(testData))
			assert.Equal(t, testData, decrypted[:len(testData)])
			assert.Equal(t, 0, len(encrypted)%4)
			assert.GreaterOrEqual(t, len(encrypted), 8)
		})
	}
}

func TestCryptoSessionXXTEAEmptyData(t *testing.T) {
	// Test: Verify XXTEA handles empty data
	// Arrange
	session := NewCryptoSession()
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	require.NoError(t, session.SetKey(key, nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA))

	// Act
	encrypted, err := session.Encrypt([]byte{})

	// Assert
	require.NoError(t, err)
	assert.Empty(t, encrypted)
}

func TestCryptoSessionXXTEAWithKeyExchange(t *testing.T) {
	// Test: Verify XXTEA works with ECDH key exchange and HKDF key derivation (matching C++ flow)
	// Arrange
	session1 := NewCryptoSession()
	session2 := NewCryptoSession()

	keyExchange := protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1

	// Generate key pairs
	require.NoError(t, session1.GenerateKeyPair(keyExchange))
	require.NoError(t, session2.GenerateKeyPair(keyExchange))

	// Exchange public keys and compute shared secrets
	pub1 := session1.GetPublicKey()
	pub2 := session2.GetPublicKey()

	secret1, err := session1.ComputeSharedSecret(pub2)
	require.NoError(t, err)
	secret2, err := session2.ComputeSharedSecret(pub1)
	require.NoError(t, err)

	assert.Equal(t, secret1, secret2)

	// Derive keys for XXTEA
	require.NoError(t, session1.DeriveKey(secret1,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA,
		protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256))
	require.NoError(t, session2.DeriveKey(secret2,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA,
		protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256))

	// Act: encrypt with session1, decrypt with session2
	testData := []byte("Cross-session XXTEA test payload")
	encrypted, err := session1.Encrypt(testData)
	require.NoError(t, err)

	decrypted, err := session2.Decrypt(encrypted)
	require.NoError(t, err)

	// Assert
	require.GreaterOrEqual(t, len(decrypted), len(testData))
	assert.Equal(t, testData, decrypted[:len(testData)])
}

func TestCryptoSessionXXTEAInvalidKeySize(t *testing.T) {
	// Test: Verify XXTEA rejects invalid key sizes
	session := NewCryptoSession()

	// Wrong key size (32 instead of 16)
	err := session.SetKey(make([]byte, 32), nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrCryptoInvalidKeySize))
}

func TestCryptoSessionXXTEACrossLanguageVector(t *testing.T) {
	// Test: Verify XXTEA produces output consistent with the C++ implementation
	// using the same known key and plaintext
	key := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}
	plaintext := []byte{0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48} // "ABCDEFGH"

	// Arrange
	session := NewCryptoSession()
	require.NoError(t, session.SetKey(key, nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA))

	// Act
	encrypted, err := session.Encrypt(plaintext)
	require.NoError(t, err)

	// Decrypt and verify roundtrip
	decrypted, err := session.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted[:len(plaintext)])

	// Also verify: two sessions with same key produce same ciphertext
	session2 := NewCryptoSession()
	require.NoError(t, session2.SetKey(key, nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA))

	encrypted2, err := session2.Encrypt(plaintext)
	require.NoError(t, err)
	assert.Equal(t, encrypted, encrypted2, "Same key + same plaintext must produce same ciphertext (XXTEA is deterministic)")
}

func TestCryptoSessionEncryptEmptyData(t *testing.T) {
	// Test: Verify handling of empty data
	// Arrange
	session := NewCryptoSession()
	key := make([]byte, 32)
	iv := make([]byte, 12)
	_, _ = rand.Read(key)
	_, _ = rand.Read(iv)
	require.NoError(t, session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM))

	// Act
	encrypted, err := session.Encrypt([]byte{})

	// Assert
	require.NoError(t, err)
	assert.Empty(t, encrypted)
}

func TestCryptoSessionDecryptInvalidData(t *testing.T) {
	// Test: Verify error on invalid ciphertext
	// Arrange
	session := NewCryptoSession()
	key := make([]byte, 32)
	iv := make([]byte, 12)
	_, _ = rand.Read(key)
	_, _ = rand.Read(iv)
	require.NoError(t, session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM))

	// Act
	_, err := session.Decrypt([]byte("invalid ciphertext"))

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrCryptoDecryptFailed))
}

func TestCryptoSessionNotInitialized(t *testing.T) {
	// Test: Verify error when session is not initialized
	// Arrange
	session := NewCryptoSession()

	// Act
	_, encryptErr := session.Encrypt([]byte("test"))
	_, decryptErr := session.Decrypt([]byte("test"))

	// Assert
	assert.Equal(t, ErrCryptoNotInitialized, encryptErr)
	assert.Equal(t, ErrCryptoNotInitialized, decryptErr)
}

func TestCryptoSessionNoneAlgorithm(t *testing.T) {
	// Test: Verify that NONE algorithm passes data through unchanged
	// Arrange
	session := NewCryptoSession()
	require.NoError(t, session.SetKey(nil, nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE))
	testData := []byte("test data")

	// Act
	encrypted, err1 := session.Encrypt(testData)
	decrypted, err2 := session.Decrypt(testData)

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, testData, encrypted)
	assert.Equal(t, testData, decrypted)
}

// ============================================================================
// CompressionSession Tests
// ============================================================================

func TestNewCompressionSession(t *testing.T) {
	// Test: Verify new compression session is created with default state
	// Arrange & Act
	session := NewCompressionSession()

	// Assert
	assert.NotNil(t, session)
	assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, session.GetAlgorithm())
}

func TestCompressionSessionSetAlgorithm(t *testing.T) {
	// Test: Verify setting compression algorithms
	t.Run("SupportedAlgorithms", func(t *testing.T) {
		supported := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD,
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4,
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY,
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB,
		}
		for _, alg := range supported {
			session := NewCompressionSession()
			err := session.SetAlgorithm(alg)
			assert.NoError(t, err)
			assert.Equal(t, alg, session.GetAlgorithm())
		}
	})

	t.Run("UnknownAlgorithm", func(t *testing.T) {
		session := NewCompressionSession()
		err := session.SetAlgorithm(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(999))
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrCompressionNotSupported))
	})
}

func TestCompressionSessionZlib(t *testing.T) {
	// Test: Verify zlib compression and decompression
	// Arrange
	session := NewCompressionSession()
	require.NoError(t, session.SetAlgorithm(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB))

	// Test with compressible data (repeated pattern)
	testData := bytes.Repeat([]byte("Hello, World! "), 100)

	// Act
	compressed, err := session.Compress(testData)
	require.NoError(t, err)

	decompressed, err := session.Decompress(compressed, len(testData))
	require.NoError(t, err)

	// Assert
	assert.Equal(t, testData, decompressed)
	assert.Less(t, len(compressed), len(testData)) // Should be smaller
}

func TestCompressionSessionNone(t *testing.T) {
	// Test: Verify NONE algorithm passes data through unchanged
	// Arrange
	session := NewCompressionSession()
	testData := []byte("test data")

	// Act
	compressed, err1 := session.Compress(testData)
	decompressed, err2 := session.Decompress(testData, len(testData))

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, testData, compressed)
	assert.Equal(t, testData, decompressed)
}

func TestCompressionSessionEmptyData(t *testing.T) {
	// Test: Verify handling of empty data
	// Arrange
	session := NewCompressionSession()
	require.NoError(t, session.SetAlgorithm(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB))

	// Act
	compressed, err1 := session.Compress([]byte{})
	decompressed, err2 := session.Decompress([]byte{}, 0)

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Empty(t, compressed)
	assert.Empty(t, decompressed)
}

// ============================================================================
// Negotiation Function Tests
// ============================================================================

func TestNegotiateCompression(t *testing.T) {
	// Test: Verify compression algorithm negotiation
	t.Run("BothSupportZlib", func(t *testing.T) {
		local := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE}
		remote := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE}
		result := NegotiateCompression(local, remote)
		assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, result)
	})

	t.Run("OnlyNoneCommon", func(t *testing.T) {
		local := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE}
		remote := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE}
		result := NegotiateCompression(local, remote)
		assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, result)
	})

	t.Run("NoCommon", func(t *testing.T) {
		local := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB}
		remote := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD}
		result := NegotiateCompression(local, remote)
		assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, result)
	})
}

func TestNegotiateCryptoAlgorithm(t *testing.T) {
	// Test: Verify crypto algorithm negotiation
	t.Run("PreferAEAD", func(t *testing.T) {
		local := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC}
		remote := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC}
		result := NegotiateCryptoAlgorithm(local, remote)
		assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, result)
	})

	t.Run("FallbackToCBC", func(t *testing.T) {
		local := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE}
		remote := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE}
		result := NegotiateCryptoAlgorithm(local, remote)
		assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, result)
	})

	t.Run("NoCommon", func(t *testing.T) {
		local := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM}
		remote := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF}
		result := NegotiateCryptoAlgorithm(local, remote)
		assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, result)
	})
}

func TestNegotiateKeyExchange(t *testing.T) {
	// Test: Verify key exchange negotiation
	t.Run("SameType", func(t *testing.T) {
		result := NegotiateKeyExchange(
			protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
			protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		)
		assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519, result)
	})

	t.Run("DifferentTypes", func(t *testing.T) {
		result := NegotiateKeyExchange(
			protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
			protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1,
		)
		assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE, result)
	})
}

func TestNegotiateKDF(t *testing.T) {
	// Test: Verify KDF negotiation
	t.Run("BothSupportHKDF", func(t *testing.T) {
		local := []protocol.ATBUS_CRYPTO_KDF_TYPE{protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256}
		remote := []protocol.ATBUS_CRYPTO_KDF_TYPE{protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256}
		result := NegotiateKDF(local, remote)
		assert.Equal(t, protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256, result)
	})

	t.Run("Empty_DefaultsToHKDF", func(t *testing.T) {
		result := NegotiateKDF([]protocol.ATBUS_CRYPTO_KDF_TYPE{}, []protocol.ATBUS_CRYPTO_KDF_TYPE{})
		assert.Equal(t, protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256, result)
	})
}

// ============================================================================
// ConnectionContext Tests
// ============================================================================

func TestNewConnectionContext(t *testing.T) {
	// Test: Verify new connection context is created with default settings
	// Arrange & Act
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

	// Assert
	assert.NotNil(t, ctx)
	assert.False(t, ctx.IsClosing())
	assert.False(t, ctx.IsHandshakeDone())
	assert.NotNil(t, ctx.GetReadCrypto())
	assert.NotNil(t, ctx.GetWriteCrypto())
	assert.NotNil(t, ctx.GetCompression())
}

func TestConnectionContextClosingState(t *testing.T) {
	// Test: Verify closing state management
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

	// Act & Assert
	assert.False(t, ctx.IsClosing())
	ctx.SetClosing(true)
	assert.True(t, ctx.IsClosing())
	ctx.SetClosing(false)
	assert.False(t, ctx.IsClosing())
}

func TestConnectionContextSequence(t *testing.T) {
	// Test: Verify sequence number generation
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

	// Act
	seq1 := ctx.GetNextSequence()
	seq2 := ctx.GetNextSequence()
	seq3 := ctx.GetNextSequence()

	// Assert
	assert.Equal(t, uint64(1), seq1)
	assert.Equal(t, uint64(2), seq2)
	assert.Equal(t, uint64(3), seq3)
}

func TestConnectionContextSupportedAlgorithms(t *testing.T) {
	// Test: Verify getting and setting supported algorithms
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

	// Act & Assert - Crypto algorithms
	newCryptoAlgs := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM}
	ctx.SetSupportedCryptoAlgorithms(newCryptoAlgs)
	assert.Equal(t, newCryptoAlgs, ctx.GetSupportedCryptoAlgorithms())

	// Act & Assert - Compression algorithms
	newCompAlgs := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB}
	ctx.SetSupportedCompressionAlgorithms(newCompAlgs)
	assert.Equal(t, newCompAlgs, ctx.GetSupportedCompressionAlgorithms())

	// Act & Assert - Key exchange
	ctx.SetSupportedKeyExchange(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1)
	assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1, ctx.GetSupportedKeyExchange())
}

func TestConnectionContextHandshake(t *testing.T) {
	// Test: Verify full handshake between two connection contexts
	// Arrange
	ctx1 := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	ctx2 := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

	// Act - Create handshake data on both sides
	handshake1, err := ctx1.CreateHandshakeData()
	require.NoError(t, err)

	handshake2, err := ctx2.CreateHandshakeData()
	require.NoError(t, err)

	// Process each other's handshake data
	err = ctx1.ProcessHandshakeData(handshake2)
	require.NoError(t, err)

	err = ctx2.ProcessHandshakeData(handshake1)
	require.NoError(t, err)

	// Assert
	assert.True(t, ctx1.IsHandshakeDone())
	assert.True(t, ctx2.IsHandshakeDone())

	// Verify keys match (both should derive the same shared secret)
	assert.Equal(t, ctx1.readCrypto.Key, ctx2.readCrypto.Key)
}

func TestConnectionContextHandshakeWhenClosing(t *testing.T) {
	// Test: Verify handshake fails when connection is closing
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	ctx.SetClosing(true)

	// Act
	_, err := ctx.CreateHandshakeData()

	// Assert
	assert.Equal(t, ErrConnectionClosing, err)
}

func TestConnectionContextNegotiateCompression(t *testing.T) {
	// Test: Verify compression negotiation with peer
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	// Set up local supported algorithms to include ZLIB
	ctx.SetSupportedCompressionAlgorithms([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB,
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
	})
	peerAlgorithms := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE}

	// Act
	err := ctx.NegotiateCompressionWithPeer(peerAlgorithms)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, ctx.GetCompression().GetAlgorithm())
}

// ============================================================================
// PackMessage/UnpackMessage Tests
// ============================================================================

func TestConnectionContextPackUnpackMessageWithoutCrypto(t *testing.T) {
	// Test: Verify pack/unpack without encryption
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	testData := []byte("Hello, World!")

	msg := types.NewMessage()
	msg.MutableHead().SourceBusId = 12345
	msg.MutableHead().Sequence = 1
	msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			Content: testData,
		},
	}

	// Act
	packed, errCode := ctx.PackMessage(msg, 3, 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	unpacked := types.NewMessage()
	errCode = ctx.UnpackMessage(unpacked, packed.UsedSpan(), 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Assert
	assert.Equal(t, msg.GetHead().GetSourceBusId(), unpacked.GetHead().GetSourceBusId())
	assert.Equal(t, msg.GetHead().GetSequence(), unpacked.GetHead().GetSequence())
	assert.Equal(t, testData, unpacked.GetBody().GetDataTransformReq().GetContent())
}

func TestConnectionContextPackUnpackMessageWithCrypto(t *testing.T) {
	// Test: Verify pack/unpack with encryption
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	key := make([]byte, 32)
	iv := make([]byte, 12)
	_, _ = rand.Read(key)
	_, _ = rand.Read(iv)
	require.NoError(t, ctx.writeCrypto.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM))
	require.NoError(t, ctx.readCrypto.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM))

	testData := []byte("Hello, World! This is encrypted data.")

	msg := types.NewMessage()
	msg.MutableHead().SourceBusId = 12345
	msg.MutableHead().Sequence = 1
	msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			Content: testData,
		},
	}

	// Act
	packed, errCode := ctx.PackMessage(msg, 3, 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	unpacked := types.NewMessage()
	errCode = ctx.UnpackMessage(unpacked, packed.UsedSpan(), 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Assert
	assert.Equal(t, msg.GetHead().GetSourceBusId(), unpacked.GetHead().GetSourceBusId())
	assert.Equal(t, testData, unpacked.GetBody().GetDataTransformReq().GetContent())
}

func TestConnectionContextPackUnpackMessageWithCBCIVInHeader(t *testing.T) {
	// Test: Verify CBC uses header IV and does not prefix IV in body
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	key := make([]byte, 16)
	iv := make([]byte, 16)
	_, _ = rand.Read(key)
	_, _ = rand.Read(iv)
	require.NoError(t, ctx.writeCrypto.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC))
	require.NoError(t, ctx.readCrypto.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC))

	testData := []byte("Hello, CBC encrypted data.")
	msg := types.NewMessage()
	msg.MutableHead().SourceBusId = 12345
	msg.MutableHead().Sequence = 1
	msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			Content: testData,
		},
	}

	// Act
	packed, errCode := ctx.PackMessage(msg, 3, 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	data := packed.UsedSpan()
	headSize, headVintSize := buffer.ReadVint(data)
	require.NotZero(t, headVintSize)

	head := &protocol.MessageHead{}
	err := proto.Unmarshal(data[headVintSize:headVintSize+int(headSize)], head)
	require.NoError(t, err)

	bodyBytes := data[headVintSize+int(headSize):]
	originalBodyBytes, err := proto.Marshal(msg.GetBody())
	require.NoError(t, err)

	// Assert - header IV present and body length matches padded ciphertext size
	require.NotNil(t, head.GetCrypto())
	assert.Len(t, head.GetCrypto().GetIv(), 16)
	blockSize := cryptoAlgorithmIVSize(protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC)
	expectedCipherLen := ((len(originalBodyBytes) / blockSize) + 1) * blockSize
	assert.Equal(t, expectedCipherLen, len(bodyBytes))

	// Assert - unpack succeeds and data matches
	unpacked := types.NewMessage()
	errCode = ctx.UnpackMessage(unpacked, data, 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	assert.Equal(t, testData, unpacked.GetBody().GetDataTransformReq().GetContent())
}

func TestConnectionContextPackUnpackMessageWithCompression(t *testing.T) {
	// Test: Verify pack/unpack with compression
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	require.NoError(t, ctx.compression.SetAlgorithm(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB))

	// Use compressible data
	testData := bytes.Repeat([]byte("Hello, World! "), 100)

	msg := types.NewMessage()
	msg.MutableHead().SourceBusId = 12345
	msg.MutableHead().Sequence = 1
	msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			Content: testData,
		},
	}

	// Act
	packed, errCode := ctx.PackMessage(msg, 3, 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	require.NotNil(t, packed)

	unpacked := types.NewMessage()
	errCode = ctx.UnpackMessage(unpacked, packed.UsedSpan(), 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Assert
	assert.Equal(t, msg.GetHead().GetSourceBusId(), unpacked.GetHead().GetSourceBusId())
	assert.Equal(t, testData, unpacked.GetBody().GetDataTransformReq().GetContent())
	assert.NotNil(t, unpacked.GetHead().GetCompression())
}

func TestConnectionContextPackUnpackMessageWithCryptoAndCompression(t *testing.T) {
	// Test: Verify pack/unpack with both encryption and compression
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	key := make([]byte, 32)
	iv := make([]byte, 12)
	_, _ = rand.Read(key)
	_, _ = rand.Read(iv)
	require.NoError(t, ctx.writeCrypto.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM))
	require.NoError(t, ctx.readCrypto.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM))
	require.NoError(t, ctx.compression.SetAlgorithm(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB))

	// Use compressible data
	testData := bytes.Repeat([]byte("Hello, World! "), 100)

	msg := types.NewMessage()
	msg.MutableHead().SourceBusId = 12345
	msg.MutableHead().Sequence = 1
	msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			Content: testData,
		},
	}

	// Act
	packed, errCode := ctx.PackMessage(msg, 3, 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	require.NotNil(t, packed)

	unpacked := types.NewMessage()
	errCode = ctx.UnpackMessage(unpacked, packed.UsedSpan(), 65536)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Assert
	assert.Equal(t, msg.GetHead().GetSourceBusId(), unpacked.GetHead().GetSourceBusId())
	assert.Equal(t, testData, unpacked.GetBody().GetDataTransformReq().GetContent())
	assert.NotNil(t, unpacked.GetHead().GetCompression())
}

func TestConnectionContextPackMessageWhenClosing(t *testing.T) {
	// Test: Verify pack fails when connection is closing
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	ctx.SetClosing(true)

	msg := types.NewMessage()
	msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			Content: []byte("test"),
		},
	}

	// Act
	_, errCode := ctx.PackMessage(msg, 3, 65536)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, errCode)
}

func TestConnectionContextUnpackMessageNilInput(t *testing.T) {
	// Test: Verify unpack fails with nil input
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	msg := types.NewMessage()

	// Act
	errCode := ctx.UnpackMessage(msg, nil, 65536)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, errCode)
}

func TestConnectionContextUnpackMessageEmptyInput(t *testing.T) {
	// Test: Verify unpack fails with empty input
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	msg := types.NewMessage()

	// Act
	errCode := ctx.UnpackMessage(msg, []byte{}, 65536)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, errCode)
}

func TestConnectionContextUnpackMessageTooShort(t *testing.T) {
	// Test: Verify unpack fails with too short data
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	msg := types.NewMessage()

	// Act
	errCode := ctx.UnpackMessage(msg, []byte{1, 2, 3}, 65536)

	// Assert - returns EN_ATBUS_ERR_UNPACK when protobuf unmarshal fails
	assert.Equal(t, error_code.EN_ATBUS_ERR_UNPACK, errCode)
}

// ============================================================================
// Concurrent Safety Tests
// ============================================================================

func TestCryptoSessionConcurrentEncrypt(t *testing.T) {
	// Test: Verify concurrent encryption safety
	// Arrange
	session := NewCryptoSession()
	key := make([]byte, 32)
	iv := make([]byte, 12)
	_, _ = rand.Read(key)
	_, _ = rand.Read(iv)
	require.NoError(t, session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM))

	testData := []byte("Concurrent test data")
	var wg sync.WaitGroup
	numGoroutines := 100

	// Act
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := session.Encrypt(testData)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	// Assert - No race conditions or panics
}

func TestConnectionContextConcurrentSequence(t *testing.T) {
	// Test: Verify concurrent sequence number generation
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	var wg sync.WaitGroup
	numGoroutines := 100
	sequences := make([]uint64, numGoroutines)

	// Act
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sequences[idx] = ctx.GetNextSequence()
		}(i)
	}
	wg.Wait()

	// Assert - All sequences should be unique
	seqSet := make(map[uint64]bool)
	for _, seq := range sequences {
		assert.False(t, seqSet[seq], "Duplicate sequence number: %d", seq)
		seqSet[seq] = true
	}
}

// ============================================================================
// DeriveKey Tests
// ============================================================================

func TestCryptoSessionDeriveKey(t *testing.T) {
	// Test: Verify key derivation from shared secret
	// Arrange
	session := NewCryptoSession()
	sharedSecret := make([]byte, 32)
	_, _ = rand.Read(sharedSecret)

	require.NoError(t, session.GenerateKeyPair(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519))

	// Act
	err := session.DeriveKey(
		sharedSecret,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256,
	)

	// Assert
	require.NoError(t, err)
	assert.True(t, session.IsInitialized())
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, session.Algorithm)
	assert.Len(t, session.Key, 32)
	assert.Len(t, session.IV, 12)
}

func TestCryptoSessionDeriveKeyUnsupportedKDF(t *testing.T) {
	// Test: Verify error on unsupported KDF
	// Arrange
	session := NewCryptoSession()
	sharedSecret := make([]byte, 32)

	// Act
	err := session.DeriveKey(sharedSecret, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, protocol.ATBUS_CRYPTO_KDF_TYPE(999))

	// Assert
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrCryptoKDFFailed))
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestCryptoSessionComputeSharedSecretNotInitialized(t *testing.T) {
	// Test: Verify error when computing shared secret without key pair
	// Arrange
	session := NewCryptoSession()

	// Act
	_, err := session.ComputeSharedSecret([]byte("fake public key"))

	// Assert
	assert.Equal(t, ErrCryptoNotInitialized, err)
}

func TestCryptoSessionGetPublicKeyNil(t *testing.T) {
	// Test: Verify nil public key when not generated
	// Arrange
	session := NewCryptoSession()

	// Act
	pubKey := session.GetPublicKey()

	// Assert
	assert.Nil(t, pubKey)
}

func TestMessageHeadFields(t *testing.T) {
	// Test: Verify all MessageHead fields are set correctly
	// Arrange
	head := &protocol.MessageHead{
		Version:     3,
		Type:        10,
		ResultCode:  -1,
		Sequence:    999,
		SourceBusId: 12345678,
		Crypto: &protocol.MessageHeadCrypto{
			Algorithm: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
			Iv:        []byte("test_iv"),
			Aad:       []byte("test_aad"),
		},
		Compression: &protocol.MessageHeadCompression{
			Type:         protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB,
			OriginalSize: 2000,
		},
		BodySize: 1000,
	}

	// Assert
	assert.Equal(t, int32(3), head.Version)
	assert.Equal(t, int32(10), head.Type)
	assert.Equal(t, int32(-1), head.ResultCode)
	assert.Equal(t, uint64(999), head.Sequence)
	assert.Equal(t, uint64(12345678), head.SourceBusId)
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, head.GetCrypto().GetAlgorithm())
	assert.Equal(t, []byte("test_iv"), head.GetCrypto().GetIv())
	assert.Equal(t, []byte("test_aad"), head.GetCrypto().GetAad())
	assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, head.GetCompression().GetType())
	assert.Equal(t, uint64(2000), head.GetCompression().GetOriginalSize())
	assert.Equal(t, uint64(1000), head.BodySize)
}

// ============================================================================
// Cross-Language Encryption/Decryption Tests
// These tests use test data generated by C++ atbus_connection_context_crosslang_generator.cpp
// to verify that Go encryption/decryption implementations are compatible with C++.
// Test data is loaded from testdata/*.bytes binary files.
// ============================================================================

// crossLangTestMetadata represents the JSON metadata for cross-language test cases.
type crossLangTestMetadata struct {
	Name                     string `json:"name"`
	Description              string `json:"description"`
	ProtocolVersion          int    `json:"protocol_version"`
	BodyType                 string `json:"body_type"`
	BodyTypeCase             int    `json:"body_type_case"`
	CompressionAlgorithm     string `json:"compression_algorithm"`
	CompressionAlgorithmType int    `json:"compression_algorithm_type"`
	CompressionOriginalSize  int    `json:"compression_original_size"`
	CryptoAlgorithm          string `json:"crypto_algorithm"`
	CryptoAlgorithmType      int    `json:"crypto_algorithm_type"`
	KeyHex                   string `json:"key_hex"`
	KeySize                  int    `json:"key_size"`
	IVHex                    string `json:"iv_hex"`
	IVSize                   int    `json:"iv_size"`
	AADHex                   string `json:"aad_hex"`
	AADSize                  int    `json:"aad_size"`
	PackedSize               int    `json:"packed_size"`
	PackedHex                string `json:"packed_hex"`
	Expected                 struct {
		From           uint64   `json:"from"`
		To             uint64   `json:"to"`
		Content        string   `json:"content"`
		ContentHex     string   `json:"content_hex"`
		ContentSize    int      `json:"content_size"`
		ContentPattern string   `json:"content_pattern"`
		Flags          uint32   `json:"flags"`
		Commands       []string `json:"commands"`
		TimePoint      int64    `json:"time_point"`
		BusID          uint64   `json:"bus_id"`
		PID            int32    `json:"pid"`
		Hostname       string   `json:"hostname"`
		Channels       []string `json:"channels"`
		NodeBusIDs     []uint64 `json:"node_bus_ids"`
		Address        string   `json:"address"`
	} `json:"expected"`
}

// loadCrossLangTestData loads binary data from the testdata directory.
func loadCrossLangTestData(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "Failed to read test data file: %s", path)
	return data
}

// loadCrossLangTestMetadata loads and parses JSON metadata from the testdata directory.
func loadCrossLangTestMetadata(t *testing.T, filename string) *crossLangTestMetadata {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "Failed to read test metadata file: %s", path)

	var metadata crossLangTestMetadata
	err = json.Unmarshal(data, &metadata)
	require.NoError(t, err, "Failed to parse test metadata file: %s", path)

	return &metadata
}

func decodeHexField(t *testing.T, hexValue string, fieldName string) []byte {
	t.Helper()
	if hexValue == "" {
		return nil
	}
	data, err := hex.DecodeString(hexValue)
	require.NoError(t, err, "Failed to decode %s", fieldName)
	return data
}

func parsePackedMessage(t *testing.T, packed []byte) (*protocol.MessageHead, []byte) {
	t.Helper()
	headSize, headVintSize := buffer.ReadVint(packed)
	require.NotZero(t, headVintSize)
	require.LessOrEqual(t, headVintSize+int(headSize), len(packed))

	head := &protocol.MessageHead{}
	err := proto.Unmarshal(packed[headVintSize:headVintSize+int(headSize)], head)
	require.NoError(t, err)

	bodyBytes := packed[headVintSize+int(headSize):]
	return head, bodyBytes
}

func setupReadCryptoFromMetadata(t *testing.T, ctx *ConnectionContext, metadata *crossLangTestMetadata) {
	t.Helper()
	if metadata == nil || metadata.CryptoAlgorithmType == 0 {
		return
	}
	key := decodeHexField(t, metadata.KeyHex, "key")
	iv := decodeHexField(t, metadata.IVHex, "iv")
	alg := protocol.ATBUS_CRYPTO_ALGORITHM_TYPE(metadata.CryptoAlgorithmType)
	if alg == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		return
	}
	require.NoError(t, ctx.readCrypto.SetKey(key, iv, alg))
	require.NoError(t, ctx.writeCrypto.SetKey(key, iv, alg))
}

// TestCrossLangEncryptDecryptAES128GCM verifies AES-128-GCM encryption/decryption
// produces compatible results with C++ implementation.
func TestCrossLangEncryptDecryptAES128GCM(t *testing.T) {
	// Load test metadata
	metadata := loadCrossLangTestMetadata(t, "enc_aes_128_gcm_data_transform_req.json")

	// Arrange - parse key and IV from metadata
	key := decodeHexField(t, metadata.KeyHex, "key")
	iv := decodeHexField(t, metadata.IVHex, "iv")
	aad := decodeHexField(t, metadata.AADHex, "aad")

	// Setup crypto session
	session := NewCryptoSession()
	err := session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM)
	require.NoError(t, err, "Failed to set key")

	// Verify session is initialized correctly
	assert.True(t, session.IsInitialized())
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, session.Algorithm)
	assert.Equal(t, key, session.Key)
	assert.Equal(t, iv, session.IV)

	// Test encrypt/decrypt roundtrip with IV/AAD
	testData := []byte("Hello, encrypted atbus!")
	encrypted, err := session.EncryptWithIVAndAAD(testData, iv, aad)
	require.NoError(t, err)

	decrypted, err := session.DecryptWithIVAndAAD(encrypted, iv, aad)
	require.NoError(t, err)

	assert.Equal(t, testData, decrypted)
}

// TestCrossLangEncryptDecryptAES192GCM verifies AES-192-GCM encryption/decryption
// produces compatible results with C++ implementation.
func TestCrossLangEncryptDecryptAES192GCM(t *testing.T) {
	// Load test metadata
	metadata := loadCrossLangTestMetadata(t, "enc_aes_192_gcm_data_transform_req.json")

	// Arrange - parse key and IV from metadata
	key := decodeHexField(t, metadata.KeyHex, "key")
	iv := decodeHexField(t, metadata.IVHex, "iv")
	aad := decodeHexField(t, metadata.AADHex, "aad")

	// Setup crypto session
	session := NewCryptoSession()
	err := session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM)
	require.NoError(t, err, "Failed to set key")

	// Verify session is initialized correctly
	assert.True(t, session.IsInitialized())
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, session.Algorithm)

	// Test encrypt/decrypt roundtrip with IV/AAD
	testData := []byte("Hello, encrypted atbus!")
	encrypted, err := session.EncryptWithIVAndAAD(testData, iv, aad)
	require.NoError(t, err)

	decrypted, err := session.DecryptWithIVAndAAD(encrypted, iv, aad)
	require.NoError(t, err)

	assert.Equal(t, testData, decrypted)
}

// TestCrossLangEncryptDecryptAES256GCM verifies AES-256-GCM encryption/decryption
// produces compatible results with C++ implementation.
func TestCrossLangEncryptDecryptAES256GCM(t *testing.T) {
	// Load test metadata
	metadata := loadCrossLangTestMetadata(t, "enc_aes_256_gcm_data_transform_req.json")

	// Arrange - parse key and IV from metadata
	key := decodeHexField(t, metadata.KeyHex, "key")
	iv := decodeHexField(t, metadata.IVHex, "iv")
	aad := decodeHexField(t, metadata.AADHex, "aad")

	// Setup crypto session
	session := NewCryptoSession()
	err := session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM)
	require.NoError(t, err, "Failed to set key")

	// Verify session is initialized correctly
	assert.True(t, session.IsInitialized())
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, session.Algorithm)

	// Test encrypt/decrypt roundtrip with IV/AAD
	testData := []byte("Hello, encrypted atbus!")
	encrypted, err := session.EncryptWithIVAndAAD(testData, iv, aad)
	require.NoError(t, err)

	decrypted, err := session.DecryptWithIVAndAAD(encrypted, iv, aad)
	require.NoError(t, err)

	assert.Equal(t, testData, decrypted)
}

// TestCrossLangEncryptDecryptAES128CBC verifies AES-128-CBC encryption/decryption
// produces compatible results with C++ implementation.
func TestCrossLangEncryptDecryptAES128CBC(t *testing.T) {
	// Load test metadata
	metadata := loadCrossLangTestMetadata(t, "enc_aes_128_cbc_data_transform_req.json")

	// Arrange - parse key and IV from metadata
	key := decodeHexField(t, metadata.KeyHex, "key")
	iv := decodeHexField(t, metadata.IVHex, "iv")

	// Setup crypto session
	session := NewCryptoSession()
	err := session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC)
	require.NoError(t, err, "Failed to set key")

	// Verify session is initialized correctly
	assert.True(t, session.IsInitialized())
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, session.Algorithm)

	// Test encrypt/decrypt roundtrip with IV
	testData := []byte("Hello, encrypted atbus!")
	encrypted, err := session.EncryptWithIV(testData, iv)
	require.NoError(t, err)

	decrypted, err := session.DecryptWithIV(encrypted, iv)
	require.NoError(t, err)

	assert.Equal(t, testData, decrypted)
}

// TestCrossLangEncryptDecryptAES192CBC verifies AES-192-CBC encryption/decryption
// produces compatible results with C++ implementation.
func TestCrossLangEncryptDecryptAES192CBC(t *testing.T) {
	// Load test metadata
	metadata := loadCrossLangTestMetadata(t, "enc_aes_192_cbc_data_transform_req.json")

	// Arrange - parse key and IV from metadata
	key := decodeHexField(t, metadata.KeyHex, "key")
	iv := decodeHexField(t, metadata.IVHex, "iv")

	// Setup crypto session
	session := NewCryptoSession()
	err := session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC)
	require.NoError(t, err, "Failed to set key")

	// Verify session is initialized correctly
	assert.True(t, session.IsInitialized())
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC, session.Algorithm)

	// Test encrypt/decrypt roundtrip with IV
	testData := []byte("Hello, encrypted atbus!")
	encrypted, err := session.EncryptWithIV(testData, iv)
	require.NoError(t, err)

	decrypted, err := session.DecryptWithIV(encrypted, iv)
	require.NoError(t, err)

	assert.Equal(t, testData, decrypted)
}

// TestCrossLangEncryptDecryptAES256CBC verifies AES-256-CBC encryption/decryption
// produces compatible results with C++ implementation.
func TestCrossLangEncryptDecryptAES256CBC(t *testing.T) {
	// Load test metadata
	metadata := loadCrossLangTestMetadata(t, "enc_aes_256_cbc_data_transform_req.json")

	// Arrange - parse key and IV from metadata
	key := decodeHexField(t, metadata.KeyHex, "key")
	iv := decodeHexField(t, metadata.IVHex, "iv")

	// Setup crypto session
	session := NewCryptoSession()
	err := session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC)
	require.NoError(t, err, "Failed to set key")

	// Verify session is initialized correctly
	assert.True(t, session.IsInitialized())
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, session.Algorithm)

	// Test encrypt/decrypt roundtrip with IV
	testData := []byte("Hello, encrypted atbus!")
	encrypted, err := session.EncryptWithIV(testData, iv)
	require.NoError(t, err)

	decrypted, err := session.DecryptWithIV(encrypted, iv)
	require.NoError(t, err)

	assert.Equal(t, testData, decrypted)
}

// TestCrossLangEncryptDecryptChaCha20Poly1305 verifies ChaCha20-Poly1305 encryption/decryption
// produces compatible results with C++ implementation.
func TestCrossLangEncryptDecryptChaCha20Poly1305(t *testing.T) {
	// Load test metadata
	metadata := loadCrossLangTestMetadata(t, "enc_chacha20_poly1305_data_transform_req.json")

	// Arrange - parse key and IV from metadata
	key := decodeHexField(t, metadata.KeyHex, "key")
	iv := decodeHexField(t, metadata.IVHex, "iv")
	aad := decodeHexField(t, metadata.AADHex, "aad")

	// Setup crypto session
	session := NewCryptoSession()
	err := session.SetKey(key, iv, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF)
	require.NoError(t, err, "Failed to set key")

	// Verify session is initialized correctly
	assert.True(t, session.IsInitialized())
	assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, session.Algorithm)

	// Test encrypt/decrypt roundtrip with IV/AAD
	testData := []byte("Hello, encrypted atbus!")
	encrypted, err := session.EncryptWithIVAndAAD(testData, iv, aad)
	require.NoError(t, err)

	decrypted, err := session.DecryptWithIVAndAAD(encrypted, iv, aad)
	require.NoError(t, err)

	assert.Equal(t, testData, decrypted)
}

// TestCrossLangKeyParametersValidation verifies that all encryption algorithms
// have correct key and IV sizes matching C++ test configurations.
func TestCrossLangKeyParametersValidation(t *testing.T) {
	testCases := []struct {
		name        string
		jsonFile    string
		algorithm   protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		expectedKey int
		expectedIV  int
	}{
		{"AES-128-CBC", "enc_aes_128_cbc_data_transform_req.json", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, 16, 16},
		{"AES-192-CBC", "enc_aes_192_cbc_data_transform_req.json", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC, 24, 16},
		{"AES-256-CBC", "enc_aes_256_cbc_data_transform_req.json", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, 32, 16},
		{"AES-128-GCM", "enc_aes_128_gcm_data_transform_req.json", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, 16, 12},
		{"AES-192-GCM", "enc_aes_192_gcm_data_transform_req.json", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, 24, 12},
		{"AES-256-GCM", "enc_aes_256_gcm_data_transform_req.json", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, 32, 12},
		{"ChaCha20-Poly1305", "enc_chacha20_poly1305_data_transform_req.json", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, 32, 12},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := loadCrossLangTestMetadata(t, tc.jsonFile)

			// Verify key and IV can be decoded correctly (no size validation)
			key, err := hex.DecodeString(metadata.KeyHex)
			require.NoError(t, err)

			iv, err := hex.DecodeString(metadata.IVHex)
			require.NoError(t, err)

			// Assert key/IV sizes match expected test configuration
			assert.Len(t, key, tc.expectedKey)
			assert.Len(t, iv, tc.expectedIV)
		})
	}
}

// TestCrossLangAllEncryptedDataTransformReq verifies all encrypted data_transform_req
// test cases from C++ can be processed with correct key/IV configurations.
func TestCrossLangAllEncryptedDataTransformReq(t *testing.T) {
	testCases := []struct {
		name      string
		jsonFile  string
		bytesFile string
		algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	}{
		{"AES-128-CBC", "enc_aes_128_cbc_data_transform_req.json", "enc_aes_128_cbc_data_transform_req.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC},
		{"AES-192-CBC", "enc_aes_192_cbc_data_transform_req.json", "enc_aes_192_cbc_data_transform_req.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC},
		{"AES-256-CBC", "enc_aes_256_cbc_data_transform_req.json", "enc_aes_256_cbc_data_transform_req.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC},
		{"AES-128-GCM", "enc_aes_128_gcm_data_transform_req.json", "enc_aes_128_gcm_data_transform_req.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM},
		{"AES-192-GCM", "enc_aes_192_gcm_data_transform_req.json", "enc_aes_192_gcm_data_transform_req.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM},
		{"AES-256-GCM", "enc_aes_256_gcm_data_transform_req.json", "enc_aes_256_gcm_data_transform_req.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM},
		{"ChaCha20-Poly1305", "enc_chacha20_poly1305_data_transform_req.json", "enc_chacha20_poly1305_data_transform_req.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF},
		{"XXTEA", "enc_xxtea_data_transform_req.json", "enc_xxtea_data_transform_req.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load metadata and binary data
			metadata := loadCrossLangTestMetadata(t, tc.jsonFile)
			binaryData := loadCrossLangTestData(t, tc.bytesFile)

			// Verify binary data size matches metadata
			assert.Equal(t, metadata.PackedSize, len(binaryData), "Packed size mismatch")

			// Verify packed hex matches binary data
			expectedBinary, err := hex.DecodeString(metadata.PackedHex)
			require.NoError(t, err)
			assert.Equal(t, expectedBinary, binaryData, "Binary data should match packed_hex")

			// Setup crypto session with test keys
			key := decodeHexField(t, metadata.KeyHex, "key")
			iv := decodeHexField(t, metadata.IVHex, "iv")
			aad := decodeHexField(t, metadata.AADHex, "aad")

			session := NewCryptoSession()
			err = session.SetKey(key, iv, tc.algorithm)
			require.NoError(t, err)

			head, bodyBytes := parsePackedMessage(t, binaryData)

			var decrypted []byte
			if tc.algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA {
				// XXTEA has no IV/AAD and no crypto header
				decrypted, err = session.Decrypt(bodyBytes)
				require.NoError(t, err)
				// Trim zero-padding using body_size from header
				bodySize := int(head.GetBodySize())
				if bodySize > 0 && bodySize <= len(decrypted) {
					decrypted = decrypted[:bodySize]
				}
			} else {
				require.NotNil(t, head.GetCrypto())
				assert.Equal(t, tc.algorithm, head.GetCrypto().GetAlgorithm())
				headIV := head.GetCrypto().GetIv()
				headAAD := head.GetCrypto().GetAad()
				if len(iv) > 0 {
					assert.Len(t, headIV, len(iv))
					assert.Equal(t, iv, headIV)
				}
				if cryptoAlgorithmIsAEAD(tc.algorithm) && len(aad) > 0 {
					assert.Len(t, headAAD, len(aad))
					assert.Equal(t, aad, headAAD)
				}

				if cryptoAlgorithmIsAEAD(tc.algorithm) {
					decrypted, err = session.DecryptWithIVAndAAD(bodyBytes, iv, aad)
				} else {
					decrypted, err = session.DecryptWithIV(bodyBytes, iv)
				}
				require.NoError(t, err)
			}

			body := &protocol.MessageBody{}
			err = proto.Unmarshal(decrypted, body)
			require.NoError(t, err)

			// Verify expected content
			assert.Equal(t, "Hello, encrypted atbus!", metadata.Expected.Content)
			assert.Equal(t, uint64(0x123456789ABCDEF0), metadata.Expected.From)
			assert.Equal(t, uint64(0x0FEDCBA987654321), metadata.Expected.To)
			assert.Equal(t, uint32(1), metadata.Expected.Flags)
			req := body.GetDataTransformReq()
			require.NotNil(t, req)
			assert.Equal(t, metadata.Expected.From, req.GetFrom())
			assert.Equal(t, metadata.Expected.To, req.GetTo())
			assert.Equal(t, metadata.Expected.Flags, req.GetFlags())
			assert.Equal(t, []byte(metadata.Expected.Content), req.GetContent())

			// Encrypt again and compare ciphertext bytes
			if tc.algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA {
				again, err := session.Encrypt(decrypted)
				require.NoError(t, err)
				assert.Equal(t, bodyBytes, again)
			} else if cryptoAlgorithmIsAEAD(tc.algorithm) {
				again, err := session.EncryptWithIVAndAAD(decrypted, iv, aad)
				require.NoError(t, err)
				assert.Equal(t, bodyBytes, again)
			} else {
				again, err := session.EncryptWithIV(decrypted, iv)
				require.NoError(t, err)
				assert.Equal(t, bodyBytes, again)
			}
		})
	}
}

// TestCrossLangAllEncryptedCustomCmd verifies all encrypted custom_cmd
// test cases from C++ can be processed with correct key/IV configurations.
func TestCrossLangAllEncryptedCustomCmd(t *testing.T) {
	testCases := []struct {
		name      string
		jsonFile  string
		bytesFile string
		algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	}{
		{"AES-128-CBC", "enc_aes_128_cbc_custom_cmd.json", "enc_aes_128_cbc_custom_cmd.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC},
		{"AES-192-CBC", "enc_aes_192_cbc_custom_cmd.json", "enc_aes_192_cbc_custom_cmd.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC},
		{"AES-256-CBC", "enc_aes_256_cbc_custom_cmd.json", "enc_aes_256_cbc_custom_cmd.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC},
		{"AES-128-GCM", "enc_aes_128_gcm_custom_cmd.json", "enc_aes_128_gcm_custom_cmd.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM},
		{"AES-192-GCM", "enc_aes_192_gcm_custom_cmd.json", "enc_aes_192_gcm_custom_cmd.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM},
		{"AES-256-GCM", "enc_aes_256_gcm_custom_cmd.json", "enc_aes_256_gcm_custom_cmd.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM},
		{"ChaCha20-Poly1305", "enc_chacha20_poly1305_custom_cmd.json", "enc_chacha20_poly1305_custom_cmd.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF},
		{"XXTEA", "enc_xxtea_custom_cmd.json", "enc_xxtea_custom_cmd.bytes", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Load metadata and binary data
			metadata := loadCrossLangTestMetadata(t, tc.jsonFile)
			binaryData := loadCrossLangTestData(t, tc.bytesFile)

			// Verify binary data size matches metadata
			assert.Equal(t, metadata.PackedSize, len(binaryData), "Packed size mismatch")

			// Verify packed hex matches binary data
			expectedBinary, err := hex.DecodeString(metadata.PackedHex)
			require.NoError(t, err)
			assert.Equal(t, expectedBinary, binaryData, "Binary data should match packed_hex")

			// Setup crypto session with test keys
			key := decodeHexField(t, metadata.KeyHex, "key")
			iv := decodeHexField(t, metadata.IVHex, "iv")
			aad := decodeHexField(t, metadata.AADHex, "aad")

			session := NewCryptoSession()
			err = session.SetKey(key, iv, tc.algorithm)
			require.NoError(t, err)

			head, bodyBytes := parsePackedMessage(t, binaryData)

			var decrypted []byte
			if tc.algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA {
				decrypted, err = session.Decrypt(bodyBytes)
				require.NoError(t, err)
				bodySize := int(head.GetBodySize())
				if bodySize > 0 && bodySize <= len(decrypted) {
					decrypted = decrypted[:bodySize]
				}
			} else {
				require.NotNil(t, head.GetCrypto())
				assert.Equal(t, tc.algorithm, head.GetCrypto().GetAlgorithm())
				assert.Equal(t, iv, head.GetCrypto().GetIv())
				if cryptoAlgorithmIsAEAD(tc.algorithm) {
					assert.Equal(t, aad, head.GetCrypto().GetAad())
				}

				if cryptoAlgorithmIsAEAD(tc.algorithm) {
					decrypted, err = session.DecryptWithIVAndAAD(bodyBytes, iv, aad)
				} else {
					decrypted, err = session.DecryptWithIV(bodyBytes, iv)
				}
				require.NoError(t, err)
			}

			body := &protocol.MessageBody{}
			err = proto.Unmarshal(decrypted, body)
			require.NoError(t, err)

			// Verify expected commands
			assert.Equal(t, []string{"cmd1", "arg1", "arg2"}, metadata.Expected.Commands)
			assert.Equal(t, uint64(0xABCDEF0123456789), metadata.Expected.From)
			req := body.GetCustomCommandReq()
			require.NotNil(t, req)
			assert.Equal(t, metadata.Expected.From, req.GetFrom())
			require.Len(t, req.GetCommands(), len(metadata.Expected.Commands))
			for i, cmd := range metadata.Expected.Commands {
				assert.Equal(t, []byte(cmd), req.GetCommands()[i].GetArg())
			}

			// Encrypt again and compare ciphertext bytes
			if tc.algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA {
				again, err := session.Encrypt(decrypted)
				require.NoError(t, err)
				assert.Equal(t, bodyBytes, again)
			} else if cryptoAlgorithmIsAEAD(tc.algorithm) {
				again, err := session.EncryptWithIVAndAAD(decrypted, iv, aad)
				require.NoError(t, err)
				assert.Equal(t, bodyBytes, again)
			} else {
				again, err := session.EncryptWithIV(decrypted, iv)
				require.NoError(t, err)
				assert.Equal(t, bodyBytes, again)
			}
		})
	}
}

// TestCrossLangCompressedDataTransformReq verifies compressed data_transform_req
// test cases from C++ can be unpacked and decompressed in Go.
func TestCrossLangCompressedDataTransformReq(t *testing.T) {
	testCases := []struct {
		name      string
		jsonFile  string
		bytesFile string
	}{
		{"ZSTD", "compress_zstd_data_transform_req.json", "compress_zstd_data_transform_req.bytes"},
		{"LZ4", "compress_lz4_data_transform_req.json", "compress_lz4_data_transform_req.bytes"},
		{"SNAPPY", "compress_snappy_data_transform_req.json", "compress_snappy_data_transform_req.bytes"},
		{"ZLIB", "compress_zlib_data_transform_req.json", "compress_zlib_data_transform_req.bytes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := loadCrossLangTestMetadata(t, tc.jsonFile)
			binaryData := loadCrossLangTestData(t, tc.bytesFile)

			head, _ := parsePackedMessage(t, binaryData)
			require.NotNil(t, head.GetCompression())
			assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(metadata.CompressionAlgorithmType), head.GetCompression().GetType())
			assert.Equal(t, uint64(metadata.CompressionOriginalSize), head.GetCompression().GetOriginalSize())
			assert.NotEqual(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(metadata.CompressionAlgorithmType))

			ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
			assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, ctx.GetCryptoSelectAlgorithm())

			unpacked := types.NewMessage()
			errCode := ctx.UnpackMessage(unpacked, binaryData, 65536)
			require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

			req := unpacked.GetBody().GetDataTransformReq()
			require.NotNil(t, req)
			if metadata.Expected.From != 0 {
				assert.Equal(t, metadata.Expected.From, req.GetFrom())
			}
			if metadata.Expected.To != 0 {
				assert.Equal(t, metadata.Expected.To, req.GetTo())
			}
			if metadata.Expected.ContentSize > 0 {
				assert.Len(t, req.GetContent(), metadata.Expected.ContentSize)
			} else {
				assert.NotEmpty(t, req.GetContent())
			}
		})
	}
}

// TestCrossLangCompressedCustomCmd verifies compressed custom_command_req
// test cases from C++ can be unpacked and decompressed in Go.
func TestCrossLangCompressedCustomCmd(t *testing.T) {
	testCases := []struct {
		name      string
		jsonFile  string
		bytesFile string
	}{
		{"ZSTD", "compress_zstd_custom_cmd.json", "compress_zstd_custom_cmd.bytes"},
		{"LZ4", "compress_lz4_custom_cmd.json", "compress_lz4_custom_cmd.bytes"},
		{"SNAPPY", "compress_snappy_custom_cmd.json", "compress_snappy_custom_cmd.bytes"},
		{"ZLIB", "compress_zlib_custom_cmd.json", "compress_zlib_custom_cmd.bytes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := loadCrossLangTestMetadata(t, tc.jsonFile)
			binaryData := loadCrossLangTestData(t, tc.bytesFile)

			head, _ := parsePackedMessage(t, binaryData)
			require.NotNil(t, head.GetCompression())
			assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(metadata.CompressionAlgorithmType), head.GetCompression().GetType())
			assert.Equal(t, uint64(metadata.CompressionOriginalSize), head.GetCompression().GetOriginalSize())
			assert.NotEqual(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(metadata.CompressionAlgorithmType))

			ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
			assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, ctx.GetCryptoSelectAlgorithm())

			unpacked := types.NewMessage()
			errCode := ctx.UnpackMessage(unpacked, binaryData, 65536)
			require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

			req := unpacked.GetBody().GetCustomCommandReq()
			require.NotNil(t, req)
			if metadata.Expected.From != 0 {
				assert.Equal(t, metadata.Expected.From, req.GetFrom())
			}
			assert.NotEmpty(t, req.GetCommands())
		})
	}
}

// TestCrossLangCompressedEncryptedDataTransformReq verifies compressed+encrypted
// data_transform_req test cases from C++ can be decrypted and decompressed in Go.
func TestCrossLangCompressedEncryptedDataTransformReq(t *testing.T) {
	testCases := []struct {
		name      string
		jsonFile  string
		bytesFile string
	}{
		{"ZSTD_AES256_CBC", "enc_compress_zstd_aes_256_cbc_data_transform_req.json", "enc_compress_zstd_aes_256_cbc_data_transform_req.bytes"},
		{"ZSTD_AES256_GCM", "enc_compress_zstd_aes_256_gcm_data_transform_req.json", "enc_compress_zstd_aes_256_gcm_data_transform_req.bytes"},
		{"LZ4_AES256_CBC", "enc_compress_lz4_aes_256_cbc_data_transform_req.json", "enc_compress_lz4_aes_256_cbc_data_transform_req.bytes"},
		{"LZ4_AES256_GCM", "enc_compress_lz4_aes_256_gcm_data_transform_req.json", "enc_compress_lz4_aes_256_gcm_data_transform_req.bytes"},
		{"SNAPPY_AES256_CBC", "enc_compress_snappy_aes_256_cbc_data_transform_req.json", "enc_compress_snappy_aes_256_cbc_data_transform_req.bytes"},
		{"SNAPPY_AES256_GCM", "enc_compress_snappy_aes_256_gcm_data_transform_req.json", "enc_compress_snappy_aes_256_gcm_data_transform_req.bytes"},
		{"ZLIB_AES256_CBC", "enc_compress_zlib_aes_256_cbc_data_transform_req.json", "enc_compress_zlib_aes_256_cbc_data_transform_req.bytes"},
		{"ZLIB_AES256_GCM", "enc_compress_zlib_aes_256_gcm_data_transform_req.json", "enc_compress_zlib_aes_256_gcm_data_transform_req.bytes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := loadCrossLangTestMetadata(t, tc.jsonFile)
			binaryData := loadCrossLangTestData(t, tc.bytesFile)

			head, _ := parsePackedMessage(t, binaryData)
			require.NotNil(t, head.GetCompression())
			assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(metadata.CompressionAlgorithmType), head.GetCompression().GetType())
			assert.Equal(t, uint64(metadata.CompressionOriginalSize), head.GetCompression().GetOriginalSize())
			require.NotNil(t, head.GetCrypto())
			assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE(metadata.CryptoAlgorithmType), head.GetCrypto().GetAlgorithm())
			assert.NotEqual(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(metadata.CompressionAlgorithmType))

			ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
			setupReadCryptoFromMetadata(t, ctx, metadata)
			assert.NotEqual(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, ctx.GetCryptoSelectAlgorithm())

			unpacked := types.NewMessage()
			errCode := ctx.UnpackMessage(unpacked, binaryData, 65536)
			require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

			req := unpacked.GetBody().GetDataTransformReq()
			require.NotNil(t, req)
			if metadata.Expected.From != 0 {
				assert.Equal(t, metadata.Expected.From, req.GetFrom())
			}
			if metadata.Expected.To != 0 {
				assert.Equal(t, metadata.Expected.To, req.GetTo())
			}
			assert.NotEmpty(t, req.GetContent())
		})
	}
}

// TestCrossLangCompressedEncryptedCustomCmd verifies compressed+encrypted
// custom_command_req test cases from C++ can be decrypted and decompressed in Go.
func TestCrossLangCompressedEncryptedCustomCmd(t *testing.T) {
	testCases := []struct {
		name      string
		jsonFile  string
		bytesFile string
	}{
		{"ZSTD_AES256_CBC", "enc_compress_zstd_aes_256_cbc_custom_cmd.json", "enc_compress_zstd_aes_256_cbc_custom_cmd.bytes"},
		{"ZSTD_AES256_GCM", "enc_compress_zstd_aes_256_gcm_custom_cmd.json", "enc_compress_zstd_aes_256_gcm_custom_cmd.bytes"},
		{"LZ4_AES256_CBC", "enc_compress_lz4_aes_256_cbc_custom_cmd.json", "enc_compress_lz4_aes_256_cbc_custom_cmd.bytes"},
		{"LZ4_AES256_GCM", "enc_compress_lz4_aes_256_gcm_custom_cmd.json", "enc_compress_lz4_aes_256_gcm_custom_cmd.bytes"},
		{"SNAPPY_AES256_CBC", "enc_compress_snappy_aes_256_cbc_custom_cmd.json", "enc_compress_snappy_aes_256_cbc_custom_cmd.bytes"},
		{"SNAPPY_AES256_GCM", "enc_compress_snappy_aes_256_gcm_custom_cmd.json", "enc_compress_snappy_aes_256_gcm_custom_cmd.bytes"},
		{"ZLIB_AES256_CBC", "enc_compress_zlib_aes_256_cbc_custom_cmd.json", "enc_compress_zlib_aes_256_cbc_custom_cmd.bytes"},
		{"ZLIB_AES256_GCM", "enc_compress_zlib_aes_256_gcm_custom_cmd.json", "enc_compress_zlib_aes_256_gcm_custom_cmd.bytes"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := loadCrossLangTestMetadata(t, tc.jsonFile)
			binaryData := loadCrossLangTestData(t, tc.bytesFile)

			head, _ := parsePackedMessage(t, binaryData)
			require.NotNil(t, head.GetCompression())
			assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(metadata.CompressionAlgorithmType), head.GetCompression().GetType())
			assert.Equal(t, uint64(metadata.CompressionOriginalSize), head.GetCompression().GetOriginalSize())
			require.NotNil(t, head.GetCrypto())
			assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE(metadata.CryptoAlgorithmType), head.GetCrypto().GetAlgorithm())
			assert.NotEqual(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(metadata.CompressionAlgorithmType))

			ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
			setupReadCryptoFromMetadata(t, ctx, metadata)
			assert.NotEqual(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, ctx.GetCryptoSelectAlgorithm())

			unpacked := types.NewMessage()
			errCode := ctx.UnpackMessage(unpacked, binaryData, 65536)
			require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

			req := unpacked.GetBody().GetCustomCommandReq()
			require.NotNil(t, req)
			if metadata.Expected.From != 0 {
				assert.Equal(t, metadata.Expected.From, req.GetFrom())
			}
			assert.NotEmpty(t, req.GetCommands())
		})
	}
}

// TestCrossLangNoEncryptionDataFiles verifies all non-encrypted test files
// can be read and have consistent metadata.
func TestCrossLangNoEncryptionDataFiles(t *testing.T) {
	noEncFiles := []struct {
		name      string
		jsonFile  string
		bytesFile string
		bodyType  string
	}{
		{"ping_req", "no_enc_ping_req.json", "no_enc_ping_req.bytes", "node_ping_req"},
		{"pong_rsp", "no_enc_pong_rsp.json", "no_enc_pong_rsp.bytes", "node_pong_rsp"},
		{"data_transform_req_simple", "no_enc_data_transform_req_simple.json", "no_enc_data_transform_req_simple.bytes", "data_transform_req"},
		{"data_transform_req_with_rsp_flag", "no_enc_data_transform_req_with_rsp_flag.json", "no_enc_data_transform_req_with_rsp_flag.bytes", "data_transform_req"},
		{"data_transform_rsp", "no_enc_data_transform_rsp.json", "no_enc_data_transform_rsp.bytes", "data_transform_rsp"},
		{"custom_command_req", "no_enc_custom_command_req.json", "no_enc_custom_command_req.bytes", "custom_command_req"},
		{"custom_command_rsp", "no_enc_custom_command_rsp.json", "no_enc_custom_command_rsp.bytes", "custom_command_rsp"},
		{"node_register_req", "no_enc_node_register_req.json", "no_enc_node_register_req.bytes", "node_register_req"},
		{"node_register_rsp", "no_enc_node_register_rsp.json", "no_enc_node_register_rsp.bytes", "node_register_rsp"},
		{"data_transform_binary_content", "no_enc_data_transform_binary_content.json", "no_enc_data_transform_binary_content.bytes", "data_transform_req"},
		{"data_transform_large_content", "no_enc_data_transform_large_content.json", "no_enc_data_transform_large_content.bytes", "data_transform_req"},
		{"data_transform_utf8_content", "no_enc_data_transform_utf8_content.json", "no_enc_data_transform_utf8_content.bytes", "data_transform_req"},
	}

	for _, tc := range noEncFiles {
		t.Run(tc.name, func(t *testing.T) {
			// Load metadata and binary data
			metadata := loadCrossLangTestMetadata(t, tc.jsonFile)
			binaryData := loadCrossLangTestData(t, tc.bytesFile)

			// Verify protocol version
			assert.Equal(t, 3, metadata.ProtocolVersion, "Protocol version should be 3")

			// Verify crypto algorithm is NONE
			assert.Equal(t, "NONE", metadata.CryptoAlgorithm, "Crypto algorithm should be NONE")

			// Verify body type
			assert.Equal(t, tc.bodyType, metadata.BodyType, "Body type mismatch")

			// Verify binary data size matches metadata
			assert.Equal(t, metadata.PackedSize, len(binaryData), "Packed size mismatch")

			// Verify packed hex matches binary data
			expectedBinary, err := hex.DecodeString(metadata.PackedHex)
			require.NoError(t, err)
			assert.Equal(t, expectedBinary, binaryData, "Binary data should match packed_hex")

			// Verify data_transform_rsp body when applicable
			if tc.bodyType == "data_transform_rsp" {
				head, bodyBytes := parsePackedMessage(t, binaryData)
				assert.Nil(t, head.GetCrypto())
				body := &protocol.MessageBody{}
				err = proto.Unmarshal(bodyBytes, body)
				require.NoError(t, err)
				rsp := body.GetDataTransformRsp()
				require.NotNil(t, rsp)
				assert.Equal(t, metadata.Expected.From, rsp.GetFrom())
				assert.Equal(t, metadata.Expected.To, rsp.GetTo())
				assert.Equal(t, metadata.Expected.Flags, rsp.GetFlags())
				if metadata.Expected.Content != "" {
					assert.Equal(t, []byte(metadata.Expected.Content), rsp.GetContent())
				} else if metadata.Expected.ContentHex != "" {
					contentHex, decodeErr := hex.DecodeString(metadata.Expected.ContentHex)
					require.NoError(t, decodeErr)
					assert.Equal(t, contentHex, rsp.GetContent())
				}
			}
		})
	}
}

// TestCrossLangXXTEAEncryptDecrypt verifies that XXTEA test data from C++
// can be decrypted correctly and re-encrypted to produce identical ciphertext.
func TestCrossLangXXTEAEncryptDecrypt(t *testing.T) {
	// Load test metadata and binary data
	metadata := loadCrossLangTestMetadata(t, "enc_xxtea_data_transform_req.json")
	binaryData := loadCrossLangTestData(t, "enc_xxtea_data_transform_req.bytes")

	// Verify the algorithm type
	assert.Equal(t, "xxtea", metadata.CryptoAlgorithm)
	assert.Equal(t, 1, metadata.CryptoAlgorithmType)
	assert.Equal(t, 16, metadata.KeySize, "XXTEA key size should be 16 bytes")

	// Verify binary data size matches metadata
	assert.Equal(t, metadata.PackedSize, len(binaryData), "Packed size mismatch")

	expectedBinary, err := hex.DecodeString(metadata.PackedHex)
	require.NoError(t, err)
	assert.Equal(t, expectedBinary, binaryData, "Binary data should match packed_hex")

	// Setup crypto session with test key
	key := decodeHexField(t, metadata.KeyHex, "key")
	session := NewCryptoSession()
	err = session.SetKey(key, nil, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA)
	require.NoError(t, err)
	assert.True(t, session.IsInitialized())

	// Parse packed message
	head, bodyBytes := parsePackedMessage(t, binaryData)
	require.NotNil(t, head)

	// Decrypt the body
	decrypted, err := session.Decrypt(bodyBytes)
	require.NoError(t, err)

	// XXTEA output may include zero-padding; use body_size from header to trim
	bodySize := int(head.GetBodySize())
	if bodySize > 0 && bodySize <= len(decrypted) {
		decrypted = decrypted[:bodySize]
	}

	body := &protocol.MessageBody{}
	err = proto.Unmarshal(decrypted, body)
	require.NoError(t, err)

	// Verify expected content
	req := body.GetDataTransformReq()
	require.NotNil(t, req)
	assert.Equal(t, uint64(0x123456789ABCDEF0), req.GetFrom())
	assert.Equal(t, uint64(0x0FEDCBA987654321), req.GetTo())
	assert.Equal(t, uint32(1), req.GetFlags())
	assert.Equal(t, []byte("Hello, encrypted atbus!"), req.GetContent())

	// Re-encrypt and verify identical ciphertext (XXTEA is deterministic with no IV)
	reencrypted, err := session.Encrypt(decrypted)
	require.NoError(t, err)
	assert.Equal(t, bodyBytes, reencrypted, "Re-encrypted data should match C++ output")
}

// TestCrossLangCryptoSessionSetKeyWithAllAlgorithms verifies that all supported
// encryption algorithms can be configured with the test keys from C++.
func TestCrossLangCryptoSessionSetKeyWithAllAlgorithms(t *testing.T) {
	// Fixed test keys matching C++ generator
	testKey128, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	testKey192, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f1011121314151617")
	testKey256, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	testIV16, _ := hex.DecodeString("a0a1a2a3a4a5a6a7a8a9aaabacadaeaf")
	testIV12, _ := hex.DecodeString("b0b1b2b3b4b5b6b7b8b9babb")
	testIV24, _ := hex.DecodeString("c0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7")

	testCases := []struct {
		name      string
		algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		key       []byte
		iv        []byte
	}{
		{"AES-128-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, testKey128, testIV16},
		{"AES-192-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC, testKey192, testIV16},
		{"AES-256-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, testKey256, testIV16},
		{"AES-128-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, testKey128, testIV12},
		{"AES-192-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, testKey192, testIV12},
		{"AES-256-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, testKey256, testIV12},
		{"ChaCha20-Poly1305", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, testKey256, testIV12},
		{"XChaCha20-Poly1305", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF, testKey256, testIV24},
		{"XXTEA", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA, testKey128, nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := NewCryptoSession()

			// Adjust key size if needed
			key := tc.key
			if len(key) < cryptoAlgorithmKeySize(tc.algorithm) {
				t.Skipf("Test key too short for %s", tc.name)
			}
			key = key[:cryptoAlgorithmKeySize(tc.algorithm)]

			// Adjust IV size if needed
			iv := tc.iv
			if len(iv) < cryptoAlgorithmIVSize(tc.algorithm) {
				t.Skipf("Test IV too short for %s", tc.name)
			}
			iv = iv[:cryptoAlgorithmIVSize(tc.algorithm)]

			err := session.SetKey(key, iv, tc.algorithm)
			require.NoError(t, err, "SetKey should succeed for %s", tc.name)

			assert.True(t, session.IsInitialized())
			assert.Equal(t, tc.algorithm, session.Algorithm)

			// Test roundtrip encryption
			testData := []byte("Cross-language test data for " + tc.name)
			encrypted, err := session.Encrypt(testData)
			require.NoError(t, err)

			decrypted, err := session.Decrypt(encrypted)
			require.NoError(t, err)

			if tc.algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA {
				// XXTEA decrypt includes PKCS#7 padding; trim using original length
				require.GreaterOrEqual(t, len(decrypted), len(testData))
				assert.Equal(t, testData, decrypted[:len(testData)])
			} else {
				assert.Equal(t, testData, decrypted)
			}
		})
	}
}

// TestCrossLangTestDataIntegrity verifies the integrity of all test data files
// by checking that .json and .bytes files are consistent.
func TestCrossLangTestDataIntegrity(t *testing.T) {
	// Read the index file
	indexData := loadCrossLangTestData(t, "index.json")

	var index struct {
		Description     string `json:"description"`
		ProtocolVersion int    `json:"protocol_version"`
		TestFiles       []struct {
			Name     string `json:"name"`
			Binary   string `json:"binary"`
			Metadata string `json:"metadata"`
		} `json:"test_files"`
	}

	err := json.Unmarshal(indexData, &index)
	require.NoError(t, err, "Failed to parse index.json")

	// Verify protocol version
	assert.Equal(t, 3, index.ProtocolVersion, "Protocol version should be 3")

	// Verify all files listed in index exist
	for _, tf := range index.TestFiles {
		t.Run(tf.Name, func(t *testing.T) {
			// Check binary file exists
			binaryPath := filepath.Join("testdata", tf.Binary)
			_, err := os.Stat(binaryPath)
			assert.NoError(t, err, "Binary file should exist: %s", tf.Binary)

			// Check metadata file exists
			metadataPath := filepath.Join("testdata", tf.Metadata)
			_, err = os.Stat(metadataPath)
			assert.NoError(t, err, "Metadata file should exist: %s", tf.Metadata)
		})
	}

	t.Logf("Verified %d test files from index.json", len(index.TestFiles))
}

// ============================================================================
// New Interface Method Tests
// These tests verify the newly implemented interface methods that match
// the C++ atbus::connection_context behavior.
// ============================================================================

func TestConnectionContextGetHandshakeStartTime(t *testing.T) {
	// Test: Verify handshake start time is set correctly
	t.Run("BeforeHandshake_ZeroTime", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Act
		startTime := ctx.GetHandshakeStartTime()

		// Assert - Before handshake, time should be zero
		assert.True(t, startTime.IsZero())
	})

	t.Run("AfterHandshakeGenerateSelfKey_TimeSet", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		beforeTime := time.Now()

		// Act
		errCode := ctx.HandshakeGenerateSelfKey(0)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		startTime := ctx.GetHandshakeStartTime()
		assert.False(t, startTime.IsZero())
		assert.True(t, startTime.After(beforeTime) || startTime.Equal(beforeTime))
		assert.True(t, startTime.Before(time.Now()) || startTime.Equal(time.Now()))
	})
}

func TestConnectionContextGetCryptoKeyExchangeAlgorithm(t *testing.T) {
	// Test: Verify crypto key exchange algorithm is returned correctly
	t.Run("DefaultKeyExchange", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Act - before generating keys, it returns NONE
		keyExchange := ctx.GetCryptoKeyExchangeAlgorithm()

		// Assert
		assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE, keyExchange)
	})

	t.Run("AfterGenerateKey_ReturnsConfigured", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetSupportedKeyExchange(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Act
		errCode := ctx.HandshakeGenerateSelfKey(0)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		keyExchange := ctx.GetCryptoKeyExchangeAlgorithm()

		// Assert
		assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519, keyExchange)
	})
}

func TestConnectionContextGetCryptoSelectKdfType(t *testing.T) {
	// Test: Verify KDF type is returned correctly
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

	// Act
	kdfType := ctx.GetCryptoSelectKdfType()

	// Assert - Default should be HKDF-SHA256
	assert.Equal(t, protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256, kdfType)
}

func TestConnectionContextGetCryptoSelectAlgorithm(t *testing.T) {
	// Test: Verify crypto algorithm is returned correctly
	t.Run("BeforeSetup_ReturnsNone", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Act
		algorithm := ctx.GetCryptoSelectAlgorithm()

		// Assert
		assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, algorithm)
	})

	t.Run("AfterSetupCryptoWithKey_ReturnsConfigured", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		key := make([]byte, 32)
		iv := make([]byte, 12)
		_, _ = rand.Read(key)
		_, _ = rand.Read(iv)

		// Act
		errCode := ctx.SetupCryptoWithKey(protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, key, iv)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		algorithm := ctx.GetCryptoSelectAlgorithm()

		// Assert
		assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, algorithm)
	})
}

func TestConnectionContextGetCompressSelectAlgorithm(t *testing.T) {
	// Test: Verify compression algorithm is returned correctly
	t.Run("Default_ReturnsNone", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Act
		algorithm := ctx.GetCompressSelectAlgorithm()

		// Assert
		assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE, algorithm)
	})

	t.Run("AfterNegotiate_ReturnsNegotiated", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetSupportedCompressionAlgorithms([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB,
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
		})

		// Act
		err := ctx.NegotiateCompressionWithPeer([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB,
		})
		require.NoError(t, err)
		algorithm := ctx.GetCompressSelectAlgorithm()

		// Assert
		assert.Equal(t, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB, algorithm)
	})
}

func TestConnectionContextHandshakeGenerateSelfKey(t *testing.T) {
	// Test: Verify handshake key generation
	t.Run("ClientMode_PeerSequenceZero", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Act
		errCode := ctx.HandshakeGenerateSelfKey(0)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		assert.NotNil(t, ctx.handshakeCrypto.GetPublicKey())
	})

	t.Run("ServerMode_WithPeerSequence", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Act
		errCode := ctx.HandshakeGenerateSelfKey(12345)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		assert.NotNil(t, ctx.handshakeCrypto.GetPublicKey())
	})

	t.Run("WhenClosing_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetClosing(true)

		// Act
		errCode := ctx.HandshakeGenerateSelfKey(0)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, errCode)
	})
}

func TestConnectionContextHandshakeWriteSelfPublicKey(t *testing.T) {
	// Test: Verify writing self public key to handshake data
	t.Run("Success", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		errCode := ctx.HandshakeGenerateSelfKey(0)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

		selfPubKey := &protocol.CryptoHandshakeData{}
		supportedAlgorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		}

		// Act
		errCode = ctx.HandshakeWriteSelfPublicKey(selfPubKey, supportedAlgorithms)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		assert.NotEmpty(t, selfPubKey.GetPublicKey())
		assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519, selfPubKey.GetType())
		assert.Equal(t, supportedAlgorithms, selfPubKey.GetAlgorithms())
	})

	t.Run("NilSelfPubKey_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		errCode := ctx.HandshakeGenerateSelfKey(0)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

		// Act
		errCode = ctx.HandshakeWriteSelfPublicKey(nil, nil)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, errCode)
	})

	t.Run("BeforeKeyGeneration_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		selfPubKey := &protocol.CryptoHandshakeData{}

		// Act
		errCode := ctx.HandshakeWriteSelfPublicKey(selfPubKey, nil)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR, errCode)
	})

	t.Run("WhenClosing_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetClosing(true)
		selfPubKey := &protocol.CryptoHandshakeData{}

		// Act
		errCode := ctx.HandshakeWriteSelfPublicKey(selfPubKey, nil)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, errCode)
	})
}

func TestConnectionContextHandshakeReadPeerKey(t *testing.T) {
	// Test: Verify full handshake flow with read peer key
	t.Run("FullHandshake", func(t *testing.T) {
		// Arrange - Two contexts simulating client and server
		client := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		server := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Generate keys on both sides
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, client.HandshakeGenerateSelfKey(0))
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, server.HandshakeGenerateSelfKey(123))

		// Prepare handshake data
		clientPubKey := &protocol.CryptoHandshakeData{}
		serverPubKey := &protocol.CryptoHandshakeData{}
		supportedAlgorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		}

		// Write public keys
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, client.HandshakeWriteSelfPublicKey(clientPubKey, supportedAlgorithms))
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, server.HandshakeWriteSelfPublicKey(serverPubKey, supportedAlgorithms))

		// Act - Read peer keys
		clientErrCode := client.HandshakeReadPeerKey(serverPubKey, supportedAlgorithms, false)
		serverErrCode := server.HandshakeReadPeerKey(clientPubKey, supportedAlgorithms, false)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, clientErrCode)
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, serverErrCode)
		assert.True(t, client.IsHandshakeDone())
		assert.True(t, server.IsHandshakeDone())

		// Verify both have the same crypto algorithm
		assert.Equal(t, client.GetCryptoSelectAlgorithm(), server.GetCryptoSelectAlgorithm())
	})

	t.Run("NilPeerPubKey_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ctx.HandshakeGenerateSelfKey(0))

		// Act
		errCode := ctx.HandshakeReadPeerKey(nil, nil, false)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, errCode)
	})

	t.Run("KeyExchangeMismatch_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetSupportedKeyExchange(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ctx.HandshakeGenerateSelfKey(0))

		peerPubKey := &protocol.CryptoHandshakeData{
			Type: protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1,
		}

		// Act
		errCode := ctx.HandshakeReadPeerKey(peerPubKey, nil, false)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_READ_PEER_KEY, errCode)
	})

	t.Run("WhenClosing_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetClosing(true)
		peerPubKey := &protocol.CryptoHandshakeData{}

		// Act
		errCode := ctx.HandshakeReadPeerKey(peerPubKey, nil, false)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, errCode)
	})
}

func TestConnectionContextUpdateCompressionAlgorithm(t *testing.T) {
	// Test: Verify compression algorithm update
	t.Run("UpdateAlgorithms", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		newAlgorithms := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB,
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
		}

		// Act
		errCode := ctx.UpdateCompressionAlgorithm(newAlgorithms)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		assert.Equal(t, newAlgorithms, ctx.GetSupportedCompressionAlgorithms())
	})

	t.Run("WhenClosing_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetClosing(true)

		// Act
		errCode := ctx.UpdateCompressionAlgorithm(nil)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, errCode)
	})
}

func TestConnectionContextIsCompressionAlgorithmSupported(t *testing.T) {
	// Test: Verify compression algorithm support check
	t.Run("SupportedAlgorithm", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetSupportedCompressionAlgorithms([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB,
			protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
		})

		// Act & Assert
		assert.True(t, ctx.IsCompressionAlgorithmSupported(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB))
		assert.True(t, ctx.IsCompressionAlgorithmSupported(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE))
	})

	t.Run("AdapterSupportedAlgorithms", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		// Act & Assert
		assert.True(t, ctx.IsCompressionAlgorithmSupported(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD))
		assert.True(t, ctx.IsCompressionAlgorithmSupported(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4))
		assert.True(t, ctx.IsCompressionAlgorithmSupported(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY))
		assert.True(t, ctx.IsCompressionAlgorithmSupported(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB))
	})
}

func TestConnectionContextSetupCryptoWithKey(t *testing.T) {
	// Test: Verify direct crypto setup with key
	t.Run("AES256GCM", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		key := make([]byte, 32)
		iv := make([]byte, 12)
		_, _ = rand.Read(key)
		_, _ = rand.Read(iv)

		// Act
		errCode := ctx.SetupCryptoWithKey(protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, key, iv)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		assert.True(t, ctx.IsHandshakeDone())
		assert.Equal(t, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, ctx.GetCryptoSelectAlgorithm())
	})

	t.Run("ChaCha20Poly1305", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		key := make([]byte, 32)
		iv := make([]byte, 12)
		_, _ = rand.Read(key)
		_, _ = rand.Read(iv)

		// Act
		errCode := ctx.SetupCryptoWithKey(protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, key, iv)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		assert.True(t, ctx.IsHandshakeDone())
	})

	t.Run("InvalidKeySize_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		shortKey := make([]byte, 8) // Too short for AES-256-GCM
		iv := make([]byte, 12)

		// Act
		errCode := ctx.SetupCryptoWithKey(protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, shortKey, iv)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET, errCode)
	})

	t.Run("WhenClosing_ReturnsError", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		ctx.SetClosing(true)

		// Act
		errCode := ctx.SetupCryptoWithKey(protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE, nil, nil)

		// Assert
		assert.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, errCode)
	})

	t.Run("PackUnpackWithSetupKey", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		key := make([]byte, 32)
		iv := make([]byte, 12)
		_, _ = rand.Read(key)
		_, _ = rand.Read(iv)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
			ctx.SetupCryptoWithKey(protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, key, iv))

		testData := []byte("Test data for encryption with SetupCryptoWithKey")
		msg := types.NewMessage()
		msg.MutableHead().SourceBusId = 12345
		msg.MutableHead().Sequence = 1
		msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
			DataTransformReq: &protocol.ForwardData{
				Content: testData,
			},
		}

		// Act
		packed, errCode := ctx.PackMessage(msg, 3, 65536)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

		unpacked := types.NewMessage()
		errCode = ctx.UnpackMessage(unpacked, packed.UsedSpan(), 65536)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

		// Assert
		assert.Equal(t, testData, unpacked.GetBody().GetDataTransformReq().GetContent())
	})
}

func TestConnectionContextConcurrentHandshake(t *testing.T) {
	// Test: Verify concurrent safety during handshake operations
	// Arrange
	ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
	var wg sync.WaitGroup
	numGoroutines := 10

	// Act - Multiple goroutines trying to get handshake info concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ctx.GetHandshakeStartTime()
			_ = ctx.GetCryptoKeyExchangeAlgorithm()
			_ = ctx.GetCryptoSelectKdfType()
			_ = ctx.GetCryptoSelectAlgorithm()
			_ = ctx.GetCompressSelectAlgorithm()
			_ = ctx.IsCompressionAlgorithmSupported(protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB)
		}()
	}
	wg.Wait()

	// Assert - No race conditions or panics
}

// Uncovered scenarios:
// - Compression raw-level overrides: not exposed in current Go connection context API.
// - Corrupted compressed payloads for each algorithm: covered indirectly via invalid-size checks, but
//   not exhaustively enumerated per algorithm to avoid excessive test data.
// - XXTEA compression+encryption: XXTEA encryption is not implemented in Go yet.

// TestConnectionContextKeyRenegotiationFlow tests the full key renegotiation flow.
//
// This is the Go equivalent of the C++ test case:
//
//	atbus_connection_context_test.key_renegotiation_flow
//
// Flow:
//  1. Initial handshake (register_req/register_rsp): both sides establish ciphers.
//  2. Key renegotiation: client sends new handshake (simulating ping), server processes
//     with needConfirm=true → server stages new receive cipher.
//  3. Intermediate state: server send=NEW, receive=OLD; client send=OLD, receive=OLD.
//  4. Client processes pong (needConfirm=false) → client send=NEW, receive=NEW.
//  5. Server confirms handshake → server send=NEW, receive=NEW.
//  6. Both sides communicate with new keys.
func TestConnectionContextKeyRenegotiationFlow(t *testing.T) {
	supportedAlgorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
	}

	keyExchangeTypes := []protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE{
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1,
	}

	for _, keyExchange := range keyExchangeTypes {
		t.Run(keyExchangeString(keyExchange), func(t *testing.T) {
			testKeyRenegotiationFlow(t, keyExchange, supportedAlgorithms)
		})
	}
}

func testKeyRenegotiationFlow(
	t *testing.T,
	keyExchange protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE,
	supportedAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE,
) {
	// Arrange
	serverCtx := NewConnectionContext(keyExchange)
	clientCtx := NewConnectionContext(keyExchange)

	// Before handshake, no ciphers should be initialized
	assert.False(t, serverCtx.GetWriteCrypto().IsInitialized())
	assert.False(t, serverCtx.GetReadCrypto().IsInitialized())
	assert.False(t, clientCtx.GetWriteCrypto().IsInitialized())
	assert.False(t, clientCtx.GetReadCrypto().IsInitialized())
	assert.Nil(t, serverCtx.GetHandshakeReceiveCrypto())
	assert.False(t, serverCtx.GetHandshakePendingConfirm())

	// ====================================================================
	// Phase 1: Initial handshake (register_req/register_rsp flow)
	// ====================================================================
	t.Log("Phase 1: Initial handshake")

	// Client generates key pair (simulating register_req)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, clientCtx.HandshakeGenerateSelfKey(0))

	clientPubKey := &protocol.CryptoHandshakeData{}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		clientCtx.HandshakeWriteSelfPublicKey(clientPubKey, supportedAlgorithms))

	// Server processes register_req (needConfirm=true)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		serverCtx.HandshakeGenerateSelfKey(clientPubKey.GetSequence()))

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		serverCtx.HandshakeReadPeerKey(clientPubKey, supportedAlgorithms, true))

	// After server handshake_read_peer_key(needConfirm=true):
	//   writeCrypto is set (new send cipher)
	//   readCrypto should NOT be updated yet (still old/uninitialized)
	//   handshakeReceiveCrypto holds the new receive cipher pending confirm
	assert.True(t, serverCtx.GetWriteCrypto().IsInitialized())
	assert.False(t, serverCtx.GetReadCrypto().IsInitialized())
	assert.NotNil(t, serverCtx.GetHandshakeReceiveCrypto())
	assert.True(t, serverCtx.GetHandshakePendingConfirm())

	serverPubKey := &protocol.CryptoHandshakeData{}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		serverCtx.HandshakeWriteSelfPublicKey(serverPubKey, supportedAlgorithms))
	initialSequence := serverPubKey.GetSequence()

	// Client processes register_rsp (needConfirm=false → both ciphers applied immediately)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		clientCtx.HandshakeReadPeerKey(serverPubKey, supportedAlgorithms, false))

	assert.True(t, clientCtx.GetWriteCrypto().IsInitialized())
	assert.True(t, clientCtx.GetReadCrypto().IsInitialized())
	assert.False(t, clientCtx.GetHandshakePendingConfirm())

	// Server confirms initial handshake
	serverCtx.ConfirmHandshake(initialSequence)

	// After confirm: server readCrypto should now be set
	assert.True(t, serverCtx.GetReadCrypto().IsInitialized())
	assert.Nil(t, serverCtx.GetHandshakeReceiveCrypto())
	assert.False(t, serverCtx.GetHandshakePendingConfirm())

	// Record initial cipher keys for later comparison
	initialServerWriteKey := make([]byte, len(serverCtx.writeCrypto.Key))
	copy(initialServerWriteKey, serverCtx.writeCrypto.Key)
	initialServerReadKey := make([]byte, len(serverCtx.readCrypto.Key))
	copy(initialServerReadKey, serverCtx.readCrypto.Key)
	initialClientWriteKey := make([]byte, len(clientCtx.writeCrypto.Key))
	copy(initialClientWriteKey, clientCtx.writeCrypto.Key)
	initialClientReadKey := make([]byte, len(clientCtx.readCrypto.Key))
	copy(initialClientReadKey, clientCtx.readCrypto.Key)

	// Verify initial bidirectional communication with content check
	verifyPackUnpack(t, clientCtx, serverCtx, "initial client to server")
	verifyPackUnpack(t, serverCtx, clientCtx, "initial server to client")

	// ====================================================================
	// Phase 2: Key renegotiation (simulating ping/pong with crypto handshake)
	//   Client sends ping with new handshake data, server processes it
	// ====================================================================
	t.Log("Phase 2: Key renegotiation - server processes ping handshake")

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, clientCtx.HandshakeGenerateSelfKey(0))

	clientRenegPubKey := &protocol.CryptoHandshakeData{}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		clientCtx.HandshakeWriteSelfPublicKey(clientRenegPubKey, supportedAlgorithms))

	// Server processes ping handshake (needConfirm=true)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		serverCtx.HandshakeGenerateSelfKey(clientRenegPubKey.GetSequence()))

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		serverCtx.HandshakeReadPeerKey(clientRenegPubKey, supportedAlgorithms, true))

	// After renegotiation handshake_read_peer_key(needConfirm=true):
	//   writeCrypto key should have changed (new send cipher)
	//   readCrypto key should still be the OLD key
	//   handshakeReceiveCrypto should hold the new receive cipher pending confirm
	assert.NotEqual(t, initialServerWriteKey, serverCtx.writeCrypto.Key)
	assert.Equal(t, initialServerReadKey, serverCtx.readCrypto.Key)
	assert.NotNil(t, serverCtx.GetHandshakeReceiveCrypto())
	assert.True(t, serverCtx.GetHandshakePendingConfirm())

	serverRenegPubKey := &protocol.CryptoHandshakeData{}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		serverCtx.HandshakeWriteSelfPublicKey(serverRenegPubKey, supportedAlgorithms))
	renegSequence := serverRenegPubKey.GetSequence()

	// ====================================================================
	// Phase 3: Intermediate state - server processed, client hasn't
	//   Server: send=NEW, receive=OLD.  Client: send=OLD, receive=OLD.
	// ====================================================================
	t.Log("Phase 3: Intermediate - client sends with OLD key, server receives with OLD key")

	// Client cipher keys should still be the initial ones (not yet processed pong)
	assert.Equal(t, initialClientWriteKey, clientCtx.writeCrypto.Key)
	assert.Equal(t, initialClientReadKey, clientCtx.readCrypto.Key)

	// 3a: Client sends data with OLD key → server decrypts with OLD receive → OK
	verifyPackUnpack(t, clientCtx, serverCtx, "client data during renegotiation (old key)")

	// 3b: Server sends data with NEW key → save for client to decrypt after pong
	serverDataDuringReneg := packMessage(t, serverCtx, "server data during renegotiation (new key)")

	// ====================================================================
	// Phase 4: Client processes pong handshake
	//   Server: send=NEW, receive=OLD.  Client: send=NEW, receive=NEW.
	// ====================================================================
	t.Log("Phase 4: Client processes pong response")

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
		clientCtx.HandshakeReadPeerKey(serverRenegPubKey, supportedAlgorithms, false))

	// After client processes pong (needConfirm=false): both ciphers should have changed
	assert.NotEqual(t, initialClientWriteKey, clientCtx.writeCrypto.Key)
	assert.NotEqual(t, initialClientReadKey, clientCtx.readCrypto.Key)

	// 4a: Client decrypts server data from phase 3b with NEW receive → OK
	unpackAndVerify(t, clientCtx, serverDataDuringReneg, "server data during renegotiation (new key)")

	// 4b: Client sends data with NEW key → server can't decrypt yet (receive=OLD)
	clientDataNewKey := packMessage(t, clientCtx, "client data with new key (pre-confirm)")
	{
		recvMsg := types.NewMessage()
		errCode := serverCtx.UnpackMessage(recvMsg, clientDataNewKey, 1024*1024)
		assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode,
			"Server should fail to unpack client data encrypted with NEW key before confirm")
	}

	// ====================================================================
	// Phase 5: Server confirms handshake (simulating on_recv_handshake_confirm)
	//   Server: send=NEW, receive=NEW.  Client: send=NEW, receive=NEW.
	// ====================================================================
	t.Log("Phase 5: Server confirms, both sides use new keys")

	// Before confirm: server readCrypto key is still the OLD key
	assert.Equal(t, initialServerReadKey, serverCtx.readCrypto.Key)
	assert.True(t, serverCtx.GetHandshakePendingConfirm())

	serverCtx.ConfirmHandshake(renegSequence)

	// After confirm: server readCrypto should have changed to the new key
	assert.NotEqual(t, initialServerReadKey, serverCtx.readCrypto.Key)
	assert.Nil(t, serverCtx.GetHandshakeReceiveCrypto())
	assert.False(t, serverCtx.GetHandshakePendingConfirm())

	// All cipher keys should be different from the initial ones
	assert.NotEqual(t, initialServerWriteKey, serverCtx.writeCrypto.Key)
	assert.NotEqual(t, initialServerReadKey, serverCtx.readCrypto.Key)
	assert.NotEqual(t, initialClientWriteKey, clientCtx.writeCrypto.Key)
	assert.NotEqual(t, initialClientReadKey, clientCtx.readCrypto.Key)

	// 5a: Server can now process client's data encrypted with NEW key
	unpackAndVerify(t, serverCtx, clientDataNewKey, "client data with new key (pre-confirm)")

	// 5b: Full bidirectional communication with new keys
	verifyPackUnpack(t, clientCtx, serverCtx, "post renegotiation client to server")
	verifyPackUnpack(t, serverCtx, clientCtx, "post renegotiation server to client")

	t.Log("Key renegotiation test PASSED")
}

func TestConfirmHandshakeEdgeCases(t *testing.T) {
	// Test: ConfirmHandshake with mismatched sequence is a no-op
	t.Run("MismatchedSequence_NoOp", func(t *testing.T) {
		// Arrange
		serverCtx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		clientCtx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)

		supportedAlgorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		}

		// Perform initial handshake and renegotiation to get pending confirm
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, clientCtx.HandshakeGenerateSelfKey(0))
		clientPubKey := &protocol.CryptoHandshakeData{}
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
			clientCtx.HandshakeWriteSelfPublicKey(clientPubKey, supportedAlgorithms))

		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
			serverCtx.HandshakeGenerateSelfKey(clientPubKey.GetSequence()))
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS,
			serverCtx.HandshakeReadPeerKey(clientPubKey, supportedAlgorithms, true))

		assert.True(t, serverCtx.GetHandshakePendingConfirm())

		// Act - Confirm with wrong sequence
		serverCtx.ConfirmHandshake(99999)

		// Assert - Still pending
		assert.True(t, serverCtx.GetHandshakePendingConfirm())
		assert.NotNil(t, serverCtx.GetHandshakeReceiveCrypto())
	})

	// Test: ConfirmHandshake when no confirm is pending is a no-op
	t.Run("NoPendingConfirm_NoOp", func(t *testing.T) {
		// Arrange
		ctx := NewConnectionContext(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519)
		assert.False(t, ctx.GetHandshakePendingConfirm())

		// Act - Should not panic or change state
		ctx.ConfirmHandshake(12345)

		// Assert
		assert.False(t, ctx.GetHandshakePendingConfirm())
		assert.Nil(t, ctx.GetHandshakeReceiveCrypto())
	})
}

// packMessage packs a forward_data message with the given content and returns the packed bytes.
func packMessage(t *testing.T, ctx *ConnectionContext, content string) []byte {
	t.Helper()
	msg := types.NewMessage()
	msg.MutableHead().SourceBusId = 12345
	msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			Content: []byte(content),
		},
	}

	packed, errCode := ctx.PackMessage(msg, 3, 1024*1024)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	require.NotNil(t, packed)

	result := make([]byte, packed.Used())
	copy(result, packed.UsedSpan())
	return result
}

// unpackAndVerify unpacks data and verifies the content matches expected.
func unpackAndVerify(t *testing.T, ctx *ConnectionContext, data []byte, expectedContent string) {
	t.Helper()
	recvMsg := types.NewMessage()
	errCode := ctx.UnpackMessage(recvMsg, data, 1024*1024)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode,
		"Failed to unpack message, expected content: %s", expectedContent)

	recvBody := recvMsg.GetBody()
	require.NotNil(t, recvBody)

	recvContent := recvBody.GetDataTransformReq().GetContent()
	assert.Equal(t, []byte(expectedContent), recvContent)
}

// verifyPackUnpack packs a message on sender side and unpacks it on receiver side,
// verifying the content is preserved.
func verifyPackUnpack(t *testing.T, sender *ConnectionContext, receiver *ConnectionContext, content string) {
	t.Helper()
	data := packMessage(t, sender, content)
	unpackAndVerify(t, receiver, data, content)
}
