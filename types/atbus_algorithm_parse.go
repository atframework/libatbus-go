package libatbus_types

import (
	"strings"

	protocol "github.com/atframework/libatbus-go/protocol"
)

// ParseCryptoAlgorithmName maps a C++-style cipher name to the corresponding
// protocol enum. It intentionally follows libatbus C++ node::parse_crypto_algorithm_name:
// exact token set, case-insensitive match, and no whitespace trimming.
func ParseCryptoAlgorithmName(name string) protocol.ATBUS_CRYPTO_ALGORITHM_TYPE {
	switch {
	case len(name) == 8 && strings.EqualFold(name, "chacha20"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20
	case len(name) == 22 && strings.EqualFold(name, "chacha20-poly1305-ietf"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF
	case len(name) == 23 && strings.EqualFold(name, "xchacha20-poly1305-ietf"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF
	case len(name) == 11 && strings.EqualFold(name, "aes-256-gcm"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM
	case len(name) == 11 && strings.EqualFold(name, "aes-256-cbc"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC
	case len(name) == 11 && strings.EqualFold(name, "aes-192-gcm"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM
	case len(name) == 11 && strings.EqualFold(name, "aes-192-cbc"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC
	case len(name) == 11 && strings.EqualFold(name, "aes-128-gcm"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM
	case len(name) == 11 && strings.EqualFold(name, "aes-128-cbc"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC
	case len(name) == 5 && strings.EqualFold(name, "xxtea"):
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA
	default:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE
	}
}

// ParseCompressionAlgorithmName maps a C++-style compression name to the
// corresponding protocol enum. It intentionally follows libatbus C++
// node::parse_compression_algorithm_name.
func ParseCompressionAlgorithmName(name string) protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE {
	switch {
	case len(name) == 4 && strings.EqualFold(name, "zstd"):
		return protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD
	case len(name) == 3 && strings.EqualFold(name, "lz4"):
		return protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4
	case len(name) == 4 && strings.EqualFold(name, "zlib"):
		return protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB
	case len(name) == 6 && strings.EqualFold(name, "snappy"):
		return protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY
	default:
		return protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE
	}
}
