// Package libatbus_impl provides internal implementation details for libatbus.
// This file implements the connection context with encryption/decryption algorithm negotiation,
// compression algorithm negotiation, encryption/decryption flow, and pack/unpack flow.
package libatbus_impl

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	compression "github.com/atframework/atframe-utils-go/algorithm/compression"
	"github.com/atframework/atframe-utils-go/algorithm/crypto/chacha20"
	"github.com/atframework/atframe-utils-go/algorithm/crypto/xxtea"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"google.golang.org/protobuf/proto"

	buffer "github.com/atframework/libatbus-go/buffer"
	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

var _ types.ConnectionContext = (*ConnectionContext)(nil)

// NOTE:
// Do NOT re-define enum types in this file.
// Use the protobuf generated enums from `libatbus-go/protocol` directly:
//   - protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
//   - protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
//   - protocol.ATBUS_CRYPTO_KDF_TYPE
//   - protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
//
// This file still needs some convenience behaviors (string formatting, key/iv sizes, curve mapping),
// so we implement them as helper functions over protocol enums.

func cryptoAlgorithmString(c protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) string {
	switch c {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE:
		return "NONE"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA:
		return "XXTEA"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC:
		return "AES-128-CBC"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC:
		return "AES-192-CBC"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC:
		return "AES-256-CBC"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM:
		return "AES-128-GCM"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM:
		return "AES-192-GCM"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM:
		return "AES-256-GCM"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20:
		return "CHACHA20"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF:
		return "CHACHA20-POLY1305"
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF:
		return "XCHACHA20-POLY1305"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", c)
	}
}

func cryptoAlgorithmKeySize(c protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) int {
	switch c {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE:
		return 0
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA:
		return 16 // 128 bits
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM:
		return 16 // 128 bits
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM:
		return 24 // 192 bits
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM:
		return 32 // 256 bits
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF:
		return 32 // 256 bits
	default:
		return 0
	}
}

func cryptoAlgorithmIVSize(c protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) int {
	switch c {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE:
		return 0
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA:
		return 0
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC:
		return aes.BlockSize // 16 bytes
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM:
		return 12 // standard GCM nonce size
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20:
		return chacha20.IVSize // 16 bytes: 8 counter + 8 nonce (original DJB variant, matches C++ libsodium)
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF:
		return chacha20poly1305.NonceSize // 12 bytes
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF:
		return chacha20poly1305.NonceSizeX // 24 bytes
	default:
		return 0
	}
}

func cryptoAlgorithmIsAEAD(c protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) bool {
	switch c {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF:
		return true
	default:
		return false
	}
}

func cryptoAlgorithmTagSize(c protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) int {
	switch c {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM:
		return 16 // GCM tag size
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF:
		return chacha20poly1305.Overhead // 16 bytes
	default:
		return 0
	}
}

func keyExchangeString(k protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE) string {
	switch k {
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE:
		return "NONE"
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519:
		return "X25519"
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1:
		return "SECP256R1"
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1:
		return "SECP384R1"
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1:
		return "SECP521R1"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", k)
	}
}

func keyExchangeCurve(k protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE) ecdh.Curve {
	switch k {
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519:
		return ecdh.X25519()
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1:
		return ecdh.P256()
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1:
		return ecdh.P384()
	case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1:
		return ecdh.P521()
	default:
		return nil
	}
}

func kdfTypeString(k protocol.ATBUS_CRYPTO_KDF_TYPE) string {
	switch k {
	case protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256:
		return "HKDF-SHA256"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", k)
	}
}

func compressionAlgorithmString(c protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) string {
	switch c {
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE:
		return "NONE"
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD:
		return "ZSTD"
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4:
		return "LZ4"
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY:
		return "SNAPPY"
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB:
		return "ZLIB"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", c)
	}
}

func compressionAlgorithmToAdapter(c protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) compression.Algorithm {
	switch c {
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD:
		return compression.AlgorithmZstd
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4:
		return compression.AlgorithmLz4
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY:
		return compression.AlgorithmSnappy
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB:
		return compression.AlgorithmZlib
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE:
		return compression.AlgorithmNone
	default:
		return compression.AlgorithmNone
	}
}

func compressionAlgorithmSupported(c protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) bool {
	if c == protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE {
		return true
	}
	return compression.IsAlgorithmSupported(compressionAlgorithmToAdapter(c))
}

// Error definitions for connection context.
var (
	ErrCryptoNotInitialized        = errors.New("crypto not initialized")
	ErrCryptoAlgorithmNotSupported = errors.New("crypto algorithm not supported")
	ErrCryptoInvalidKeySize        = errors.New("invalid crypto key size")
	ErrCryptoInvalidIVSize         = errors.New("invalid crypto iv/nonce size")
	ErrCryptoEncryptFailed         = errors.New("crypto encrypt failed")
	ErrCryptoDecryptFailed         = errors.New("crypto decrypt failed")
	ErrCryptoHandshakeFailed       = errors.New("crypto handshake failed")
	ErrCryptoKeyExchangeFailed     = errors.New("crypto key exchange failed")
	ErrCryptoKDFFailed             = errors.New("crypto kdf failed")
	ErrCompressionNotSupported     = errors.New("compression algorithm not supported")
	ErrCompressionFailed           = errors.New("compression failed")
	ErrDecompressionFailed         = errors.New("decompression failed")
	ErrPackFailed                  = errors.New("pack failed")
	ErrUnpackFailed                = errors.New("unpack failed")
	ErrInvalidData                 = errors.New("invalid data")
	ErrConnectionClosing           = errors.New("connection is closing")
)

// CryptoHandshakeData holds the data for crypto handshake.
type CryptoHandshakeData struct {
	Sequence    uint64
	KeyExchange protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
	KDFTypes    []protocol.ATBUS_CRYPTO_KDF_TYPE
	Algorithms  []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	PublicKey   []byte
	IVSize      uint32
	TagSize     uint32
}

// CryptoSession holds the crypto session state.
type CryptoSession struct {
	mu sync.RWMutex

	// Negotiated algorithm and parameters
	Algorithm   protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	KeyExchange protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
	KDFType     protocol.ATBUS_CRYPTO_KDF_TYPE
	Key         []byte
	IV          []byte
	TagSize     uint32
	IVSize      uint32

	// ECDH key pair
	privateKey *ecdh.PrivateKey
	publicKey  *ecdh.PublicKey

	// Cipher instances (cached for performance)
	aeadCipher  cipher.AEAD
	blockCipher cipher.Block
	xxteaKey    *xxtea.Key

	// Nonce counter for AEAD modes
	nonceCounter uint64

	initialized bool
}

// NewCryptoSession creates a new crypto session.
func NewCryptoSession() *CryptoSession {
	return &CryptoSession{}
}

// IsInitialized returns true if the crypto session is initialized.
func (cs *CryptoSession) IsInitialized() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.initialized
}

// GenerateKeyPair generates a new ECDH key pair for the given key exchange type.
func (cs *CryptoSession) GenerateKeyPair(keyExchange protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	curve := keyExchangeCurve(keyExchange)
	if curve == nil {
		return fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, keyExchangeString(keyExchange))
	}

	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCryptoHandshakeFailed, err)
	}

	cs.privateKey = privateKey
	cs.publicKey = privateKey.PublicKey()
	cs.KeyExchange = keyExchange

	return nil
}

// GetPublicKey returns the public key bytes.
func (cs *CryptoSession) GetPublicKey() []byte {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if cs.publicKey == nil {
		return nil
	}
	return cs.publicKey.Bytes()
}

