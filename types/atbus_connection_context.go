// Package libatbus_types defines shared types and interfaces for libatbus.
package libatbus_types

import (
	"time"

	buffer "github.com/atframework/libatbus-go/buffer"
	protocol "github.com/atframework/libatbus-go/protocol"
)

// ConnectionContext abstracts the per-connection state and the pack/unpack flow.
//
// This interface is the Go equivalent of C++ atbus::connection_context.
// The pack/unpack methods match the C++ signatures:
//   - pack_message(message&, protocol_version, random_engine, max_body_size) -> buffer_result_t
//   - unpack_message(message&, input, max_body_size) -> int
//
// This interface is implemented by [libatbus_impl.ConnectionContext].
// Keep it minimal and stable: only add methods that are required by callers.
type ConnectionContext interface {
	// IsClosing reports whether the connection is in a closing state.
	IsClosing() bool

	// SetClosing sets the closing state of the connection.
	SetClosing(closing bool)

	// IsHandshakeDone reports whether the handshake has been completed.
	IsHandshakeDone() bool

	// GetHandshakeStartTime returns the time when the handshake was started.
	GetHandshakeStartTime() time.Time

	// GetCryptoKeyExchangeAlgorithm returns the key exchange algorithm used for crypto handshake.
	GetCryptoKeyExchangeAlgorithm() protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE

	// GetCryptoSelectKdfType returns the KDF type selected during handshake.
	GetCryptoSelectKdfType() protocol.ATBUS_CRYPTO_KDF_TYPE

	// GetCryptoSelectAlgorithm returns the crypto algorithm selected during handshake.
	GetCryptoSelectAlgorithm() protocol.ATBUS_CRYPTO_ALGORITHM_TYPE

	// GetCompressSelectAlgorithm returns the compression algorithm selected during handshake.
	GetCompressSelectAlgorithm() protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE

	// GetNextSequence returns the next sequence number for message ordering.
	// Each call increments the internal counter and returns the new value.
	GetNextSequence() uint64

	// PackMessage packs a Message into a StaticBufferBlock for transmission.
	//
	// Parameters:
	//   - m: the message to pack (head will be modified with version, body_size, etc.)
	//   - protocolVersion: the protocol version to set in the message head
	//   - maxBodySize: maximum allowed body size (0 means no limit)
	//
	// Returns:
	//   - StaticBufferBlock containing the packed message data
	//   - error code if packing fails, EN_ATBUS_ERR_SUCCESS on success
	PackMessage(m *Message, protocolVersion int32, maxBodySize int) (*buffer.StaticBufferBlock, ErrorType)

	// UnpackMessage unpacks binary data into a Message.
	//
	// Parameters:
	//   - m: the message to populate (will be modified)
	//   - input: the binary data to unpack
	//   - maxBodySize: maximum allowed body size (0 means no limit)
	//
	// Returns:
	//   - error code if unpacking fails, EN_ATBUS_ERR_SUCCESS on success
	UnpackMessage(m *Message, input []byte, maxBodySize int) ErrorType

	// HandshakeGenerateSelfKey generates the local ECDH key pair for handshake.
	//
	// In client mode, peerSequenceId should be 0 to generate a new sequence.
	// In server mode, peerSequenceId should be the peer's handshake sequence.
	//
	// Returns error code, EN_ATBUS_ERR_SUCCESS on success.
	HandshakeGenerateSelfKey(peerSequenceId uint64) ErrorType

	// HandshakeReadPeerKey reads the peer's public key and computes the shared secret.
	//
	// Parameters:
	//   - peerPubKey: the peer's handshake data containing public key
	//   - supportedCryptoAlgorithms: list of locally supported crypto algorithms
	//
	// Returns error code, EN_ATBUS_ERR_SUCCESS on success.
	HandshakeReadPeerKey(peerPubKey *protocol.CryptoHandshakeData,
		supportedCryptoAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) ErrorType

	// HandshakeWriteSelfPublicKey writes the local public key to the handshake data structure.
	//
	// Parameters:
	//   - selfPubKey: output handshake data to populate with local public key
	//   - supportedCryptoAlgorithms: list of locally supported crypto algorithms
	//
	// Returns error code, EN_ATBUS_ERR_SUCCESS on success.
	HandshakeWriteSelfPublicKey(
		selfPubKey *protocol.CryptoHandshakeData,
		supportedCryptoAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) ErrorType

	// UpdateCompressionAlgorithm updates the list of supported compression algorithms.
	//
	// Returns error code, EN_ATBUS_ERR_SUCCESS on success.
	UpdateCompressionAlgorithm(algorithm []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) ErrorType

	// SetupCryptoWithKey directly sets the encryption key and IV, skipping key exchange.
	// This is primarily used for testing purposes.
	//
	// Parameters:
	//   - algorithm: the crypto algorithm type
	//   - key: the encryption key data
	//   - iv: the initialization vector data
	//
	// Returns error code, EN_ATBUS_ERR_SUCCESS on success.
	SetupCryptoWithKey(algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, key []byte, iv []byte) ErrorType
}

// IsCompressionAlgorithmSupported reports whether the specified compression algorithm is supported.
var isCompressionAlgorithmSupportedDelegate func(algorithm protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) bool

func IsCompressionAlgorithmSupported(algorithm protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) bool {
	if isCompressionAlgorithmSupportedDelegate != nil {
		return isCompressionAlgorithmSupportedDelegate(algorithm)
	}

	return false
}

func InternalSetDelegateIsCompressionAlgorithmSupported(delegate func(algorithm protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) bool) {
	isCompressionAlgorithmSupportedDelegate = delegate
}