// ComputeSharedSecret computes the shared secret using the peer's public key.
func (cs *CryptoSession) ComputeSharedSecret(peerPublicKeyBytes []byte) ([]byte, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.privateKey == nil {
		return nil, ErrCryptoNotInitialized
	}

	curve := keyExchangeCurve(cs.KeyExchange)
	if curve == nil {
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, keyExchangeString(cs.KeyExchange))
	}

	peerPublicKey, err := curve.NewPublicKey(peerPublicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoKeyExchangeFailed, err)
	}

	sharedSecret, err := cs.privateKey.ECDH(peerPublicKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoKeyExchangeFailed, err)
	}

	return sharedSecret, nil
}

// DeriveKey derives the encryption key and IV from the shared secret using HKDF.
func (cs *CryptoSession) DeriveKey(sharedSecret []byte, algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, kdfType protocol.ATBUS_CRYPTO_KDF_TYPE) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if kdfType != protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256 {
		return fmt.Errorf("%w: %s", ErrCryptoKDFFailed, kdfTypeString(kdfType))
	}

	keySize := cryptoAlgorithmKeySize(algorithm)
	ivSize := cryptoAlgorithmIVSize(algorithm)
	if keySize == 0 && algorithm != protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		return fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(algorithm))
	}

	// Derive key material using HKDF
	totalSize := keySize + ivSize
	if totalSize == 0 {
		// No encryption needed
		cs.Algorithm = algorithm
		cs.KDFType = kdfType
		cs.initialized = true
		return nil
	}

	hkdfReader := hkdf.New(sha256.New, sharedSecret, nil, nil)
	keyMaterial := make([]byte, totalSize)
	if _, err := io.ReadFull(hkdfReader, keyMaterial); err != nil {
		return fmt.Errorf("%w: %v", ErrCryptoKDFFailed, err)
	}

	cs.Key = keyMaterial[:keySize]
	if ivSize > 0 {
		cs.IV = keyMaterial[keySize:]
	}
	cs.Algorithm = algorithm
	cs.KDFType = kdfType
	cs.IVSize = uint32(ivSize)
	cs.TagSize = uint32(cryptoAlgorithmTagSize(algorithm))

	// Initialize cipher
	if err := cs.initCipher(); err != nil {
		return err
	}

	cs.initialized = true
	return nil
}

// SetKey directly sets the encryption key and IV.
func (cs *CryptoSession) SetKey(key, iv []byte, algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	expectedKeySize := cryptoAlgorithmKeySize(algorithm)
	expectedIVSize := cryptoAlgorithmIVSize(algorithm)

	if algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		cs.Algorithm = algorithm
		cs.initialized = true
		return nil
	}

	if len(key) != expectedKeySize {
		return fmt.Errorf("%w: expected %d, got %d", ErrCryptoInvalidKeySize, expectedKeySize, len(key))
	}

	if len(iv) != expectedIVSize && expectedIVSize > 0 {
		return fmt.Errorf("%w: expected %d, got %d", ErrCryptoInvalidIVSize, expectedIVSize, len(iv))
	}

	cs.Key = make([]byte, len(key))
	copy(cs.Key, key)
	if len(iv) > 0 {
		cs.IV = make([]byte, len(iv))
		copy(cs.IV, iv)
	}
	cs.Algorithm = algorithm
	cs.IVSize = uint32(expectedIVSize)
	cs.TagSize = uint32(cryptoAlgorithmTagSize(algorithm))

	if err := cs.initCipher(); err != nil {
		return err
	}

	cs.initialized = true
	return nil
}

// initCipher initializes the cipher based on the algorithm.
// Caller must hold the lock.
func (cs *CryptoSession) initCipher() error {
	cs.aeadCipher = nil
	cs.blockCipher = nil
	cs.xxteaKey = nil

	switch cs.Algorithm {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE:
		return nil

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM:
		block, err := aes.NewCipher(cs.Key)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCryptoAlgorithmNotSupported, err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCryptoAlgorithmNotSupported, err)
		}
		cs.aeadCipher = aead
		cs.blockCipher = block

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC:
		block, err := aes.NewCipher(cs.Key)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCryptoAlgorithmNotSupported, err)
		}
		cs.blockCipher = block

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF:
		aead, err := chacha20poly1305.New(cs.Key)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCryptoAlgorithmNotSupported, err)
		}
		cs.aeadCipher = aead

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20:
		if len(cs.Key) != chacha20.KeySize {
			return fmt.Errorf("%w: invalid key size %d for ChaCha20", ErrCryptoAlgorithmNotSupported, len(cs.Key))
		}

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF:
		aead, err := chacha20poly1305.NewX(cs.Key)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCryptoAlgorithmNotSupported, err)
		}
		cs.aeadCipher = aead

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA:
		k, err := xxtea.Setup(cs.Key)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrCryptoAlgorithmNotSupported, err)
		}
		cs.xxteaKey = &k

	default:
		return fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}

	return nil
}

// Encrypt encrypts the plaintext data.
func (cs *CryptoSession) Encrypt(plaintext []byte) ([]byte, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.initialized {
		return nil, ErrCryptoNotInitialized
	}

	if cs.Algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		result := make([]byte, len(plaintext))
		copy(result, plaintext)
		return result, nil
	}

	if len(plaintext) == 0 {
		return []byte{}, nil
	}

	switch cs.Algorithm {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF:
		return cs.encryptAEAD(plaintext)

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC:
		return cs.encryptCBC(plaintext)

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20:
		return cs.encryptChaCha20(plaintext)

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA:
		return cs.encryptXXTEA(plaintext)

	default:
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}
}

// EncryptWithIVAndAAD encrypts data using AEAD with a caller-provided IV/nonce and AAD.
// This is used for cross-language compatibility where IV/AAD are carried in message headers.
func (cs *CryptoSession) EncryptWithIVAndAAD(plaintext []byte, iv []byte, aad []byte) ([]byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if !cs.initialized {
		return nil, ErrCryptoNotInitialized
	}

	if !cryptoAlgorithmIsAEAD(cs.Algorithm) {
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}

	if cs.aeadCipher == nil {
		return nil, ErrCryptoNotInitialized
	}

	nonceSize := cs.aeadCipher.NonceSize()
	if len(iv) != nonceSize {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrCryptoInvalidIVSize, nonceSize, len(iv))
	}

	return cs.aeadCipher.Seal(nil, iv, plaintext, aad), nil
}

// EncryptWithIV encrypts data for non-AEAD algorithms that require an IV (e.g., CBC).
// The IV must be provided by the caller and will not be prepended to ciphertext.
func (cs *CryptoSession) EncryptWithIV(plaintext []byte, iv []byte) ([]byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if !cs.initialized {
		return nil, ErrCryptoNotInitialized
	}

	if cryptoAlgorithmIsAEAD(cs.Algorithm) {
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}

	if cs.Algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		result := make([]byte, len(plaintext))
		copy(result, plaintext)
		return result, nil
	}

	switch cs.Algorithm {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC:
		return cs.encryptCBCWithIV(plaintext, iv)
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20:
		return cs.encryptChaCha20WithIV(plaintext, iv)
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA:
		// XXTEA has no IV, ignore the provided IV
		return cs.encryptXXTEA(plaintext)
	default:
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}
}

// encryptAEAD encrypts using AEAD cipher.
// Caller must hold the lock.
func (cs *CryptoSession) encryptAEAD(plaintext []byte) ([]byte, error) {
	if cs.aeadCipher == nil {
		return nil, ErrCryptoNotInitialized
	}

	nonceSize := cs.aeadCipher.NonceSize()
	nonce := make([]byte, nonceSize)

	// Generate nonce: use counter + random for uniqueness
	counter := atomic.AddUint64(&cs.nonceCounter, 1)
	binary.LittleEndian.PutUint64(nonce, counter)
	if nonceSize > 8 {
		// Fill remaining bytes with random data
		if _, err := rand.Read(nonce[8:]); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCryptoEncryptFailed, err)
		}
	}

	// Encrypt: nonce || ciphertext || tag
	ciphertext := cs.aeadCipher.Seal(nil, nonce, plaintext, nil)

	// Prepend nonce to ciphertext
	result := make([]byte, nonceSize+len(ciphertext))
	copy(result[:nonceSize], nonce)
	copy(result[nonceSize:], ciphertext)

	return result, nil
}

// encryptCBC encrypts using CBC mode with PKCS#7 padding.
// Caller must hold the lock.
func (cs *CryptoSession) encryptCBC(plaintext []byte) ([]byte, error) {
	if cs.blockCipher == nil {
		return nil, ErrCryptoNotInitialized
	}

	blockSize := cs.blockCipher.BlockSize()

	// Apply PKCS#7 padding
	padding := blockSize - (len(plaintext) % blockSize)
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	// Generate random IV
	iv := make([]byte, blockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoEncryptFailed, err)
	}

	// Encrypt
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(cs.blockCipher, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Prepend IV to ciphertext
	result := make([]byte, blockSize+len(ciphertext))
	copy(result[:blockSize], iv)
	copy(result[blockSize:], ciphertext)

	return result, nil
}

// encryptCBCWithIV encrypts using CBC mode with PKCS#7 padding and caller-provided IV.
// Caller must hold the lock.
func (cs *CryptoSession) encryptCBCWithIV(plaintext []byte, iv []byte) ([]byte, error) {
	if cs.blockCipher == nil {
		return nil, ErrCryptoNotInitialized
	}

	blockSize := cs.blockCipher.BlockSize()
	if len(iv) != blockSize {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrCryptoInvalidIVSize, blockSize, len(iv))
	}

	// Apply PKCS#7 padding
	padding := blockSize - (len(plaintext) % blockSize)
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	// Encrypt
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(cs.blockCipher, iv)
	mode.CryptBlocks(ciphertext, padded)

	return ciphertext, nil
}

// encryptChaCha20 encrypts using the pure ChaCha20 stream cipher.
// The generated nonce is prepended to the ciphertext for the generic Encrypt API.
// Caller must hold the lock.
func (cs *CryptoSession) encryptChaCha20(plaintext []byte) ([]byte, error) {
	nonceSize := cryptoAlgorithmIVSize(cs.Algorithm)
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoEncryptFailed, err)
	}

	ciphertext, err := cs.encryptChaCha20WithIV(plaintext, nonce)
	if err != nil {
		return nil, err
	}

	result := make([]byte, nonceSize+len(ciphertext))
	copy(result[:nonceSize], nonce)
	copy(result[nonceSize:], ciphertext)
	return result, nil
}

// encryptChaCha20WithIV encrypts using the pure ChaCha20 stream cipher and a caller-provided nonce.
// Caller must hold the lock.
func (cs *CryptoSession) encryptChaCha20WithIV(plaintext []byte, iv []byte) ([]byte, error) {
	if len(iv) != chacha20.IVSize {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrCryptoInvalidIVSize, chacha20.IVSize, len(iv))
	}

	ciphertext := make([]byte, len(plaintext))
	if err := chacha20.XORKeyStream(ciphertext, plaintext, cs.Key, iv); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoEncryptFailed, err)
	}
	return ciphertext, nil
}

// encryptXXTEA encrypts using XXTEA algorithm.
// Applies PKCS#7 padding to 4-byte boundary before encryption (matching C++ behavior).
// Caller must hold the lock.
func (cs *CryptoSession) encryptXXTEA(plaintext []byte) ([]byte, error) {
	if cs.xxteaKey == nil {
		return nil, ErrCryptoNotInitialized
	}

	// Apply PKCS#7 padding to 4-byte boundary (matching C++ pack_message_with)
	const blockSize = 4
	padding := blockSize - (len(plaintext) % blockSize)
	paddedLen := len(plaintext) + padding
	if paddedLen < 8 {
		paddedLen = 8
	}
	padded := make([]byte, paddedLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(plaintext)+padding; i++ {
		padded[i] = byte(padding)
	}

	err := xxtea.EncryptInPlace(cs.xxteaKey, padded)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoEncryptFailed, err)
	}
	return padded, nil
}

// decryptXXTEA decrypts using XXTEA algorithm.
// Returns decrypted data including PKCS#7 padding bytes.
// The caller should use body_size from the message header to truncate to the original size.
// Caller must hold the lock.
func (cs *CryptoSession) decryptXXTEA(ciphertext []byte) ([]byte, error) {
	if cs.xxteaKey == nil {
		return nil, ErrCryptoNotInitialized
	}

	result, err := xxtea.Decrypt(cs.xxteaKey, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoDecryptFailed, err)
	}
	return result, nil
}

// Decrypt decrypts the ciphertext data.
func (cs *CryptoSession) Decrypt(ciphertext []byte) ([]byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if !cs.initialized {
		return nil, ErrCryptoNotInitialized
	}

	if cs.Algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		result := make([]byte, len(ciphertext))
		copy(result, ciphertext)
		return result, nil
	}

	if len(ciphertext) == 0 {
		return []byte{}, nil
	}

	switch cs.Algorithm {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF:
		return cs.decryptAEAD(ciphertext)

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC:
		return cs.decryptCBC(ciphertext)

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20:
		return cs.decryptChaCha20(ciphertext)

	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA:
		return cs.decryptXXTEA(ciphertext)

	default:
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}
}

// DecryptWithIVAndAAD decrypts data using AEAD with a caller-provided IV/nonce and AAD.
// This is used for cross-language compatibility where IV/AAD are carried in message headers.
func (cs *CryptoSession) DecryptWithIVAndAAD(ciphertext []byte, iv []byte, aad []byte) ([]byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if !cs.initialized {
		return nil, ErrCryptoNotInitialized
	}

	if !cryptoAlgorithmIsAEAD(cs.Algorithm) {
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}

	if cs.aeadCipher == nil {
		return nil, ErrCryptoNotInitialized
	}

	nonceSize := cs.aeadCipher.NonceSize()
	if len(iv) != nonceSize {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrCryptoInvalidIVSize, nonceSize, len(iv))
	}

	plaintext, err := cs.aeadCipher.Open(nil, iv, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoDecryptFailed, err)
	}

	return plaintext, nil
}

// DecryptWithIV decrypts data for non-AEAD algorithms that require an IV (e.g., CBC).
// The IV must be provided by the caller and is not expected to be prepended to ciphertext.
func (cs *CryptoSession) DecryptWithIV(ciphertext []byte, iv []byte) ([]byte, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if !cs.initialized {
		return nil, ErrCryptoNotInitialized
	}

	if cryptoAlgorithmIsAEAD(cs.Algorithm) {
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}

	if cs.Algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		result := make([]byte, len(ciphertext))
		copy(result, ciphertext)
		return result, nil
	}

	switch cs.Algorithm {
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC:
		return cs.decryptCBCWithIV(ciphertext, iv)
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20:
		return cs.decryptChaCha20WithIV(ciphertext, iv)
	case protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA:
		// XXTEA has no IV, ignore the provided IV
		return cs.decryptXXTEA(ciphertext)
	default:
		return nil, fmt.Errorf("%w: %s", ErrCryptoAlgorithmNotSupported, cryptoAlgorithmString(cs.Algorithm))
	}
}

// decryptAEAD decrypts using AEAD cipher.
// Caller must hold the lock.
func (cs *CryptoSession) decryptAEAD(ciphertext []byte) ([]byte, error) {
	if cs.aeadCipher == nil {
		return nil, ErrCryptoNotInitialized
	}

	nonceSize := cs.aeadCipher.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrCryptoDecryptFailed)
	}

	nonce := ciphertext[:nonceSize]
	encryptedData := ciphertext[nonceSize:]

	plaintext, err := cs.aeadCipher.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoDecryptFailed, err)
	}

	return plaintext, nil
}

// decryptCBC decrypts using CBC mode and removes PKCS#7 padding.
// Caller must hold the lock.
func (cs *CryptoSession) decryptCBC(ciphertext []byte) ([]byte, error) {
	if cs.blockCipher == nil {
		return nil, ErrCryptoNotInitialized
	}

	blockSize := cs.blockCipher.BlockSize()
	if len(ciphertext) < blockSize*2 {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrCryptoDecryptFailed)
	}

	if len(ciphertext)%blockSize != 0 {
		return nil, fmt.Errorf("%w: invalid ciphertext length", ErrCryptoDecryptFailed)
	}

	iv := ciphertext[:blockSize]
	encryptedData := ciphertext[blockSize:]

	// Decrypt
	plaintext := make([]byte, len(encryptedData))
	mode := cipher.NewCBCDecrypter(cs.blockCipher, iv)
	mode.CryptBlocks(plaintext, encryptedData)

	// Remove PKCS#7 padding
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("%w: invalid padding", ErrCryptoDecryptFailed)
	}

	padding := int(plaintext[len(plaintext)-1])
	if padding <= 0 || padding > blockSize {
		return nil, fmt.Errorf("%w: invalid padding value", ErrCryptoDecryptFailed)
	}

	// Verify padding
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if plaintext[i] != byte(padding) {
			return nil, fmt.Errorf("%w: invalid padding bytes", ErrCryptoDecryptFailed)
		}
	}

	return plaintext[:len(plaintext)-padding], nil
}

// decryptCBCWithIV decrypts using CBC mode and removes PKCS#7 padding with caller-provided IV.
// Caller must hold the lock.
func (cs *CryptoSession) decryptCBCWithIV(ciphertext []byte, iv []byte) ([]byte, error) {
	if cs.blockCipher == nil {
		return nil, ErrCryptoNotInitialized
	}

	blockSize := cs.blockCipher.BlockSize()
	if len(iv) != blockSize {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrCryptoInvalidIVSize, blockSize, len(iv))
	}

	if len(ciphertext) < blockSize {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrCryptoDecryptFailed)
	}

	if len(ciphertext)%blockSize != 0 {
		return nil, fmt.Errorf("%w: invalid ciphertext length", ErrCryptoDecryptFailed)
	}

	// Decrypt
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(cs.blockCipher, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS#7 padding
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("%w: invalid padding", ErrCryptoDecryptFailed)
	}

	padding := int(plaintext[len(plaintext)-1])
	if padding <= 0 || padding > blockSize {
		return nil, fmt.Errorf("%w: invalid padding value", ErrCryptoDecryptFailed)
	}

	// Verify padding
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if plaintext[i] != byte(padding) {
			return nil, fmt.Errorf("%w: invalid padding bytes", ErrCryptoDecryptFailed)
		}
	}

	return plaintext[:len(plaintext)-padding], nil
}

// decryptChaCha20 decrypts using the pure ChaCha20 stream cipher.
// The generic Decrypt API expects the nonce to be prepended to ciphertext.
// Caller must hold the lock.
func (cs *CryptoSession) decryptChaCha20(ciphertext []byte) ([]byte, error) {
	nonceSize := cryptoAlgorithmIVSize(cs.Algorithm)
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrCryptoDecryptFailed)
	}

	return cs.decryptChaCha20WithIV(ciphertext[nonceSize:], ciphertext[:nonceSize])
}

// decryptChaCha20WithIV decrypts using the pure ChaCha20 stream cipher and a caller-provided nonce.
// Caller must hold the lock.
func (cs *CryptoSession) decryptChaCha20WithIV(ciphertext []byte, iv []byte) ([]byte, error) {
	if len(iv) != chacha20.IVSize {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrCryptoInvalidIVSize, chacha20.IVSize, len(iv))
	}

	plaintext := make([]byte, len(ciphertext))
	if err := chacha20.XORKeyStream(plaintext, ciphertext, cs.Key, iv); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCryptoDecryptFailed, err)
	}
	return plaintext, nil
}

// CompressionSession handles compression and decompression.
type CompressionSession struct {
	mu        sync.RWMutex
	Algorithm protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
}

// NewCompressionSession creates a new compression session.
func NewCompressionSession() *CompressionSession {
	return &CompressionSession{
		Algorithm: protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
	}
}

// SetAlgorithm sets the compression algorithm.
func (cs *CompressionSession) SetAlgorithm(algorithm protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if algorithm == protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE {
		cs.Algorithm = algorithm
		return nil
	}

	if !compressionAlgorithmSupported(algorithm) {
		return fmt.Errorf("%w: %s", ErrCompressionNotSupported, compressionAlgorithmString(algorithm))
	}

	cs.Algorithm = algorithm
	return nil
}

// GetAlgorithm returns the current compression algorithm.
func (cs *CompressionSession) GetAlgorithm() protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.Algorithm
}

// Compress compresses the data using the configured algorithm.
func (cs *CompressionSession) Compress(data []byte) ([]byte, error) {
	cs.mu.RLock()
	algorithm := cs.Algorithm
	cs.mu.RUnlock()

	if len(data) == 0 {
		return []byte{}, nil
	}

	switch algorithm {
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE:
		result := make([]byte, len(data))
		copy(result, data)
		return result, nil
	default:
		adapterAlg := compressionAlgorithmToAdapter(algorithm)
		compressed, code := compression.Compress(adapterAlg, data, compression.LevelBalanced)
		switch code {
		case compression.ErrorCodeOk:
			return compressed, nil
		case compression.ErrorCodeNotSupport, compression.ErrorCodeDisabled:
			return nil, fmt.Errorf("%w: %s", ErrCompressionNotSupported, compressionAlgorithmString(algorithm))
		default:
			return nil, fmt.Errorf("%w: %s", ErrCompressionFailed, compressionAlgorithmString(algorithm))
		}
	}
}

// Decompress decompresses the data using the configured algorithm.
func (cs *CompressionSession) Decompress(data []byte, originalSize int) ([]byte, error) {
	cs.mu.RLock()
	algorithm := cs.Algorithm
	cs.mu.RUnlock()

	if len(data) == 0 {
		return []byte{}, nil
	}

	switch algorithm {
	case protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE:
		result := make([]byte, len(data))
		copy(result, data)
		return result, nil
	default:
		adapterAlg := compressionAlgorithmToAdapter(algorithm)
		decompressed, code := compression.Decompress(adapterAlg, data, originalSize)
		switch code {
		case compression.ErrorCodeOk:
			return decompressed, nil
		case compression.ErrorCodeNotSupport, compression.ErrorCodeDisabled:
			return nil, fmt.Errorf("%w: %s", ErrCompressionNotSupported, compressionAlgorithmString(algorithm))
		default:
			return nil, fmt.Errorf("%w: %s", ErrDecompressionFailed, compressionAlgorithmString(algorithm))
		}
	}
}

// NegotiateCompression negotiates the compression algorithm based on supported algorithms.
func NegotiateCompression(local, remote []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE {
	// Priority order: ZSTD > LZ4 > SNAPPY > ZLIB > NONE
	priority := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD,
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4,
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY,
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB,
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
	}

	localSet := make(map[protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE]bool)
	for _, alg := range local {
		localSet[alg] = true
	}

	remoteSet := make(map[protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE]bool)
	for _, alg := range remote {
		remoteSet[alg] = true
	}

	for _, alg := range priority {
		if localSet[alg] && remoteSet[alg] {
			return alg
		}
	}

	return protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE
}

// NegotiateCryptoAlgorithm negotiates the crypto algorithm based on supported algorithms.
func NegotiateCryptoAlgorithm(local, remote []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) protocol.ATBUS_CRYPTO_ALGORITHM_TYPE {
	// Priority order: AEAD algorithms first, then CBC
	priority := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE,
	}

	localSet := make(map[protocol.ATBUS_CRYPTO_ALGORITHM_TYPE]bool)
	for _, alg := range local {
		localSet[alg] = true
	}

	remoteSet := make(map[protocol.ATBUS_CRYPTO_ALGORITHM_TYPE]bool)
	for _, alg := range remote {
		remoteSet[alg] = true
	}

	for _, alg := range priority {
		if localSet[alg] && remoteSet[alg] {
			return alg
		}
	}

	return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE
}

// NegotiateKeyExchange negotiates the key exchange algorithm.
func NegotiateKeyExchange(local, remote protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE) protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE {
	// Both sides must agree on the same key exchange type
	if local == remote {
		return local
	}
	return protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE
}

// NegotiateKDF negotiates the KDF type based on supported types.
func NegotiateKDF(local, remote []protocol.ATBUS_CRYPTO_KDF_TYPE) protocol.ATBUS_CRYPTO_KDF_TYPE {
	localSet := make(map[protocol.ATBUS_CRYPTO_KDF_TYPE]bool)
	for _, kdf := range local {
		localSet[kdf] = true
	}

	for _, kdf := range remote {
		if localSet[kdf] {
			return kdf
		}
	}

	return protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256 // Default
}

// ConnectionContext manages the connection state including crypto and compression.
type ConnectionContext struct {
	mu sync.RWMutex

	// Crypto sessions
	readCrypto             *CryptoSession
	writeCrypto            *CryptoSession
	handshakeCrypto        *CryptoSession
	handshakeReceiveCrypto *CryptoSession // staged receive cipher during renegotiation

	// Compression session
	compression *CompressionSession

	// Connection state
	sequence                uint64
	closing                 bool
	handshakeDone           bool
	handshakePendingConfirm bool // true when waiting for handshake confirm
	handshakeStartTime      time.Time
	handshakeSequence       uint64

	// Supported algorithms
	supportedCryptoAlgorithms      []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	supportedCompressionAlgorithms []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
	supportedKeyExchange           protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
	supportedKDFTypes              []protocol.ATBUS_CRYPTO_KDF_TYPE
}

// NewConnectionContext creates a new connection context with default settings.
func NewConnectionContext(keyExchangeType protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE) *ConnectionContext {
	ret := &ConnectionContext{
		readCrypto:             NewCryptoSession(),
		writeCrypto:            NewCryptoSession(),
		handshakeCrypto:        NewCryptoSession(),
		handshakeReceiveCrypto: nil,
		compression:            NewCompressionSession(),
		supportedCryptoAlgorithms: []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF,
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC,
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC,
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20,
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA,
			protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE,
		},
		supportedCompressionAlgorithms: nil,
		supportedKeyExchange:           keyExchangeType,
		supportedKDFTypes: []protocol.ATBUS_CRYPTO_KDF_TYPE{
			protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256,
		},
	}

	ret.supportedCompressionAlgorithms = ret.GetSupportedCompressionAlgorithms()
	return ret
}

// IsClosing returns true if the connection is closing.
func (cc *ConnectionContext) IsClosing() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.closing
}

// SetClosing sets the closing state.
func (cc *ConnectionContext) SetClosing(closing bool) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.closing = closing
}

// IsHandshakeDone returns true if the handshake is completed.
func (cc *ConnectionContext) IsHandshakeDone() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.handshakeDone
}

// GetHandshakeStartTime returns the time when the handshake was started.
func (cc *ConnectionContext) GetHandshakeStartTime() time.Time {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.handshakeStartTime
}

// GetCryptoKeyExchangeAlgorithm returns the key exchange algorithm used for crypto handshake.
func (cc *ConnectionContext) GetCryptoKeyExchangeAlgorithm() protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if cc.handshakeCrypto != nil {
		return cc.handshakeCrypto.KeyExchange
	}
	return protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE
}

// GetCryptoSelectKdfType returns the KDF type selected during handshake.
func (cc *ConnectionContext) GetCryptoSelectKdfType() protocol.ATBUS_CRYPTO_KDF_TYPE {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if cc.handshakeCrypto != nil {
		return cc.handshakeCrypto.KDFType
	}
	return protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256
}

// GetCryptoSelectAlgorithm returns the crypto algorithm selected during handshake.
func (cc *ConnectionContext) GetCryptoSelectAlgorithm() protocol.ATBUS_CRYPTO_ALGORITHM_TYPE {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if cc.writeCrypto != nil && cc.writeCrypto.IsInitialized() {
		return cc.writeCrypto.Algorithm
	}
	return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE
}

// GetCompressSelectAlgorithm returns the compression algorithm selected during handshake.
func (cc *ConnectionContext) GetCompressSelectAlgorithm() protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if cc.compression != nil {
		return cc.compression.GetAlgorithm()
	}
	return protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE
}

// GetNextSequence returns the next sequence number.
func (cc *ConnectionContext) GetNextSequence() uint64 {
	return atomic.AddUint64(&cc.sequence, 1)
}

// GetReadCrypto returns the read crypto session.
func (cc *ConnectionContext) GetReadCrypto() *CryptoSession {
	return cc.readCrypto
}

// GetWriteCrypto returns the write crypto session.
func (cc *ConnectionContext) GetWriteCrypto() *CryptoSession {
	return cc.writeCrypto
}

// GetCompression returns the compression session.
func (cc *ConnectionContext) GetCompression() *CompressionSession {
	return cc.compression
}

// SetSupportedCryptoAlgorithms sets the supported crypto algorithms.
func (cc *ConnectionContext) SetSupportedCryptoAlgorithms(algorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.supportedCryptoAlgorithms = make([]protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, len(algorithms))
	copy(cc.supportedCryptoAlgorithms, algorithms)
}

// GetSupportedCryptoAlgorithms returns the supported crypto algorithms.
func (cc *ConnectionContext) GetSupportedCryptoAlgorithms() []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	result := make([]protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, len(cc.supportedCryptoAlgorithms))
	copy(result, cc.supportedCryptoAlgorithms)
	return result
}

// SetSupportedCompressionAlgorithms sets the supported compression algorithms.
func (cc *ConnectionContext) SetSupportedCompressionAlgorithms(algorithms []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.supportedCompressionAlgorithms = make([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE, len(algorithms))
	copy(cc.supportedCompressionAlgorithms, algorithms)
}

// GetSupportedCompressionAlgorithms returns the supported compression algorithms.
func (cc *ConnectionContext) GetSupportedCompressionAlgorithms() []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	result := make([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE, len(cc.supportedCompressionAlgorithms))
	copy(result, cc.supportedCompressionAlgorithms)
	return result
}

// SetSupportedKeyExchange sets the supported key exchange type.
func (cc *ConnectionContext) SetSupportedKeyExchange(keyExchange protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.supportedKeyExchange = keyExchange
}

// GetSupportedKeyExchange returns the supported key exchange type.
func (cc *ConnectionContext) GetSupportedKeyExchange() protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.supportedKeyExchange
}

// CreateHandshakeData creates the handshake data for initiating a handshake.
func (cc *ConnectionContext) CreateHandshakeData() (*CryptoHandshakeData, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closing {
		return nil, ErrConnectionClosing
	}

	// Generate key pair
	if err := cc.handshakeCrypto.GenerateKeyPair(cc.supportedKeyExchange); err != nil {
		return nil, err
	}

	return &CryptoHandshakeData{
		Sequence:    atomic.AddUint64(&cc.sequence, 1),
		KeyExchange: cc.supportedKeyExchange,
		KDFTypes:    cc.supportedKDFTypes,
		Algorithms:  cc.supportedCryptoAlgorithms,
		PublicKey:   cc.handshakeCrypto.GetPublicKey(),
		IVSize:      0, // Will be set after negotiation
		TagSize:     0, // Will be set after negotiation
	}, nil
}

// ProcessHandshakeData processes the received handshake data and completes the key exchange.
func (cc *ConnectionContext) ProcessHandshakeData(peerData *CryptoHandshakeData) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closing {
		return ErrConnectionClosing
	}

	// Negotiate key exchange
	keyExchange := NegotiateKeyExchange(cc.supportedKeyExchange, peerData.KeyExchange)
	if keyExchange == protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE &&
		cc.supportedKeyExchange != protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE {
		return fmt.Errorf("%w: key exchange mismatch", ErrCryptoHandshakeFailed)
	}

	// Negotiate crypto algorithm
	algorithm := NegotiateCryptoAlgorithm(cc.supportedCryptoAlgorithms, peerData.Algorithms)

	// Negotiate KDF
	kdf := NegotiateKDF(cc.supportedKDFTypes, peerData.KDFTypes)

	// Generate key pair if not already done
	if cc.handshakeCrypto.privateKey == nil {
		if err := cc.handshakeCrypto.GenerateKeyPair(keyExchange); err != nil {
			return err
		}
	}

	// Compute shared secret
	sharedSecret, err := cc.handshakeCrypto.ComputeSharedSecret(peerData.PublicKey)
	if err != nil {
		return err
	}

	// Derive keys
	if err := cc.handshakeCrypto.DeriveKey(sharedSecret, algorithm, kdf); err != nil {
		return err
	}

	// Set up read and write crypto sessions with the same key
	if err := cc.readCrypto.SetKey(cc.handshakeCrypto.Key, cc.handshakeCrypto.IV, algorithm); err != nil {
		return err
	}
	if err := cc.writeCrypto.SetKey(cc.handshakeCrypto.Key, cc.handshakeCrypto.IV, algorithm); err != nil {
		return err
	}

	cc.handshakeDone = true
	return nil
}

// NegotiateCompressionWithPeer negotiates compression with peer's supported algorithms.
func (cc *ConnectionContext) NegotiateCompressionWithPeer(peerAlgorithms []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	algorithm := NegotiateCompression(cc.supportedCompressionAlgorithms, peerAlgorithms)
	return cc.compression.SetAlgorithm(algorithm)
}

// HandshakeGenerateSelfKey generates the local ECDH key pair for handshake.
// In client mode, peerSequenceId should be 0 to generate a new sequence.
// In server mode, peerSequenceId should be the peer's handshake sequence.
func (cc *ConnectionContext) HandshakeGenerateSelfKey(peerSequenceId uint64) error_code.ErrorType {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closing {
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}

	// Set handshake start time
	cc.handshakeStartTime = time.Now()

	// Generate sequence ID
	if peerSequenceId == 0 {
		cc.handshakeSequence = atomic.AddUint64(&cc.sequence, 1)
	} else {
		cc.handshakeSequence = peerSequenceId
	}

	// Generate key pair using the configured key exchange algorithm
	if err := cc.handshakeCrypto.GenerateKeyPair(cc.supportedKeyExchange); err != nil {
		return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

// HandshakeReadPeerKey reads the peer's public key and computes the shared secret.
func (cc *ConnectionContext) HandshakeReadPeerKey(peerPubKey *protocol.CryptoHandshakeData,
	supportedCryptoAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE,
	needConfirm bool,
) error_code.ErrorType {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closing {
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}

	if peerPubKey == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// Verify key exchange algorithm matches
	if peerPubKey.GetType() != cc.supportedKeyExchange {
		return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_READ_PEER_KEY
	}

	// Compute shared secret
	sharedSecret, err := cc.handshakeCrypto.ComputeSharedSecret(peerPubKey.GetPublicKey())
	if err != nil {
		return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET
	}

	// Negotiate crypto algorithm
	peerAlgorithms := peerPubKey.GetAlgorithms()
	algorithm := NegotiateCryptoAlgorithm(supportedCryptoAlgorithms, peerAlgorithms)

	// Negotiate KDF
	peerKDFs := peerPubKey.GetKdfType()
	kdf := NegotiateKDF(cc.supportedKDFTypes, peerKDFs)

	// Derive keys
	if err := cc.handshakeCrypto.DeriveKey(sharedSecret, algorithm, kdf); err != nil {
		return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_ERROR
	}

	// Set up write crypto session (always applied immediately)
	if err := cc.writeCrypto.SetKey(cc.handshakeCrypto.Key, cc.handshakeCrypto.IV, algorithm); err != nil {
		return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET
	}

	if needConfirm {
		// Stage the receive cipher for later confirmation
		cc.handshakeReceiveCrypto = NewCryptoSession()
		if err := cc.handshakeReceiveCrypto.SetKey(cc.handshakeCrypto.Key, cc.handshakeCrypto.IV, algorithm); err != nil {
			cc.handshakeReceiveCrypto = nil
			return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET
		}
		cc.handshakePendingConfirm = true
	} else {
		// Apply immediately (client side)
		if err := cc.readCrypto.SetKey(cc.handshakeCrypto.Key, cc.handshakeCrypto.IV, algorithm); err != nil {
			return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET
		}

		cc.handshakeDone = true
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

// HandshakeWriteSelfPublicKey writes the local public key to the handshake data structure.
func (cc *ConnectionContext) HandshakeWriteSelfPublicKey(
	selfPubKey *protocol.CryptoHandshakeData,
	supportedCryptoAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE,
) error_code.ErrorType {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if cc.closing {
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}

	if selfPubKey == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// Check if key pair has been generated
	pubKey := cc.handshakeCrypto.GetPublicKey()
	if pubKey == nil {
		return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR
	}

	// Fill in the handshake data
	selfPubKey.Sequence = cc.handshakeSequence
	selfPubKey.Type = cc.supportedKeyExchange
	selfPubKey.KdfType = cc.supportedKDFTypes
	selfPubKey.Algorithms = supportedCryptoAlgorithms
	selfPubKey.PublicKey = pubKey

	return error_code.EN_ATBUS_ERR_SUCCESS
}

// ConfirmHandshake promotes the staged receive cipher to the active receive cipher.
// This should be called after receiving a handshake_confirm from the peer.
// If the handshakeSequence does not match or no confirm is pending, this is a no-op.
func (cc *ConnectionContext) ConfirmHandshake(handshakeSequence uint64) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.handshakeSequence != handshakeSequence || !cc.handshakePendingConfirm {
		return
	}

	cc.readCrypto = cc.handshakeReceiveCrypto
	cc.handshakeReceiveCrypto = nil
	cc.handshakePendingConfirm = false

	cc.handshakeDone = true
}

// GetHandshakePendingConfirm returns true if a handshake confirm is pending.
func (cc *ConnectionContext) GetHandshakePendingConfirm() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.handshakePendingConfirm
}

// GetHandshakeReceiveCrypto returns the staged receive crypto session (nil if none pending).
func (cc *ConnectionContext) GetHandshakeReceiveCrypto() *CryptoSession {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.handshakeReceiveCrypto
}

// GetHandshakeSequence returns the current handshake sequence ID.
func (cc *ConnectionContext) GetHandshakeSequence() uint64 {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.handshakeSequence
}

// UpdateCompressionAlgorithm updates the list of supported compression algorithms.
func (cc *ConnectionContext) UpdateCompressionAlgorithm(
	algorithms []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE,
) error_code.ErrorType {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closing {
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}

	cc.supportedCompressionAlgorithms = make([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE, len(algorithms))
	copy(cc.supportedCompressionAlgorithms, algorithms)

	return error_code.EN_ATBUS_ERR_SUCCESS
}

// IsCompressionAlgorithmSupported reports whether the specified compression algorithm is supported.
func (cc *ConnectionContext) IsCompressionAlgorithmSupported(
	algorithm protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE,
) bool {
	return compressionAlgorithmSupported(algorithm)
}

// SetupCryptoWithKey directly sets the encryption key and IV, skipping key exchange.
// This is primarily used for testing purposes.
func (cc *ConnectionContext) SetupCryptoWithKey(
	algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, key []byte, iv []byte,
) error_code.ErrorType {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closing {
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}

	// Set up read crypto session
	if err := cc.readCrypto.SetKey(key, iv, algorithm); err != nil {
		return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET
	}

	// Set up write crypto session
	if err := cc.writeCrypto.SetKey(key, iv, algorithm); err != nil {
		return error_code.EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET
	}

	cc.handshakeDone = true
	return error_code.EN_ATBUS_ERR_SUCCESS
}

// allowCrypto checks if a message type allows encryption.
func allowCrypto(bodyType types.MessageBodyType) bool {
	switch bodyType {
	case types.MessageBodyTypeNodeRegisterReq,
		types.MessageBodyTypeNodeRegisterRsp,
		types.MessageBodyTypeNodePingReq,
		types.MessageBodyTypeNodePongRsp,
		types.MessageBodyTypeHandshakeConfirm:
		return false
	default:
		return true
	}
}

// allowCompress checks if compression should be applied.
func allowCompress(messageType protocol.MessageBody_EnMessageTypeID, bodySize int) bool {
	// If body is too small, compression overhead may make it larger
	if bodySize <= 512 {
		return false
	}

	switch messageType {
	case protocol.MessageBody_EnMessageTypeID_DataTransformReq,
		protocol.MessageBody_EnMessageTypeID_DataTransformRsp,
		protocol.MessageBody_EnMessageTypeID_CustomCommandReq,
		protocol.MessageBody_EnMessageTypeID_CustomCommandRsp:
		return bodySize >= 1024
	default:
		return bodySize >= 2048
	}
}

// PackMessage packs a Message into a StaticBufferBlock for transmission.
// This matches the C++ connection_context::pack_message signature.
// Message frame format: vint(header_length) + header + body
func (cc *ConnectionContext) PackMessage(m *types.Message, protocolVersion int32, maxBodySize int) (*buffer.StaticBufferBlock, error_code.ErrorType) {
	cc.mu.RLock()
	if cc.closing {
		cc.mu.RUnlock()
		return nil, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}
	cc.mu.RUnlock()

	if m == nil {
		return nil, error_code.EN_ATBUS_ERR_PARAMS
	}

	// Get body size
	body := m.GetBody()
	var bodySize int
	if body != nil {
		bodySize = proto.Size(body)
	}

	// Determine compression and crypto algorithms based on message type
	compressionAlg := protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE
	cryptoAlg := protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE

	bodyType := m.GetBodyType()
	if allowCrypto(bodyType) && cc.writeCrypto.IsInitialized() {
		cryptoAlg = cc.writeCrypto.Algorithm
	}
	if allowCompress(bodyType, bodySize) {
		compressionAlg = cc.compression.GetAlgorithm()
	}

	// Check body size limits
	if maxBodySize > 0 && bodySize > maxBodySize {
		return nil, error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	// Set head fields
	head := m.MutableHead()
	head.Version = protocolVersion
	head.BodySize = uint64(bodySize)

	// If no compression and no encryption, use simple packing
	if compressionAlg == protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE &&
		cryptoAlg == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		return cc.packMessageOrigin(m)
	}

	return cc.packMessageWith(m, compressionAlg, cryptoAlg)
}

// packMessageOrigin packs a message without compression or encryption.
func (cc *ConnectionContext) packMessageOrigin(m *types.Message) (*buffer.StaticBufferBlock, error_code.ErrorType) {
	head := m.GetHead()
	body := m.GetBody()

	bodySize := int(head.GetBodySize())

	// Serialize head
	headBytes, err := proto.Marshal(head)
	if err != nil {
		return nil, error_code.EN_ATBUS_ERR_PACK
	}
	headSize := len(headBytes)

	// Calculate vint size for head length
	headVintSize := buffer.VintEncodedSize(uint64(headSize))

	// Total size: vint(head_len) + head + body
	totalSize := headVintSize + headSize + bodySize
	buf := buffer.AllocateTemporaryBufferBlock(totalSize)
	if buf == nil {
		return nil, error_code.EN_ATBUS_ERR_MALLOC
	}

	// Write head length as vint
	data := buf.Data()
	vintWritten := buffer.WriteVint(uint64(headSize), data)
	if vintWritten != headVintSize {
		return nil, error_code.EN_ATBUS_ERR_PACK
	}

	// Write head
	copy(data[headVintSize:], headBytes)

	// Write body
	if body != nil && bodySize > 0 {
		bodyBytes, err := proto.Marshal(body)
		if err != nil {
			return nil, error_code.EN_ATBUS_ERR_PACK
		}
		copy(data[headVintSize+headSize:], bodyBytes)
	}

	buf.SetUsed(totalSize)
	return buf, error_code.EN_ATBUS_ERR_SUCCESS
}

// packMessageWith packs a message with compression and/or encryption.
func (cc *ConnectionContext) packMessageWith(m *types.Message, compressionAlg protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE, cryptoAlg protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) (*buffer.StaticBufferBlock, error_code.ErrorType) {
	head := m.MutableHead()
	body := m.GetBody()

	bodySize := int(head.GetBodySize())

	// Serialize body first
	var originBodyBytes []byte
	if body != nil && bodySize > 0 {
		var err error
		originBodyBytes, err = proto.Marshal(body)
		if err != nil {
			return nil, error_code.EN_ATBUS_ERR_PACK
		}
	}

	processedBody := originBodyBytes
	compressedSize := 0

	// Step 1: Compress if needed
	if compressionAlg != protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE && len(originBodyBytes) > 0 {
		if !compressionAlgorithmSupported(compressionAlg) {
			return nil, error_code.EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT
		}

		compressed, err := cc.compression.Compress(originBodyBytes)
		if err != nil {
			if errors.Is(err, ErrCompressionNotSupported) {
				return nil, error_code.EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT
			}
			return nil, error_code.EN_ATBUS_ERR_PACK
		}

		if len(compressed) > 0 && len(compressed) < len(originBodyBytes) {
			processedBody = compressed
			compressedSize = len(compressed)
		} else {
			compressionAlg = protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE
		}
	}

	// Step 2: Encrypt if needed
	if cryptoAlg != protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		if !cc.writeCrypto.IsInitialized() || cryptoAlg != cc.writeCrypto.Algorithm {
			return nil, error_code.EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_MATCH
		}

		// Set crypto info in head (preserve existing AAD if already set)
		if head.Crypto == nil {
			head.Crypto = &protocol.MessageHeadCrypto{}
		}
		head.Crypto.Algorithm = cryptoAlg

		if cryptoAlgorithmIsAEAD(cryptoAlg) {
			nonceSize := cryptoAlgorithmIVSize(cryptoAlg)
			iv := make([]byte, nonceSize)
			if _, err := rand.Read(iv); err != nil {
				return nil, error_code.EN_ATBUS_ERR_CRYPTO_ENCRYPT
			}
			head.Crypto.Iv = iv

			encrypted, err := cc.writeCrypto.EncryptWithIVAndAAD(processedBody, iv, head.Crypto.Aad)
			if err != nil {
				return nil, error_code.EN_ATBUS_ERR_CRYPTO_ENCRYPT
			}
			processedBody = encrypted
		} else {
			ivSize := cryptoAlgorithmIVSize(cryptoAlg)
			if ivSize > 0 {
				iv := make([]byte, ivSize)
				if _, err := rand.Read(iv); err != nil {
					return nil, error_code.EN_ATBUS_ERR_CRYPTO_ENCRYPT
				}
				head.Crypto.Iv = iv

				encrypted, err := cc.writeCrypto.EncryptWithIV(processedBody, iv)
				if err != nil {
					return nil, error_code.EN_ATBUS_ERR_CRYPTO_ENCRYPT
				}
				processedBody = encrypted
			} else {
				head.Crypto.Iv = nil
				encrypted, err := cc.writeCrypto.Encrypt(processedBody)
				if err != nil {
					return nil, error_code.EN_ATBUS_ERR_CRYPTO_ENCRYPT
				}
				processedBody = encrypted
			}
		}
	}

	// Set compression info if used (for future)
	if compressionAlg != protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE {
		head.Compression = &protocol.MessageHeadCompression{
			Type:         compressionAlg,
			OriginalSize: uint64(compressedSize),
		}
	}

	// Serialize head (after setting crypto/compression info)
	headBytes, err := proto.Marshal(head)
	if err != nil {
		return nil, error_code.EN_ATBUS_ERR_PACK
	}
	headSize := len(headBytes)
	headVintSize := buffer.VintEncodedSize(uint64(headSize))

	// Total size: vint(head_len) + head + encrypted_body
	totalSize := headVintSize + headSize + len(processedBody)
	buf := buffer.AllocateTemporaryBufferBlock(totalSize)
	if buf == nil {
		return nil, error_code.EN_ATBUS_ERR_MALLOC
	}

	data := buf.Data()

	// Write head length as vint
	buffer.WriteVint(uint64(headSize), data)

	// Write head
	copy(data[headVintSize:], headBytes)

	// Write processed body
	copy(data[headVintSize+headSize:], processedBody)

	buf.SetUsed(headVintSize + headSize + len(processedBody))
	return buf, error_code.EN_ATBUS_ERR_SUCCESS
}

// UnpackMessage unpacks binary data into a Message.
// This matches the C++ connection_context::unpack_message signature.
// Message frame format: vint(header_length) + header + body
func (cc *ConnectionContext) UnpackMessage(m *types.Message, input []byte, maxBodySize int) error_code.ErrorType {
	cc.mu.RLock()
	if cc.closing {
		cc.mu.RUnlock()
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}
	cc.mu.RUnlock()

	if m == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if len(input) == 0 {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// Check max size early
	if maxBodySize > 0 && len(input) > maxBodySize+256 { // 256 bytes for header overhead
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	// Read head length vint
	headSize, headVintSize := buffer.ReadVint(input)
	if headVintSize == 0 {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	if int(headSize)+headVintSize > len(input) {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	// Parse head
	head := m.MutableHead()
	if err := proto.Unmarshal(input[headVintSize:headVintSize+int(headSize)], head); err != nil {
		return error_code.EN_ATBUS_ERR_UNPACK
	}

	bodySize := head.GetBodySize()
	if bodySize == 0 {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	if maxBodySize > 0 && int(bodySize) > maxBodySize+256 {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	// Get the body data block
	nextBlock := input[headVintSize+int(headSize):]

	// Step 1: Decrypt if needed
	cryptoInfo := head.GetCrypto()
	if cryptoInfo != nil && cryptoInfo.GetAlgorithm() != protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		if !cc.readCrypto.IsInitialized() || cryptoInfo.GetAlgorithm() != cc.readCrypto.Algorithm {
			return error_code.EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_MATCH
		}

		if cryptoAlgorithmIsAEAD(cryptoInfo.GetAlgorithm()) {
			iv := cryptoInfo.GetIv()
			if len(iv) != cryptoAlgorithmIVSize(cryptoInfo.GetAlgorithm()) {
				return error_code.EN_ATBUS_ERR_CRYPTO_DECRYPT
			}
			decrypted, err := cc.readCrypto.DecryptWithIVAndAAD(nextBlock, iv, cryptoInfo.GetAad())
			if err != nil {
				return error_code.EN_ATBUS_ERR_CRYPTO_DECRYPT
			}
			nextBlock = decrypted
		} else {
			ivSize := cryptoAlgorithmIVSize(cryptoInfo.GetAlgorithm())
			if ivSize > 0 {
				iv := cryptoInfo.GetIv()
				if len(iv) != ivSize {
					return error_code.EN_ATBUS_ERR_CRYPTO_DECRYPT
				}
				decrypted, err := cc.readCrypto.DecryptWithIV(nextBlock, iv)
				if err != nil {
					return error_code.EN_ATBUS_ERR_CRYPTO_DECRYPT
				}
				nextBlock = decrypted
			} else {
				decrypted, err := cc.readCrypto.Decrypt(nextBlock)
				if err != nil {
					return error_code.EN_ATBUS_ERR_CRYPTO_DECRYPT
				}
				nextBlock = decrypted
			}
		}
	}

	// Step 2: Decompress if needed
	compressionInfo := head.GetCompression()
	if compressionInfo != nil && compressionInfo.GetType() != protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE {
		if !compressionAlgorithmSupported(compressionInfo.GetType()) {
			return error_code.EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT
		}

		compressedSize := int(compressionInfo.GetOriginalSize())
		if compressedSize <= 0 || compressedSize > len(nextBlock) {
			return error_code.EN_ATBUS_ERR_INVALID_SIZE
		}

		compressedBlock := nextBlock[:compressedSize]
		adapterAlg := compressionAlgorithmToAdapter(compressionInfo.GetType())
		decompressed, code := compression.Decompress(adapterAlg, compressedBlock, int(bodySize))
		switch code {
		case compression.ErrorCodeOk:
			nextBlock = decompressed
		case compression.ErrorCodeNotSupport, compression.ErrorCodeDisabled:
			return error_code.EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT
		default:
			return error_code.EN_ATBUS_ERR_UNPACK
		}
	}

	// Parse body
	if int(bodySize) > len(nextBlock) {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	body := m.MutableBody()
	if err := proto.Unmarshal(nextBlock[:bodySize], body); err != nil {
		return error_code.EN_ATBUS_ERR_UNPACK
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func init() {
	types.InternalSetDelegateIsCompressionAlgorithmSupported(func(algorithm protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) bool {
		return compressionAlgorithmSupported(algorithm)
	})
}
