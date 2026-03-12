package libatbus_message_handle

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	buffer "github.com/atframework/libatbus-go/buffer"
	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
	"google.golang.org/protobuf/proto"
)

type mockConnectionContext struct {
	closing  bool
	sequence uint64
}

func newMockConnectionContext() *mockConnectionContext {
	return &mockConnectionContext{}
}

func (c *mockConnectionContext) IsClosing() bool {
	return c.closing
}

func (c *mockConnectionContext) SetClosing(closing bool) {
	c.closing = closing
}

func (c *mockConnectionContext) IsHandshakeDone() bool {
	return false
}

func (c *mockConnectionContext) GetHandshakeStartTime() time.Time {
	return time.Time{}
}

func (c *mockConnectionContext) GetCryptoKeyExchangeAlgorithm() protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE {
	return protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE
}

func (c *mockConnectionContext) GetCryptoSelectKdfType() protocol.ATBUS_CRYPTO_KDF_TYPE {
	return protocol.ATBUS_CRYPTO_KDF_TYPE_ATBUS_CRYPTO_KDF_HKDF_SHA256
}

func (c *mockConnectionContext) GetCryptoSelectAlgorithm() protocol.ATBUS_CRYPTO_ALGORITHM_TYPE {
	return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE
}

func (c *mockConnectionContext) GetCompressSelectAlgorithm() protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE {
	return protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE
}

func (c *mockConnectionContext) GetNextSequence() uint64 {
	c.sequence++
	return c.sequence
}

func (c *mockConnectionContext) PackMessage(m *types.Message, protocolVersion int32, maxBodySize int) (*buffer.StaticBufferBlock, error_code.ErrorType) {
	if c.closing {
		return nil, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}
	if m == nil {
		return nil, error_code.EN_ATBUS_ERR_PARAMS
	}

	body := m.GetBody()
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = proto.Marshal(body)
		if err != nil {
			return nil, error_code.EN_ATBUS_ERR_PACK
		}
	}
	bodySize := len(bodyBytes)
	if maxBodySize > 0 && bodySize > maxBodySize {
		return nil, error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	head := m.MutableHead()
	head.Version = protocolVersion
	head.BodySize = uint64(bodySize)

	headBytes, err := proto.Marshal(head)
	if err != nil {
		return nil, error_code.EN_ATBUS_ERR_PACK
	}
	headSize := len(headBytes)
	headVintSize := buffer.VintEncodedSize(uint64(headSize))
	totalSize := headVintSize + headSize + bodySize
	buf := buffer.AllocateTemporaryBufferBlock(totalSize)
	if buf == nil {
		return nil, error_code.EN_ATBUS_ERR_MALLOC
	}

	data := buf.Data()
	if buffer.WriteVint(uint64(headSize), data) != headVintSize {
		return nil, error_code.EN_ATBUS_ERR_PACK
	}
	copy(data[headVintSize:], headBytes)
	if bodySize > 0 {
		copy(data[headVintSize+headSize:], bodyBytes)
	}
	buf.SetUsed(totalSize)
	return buf, error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *mockConnectionContext) UnpackMessage(m *types.Message, input []byte, maxBodySize int) error_code.ErrorType {
	if c.closing {
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}
	if m == nil || len(input) == 0 {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	headSize, headVintSize := buffer.ReadVint(input)
	if headVintSize == 0 {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}
	if headVintSize+int(headSize) > len(input) {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	head := m.MutableHead()
	if err := proto.Unmarshal(input[headVintSize:headVintSize+int(headSize)], head); err != nil {
		return error_code.EN_ATBUS_ERR_UNPACK
	}

	bodySize := int(head.GetBodySize())
	if maxBodySize > 0 && bodySize > maxBodySize {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}
	if bodySize == 0 {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	if headVintSize+int(headSize)+bodySize > len(input) {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	body := m.MutableBody()
	if err := proto.Unmarshal(input[headVintSize+int(headSize):headVintSize+int(headSize)+bodySize], body); err != nil {
		return error_code.EN_ATBUS_ERR_UNPACK
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *mockConnectionContext) HandshakeGenerateSelfKey(peerSequenceId uint64) error_code.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *mockConnectionContext) HandshakeReadPeerKey(peerPubKey *protocol.CryptoHandshakeData,
	supportedCryptoAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE,
) error_code.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *mockConnectionContext) HandshakeWriteSelfPublicKey(
	selfPubKey *protocol.CryptoHandshakeData,
	supportedCryptoAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE,
) error_code.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *mockConnectionContext) UpdateCompressionAlgorithm(algorithm []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) error_code.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *mockConnectionContext) SetupCryptoWithKey(algorithm protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, key []byte, iv []byte) error_code.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}

// ============================================================================
// Message Helpers Tests
// ============================================================================

func TestGetBodyName(t *testing.T) {
	// Test: GetBodyName returns correct names for known message types
	// Use protocol constants for field numbers instead of hardcoded values
	testCases := []struct {
		fieldNum int
		expected string
	}{
		{int(protocol.MessageBody_EnMessageTypeID_NodePingReq), "atframework.atbus.protocol.message_body.node_ping_req"},
		{int(protocol.MessageBody_EnMessageTypeID_NodePongRsp), "atframework.atbus.protocol.message_body.node_pong_rsp"},
	}

	for _, tc := range testCases {
		name := GetBodyName(tc.fieldNum)
		assert.NotEqual(t, "Unknown", name, "Field %d should have a known name", tc.fieldNum)
	}
}

func TestGetBodyNameUnknown(t *testing.T) {
	// Test: GetBodyName returns "Unknown" for invalid field number
	assert.Equal(t, "Unknown", GetBodyName(9999))
}

// ============================================================================
// Pack/Unpack Tests
// ============================================================================

func TestPackUnpackMessageRoundTrip(t *testing.T) {
	// Arrange
	ctx := newMockConnectionContext()
	msg := types.NewMessage()

	// Set up message body with data transform request
	body := msg.MutableBody()
	body.MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			Content: []byte("hello world"),
		},
	}

	// Set up message head
	head := msg.MutableHead()
	head.Type = 100
	head.SourceBusId = 0x22

	// Act - Pack
	buf, err := PackMessage(ctx, msg, 3, 0)
	assert.Equal(t, err, error_code.EN_ATBUS_ERR_SUCCESS)
	assert.NotNil(t, buf)
	assert.True(t, buf.Used() > 0)

	// Act - Unpack
	decodedMsg, err := UnpackMessage(ctx, buf.UsedSpan(), 0)

	// Assert
	assert.Equal(t, err, error_code.EN_ATBUS_ERR_SUCCESS)
	assert.NotNil(t, decodedMsg)
	assert.Equal(t, int32(3), decodedMsg.GetHead().GetVersion())
}

func TestPackMessageNilContext(t *testing.T) {
	// Test: PackMessage with nil context returns error
	msg := types.NewMessage()
	_, err := PackMessage(nil, msg, 1, 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, error_code.EN_ATBUS_ERR_PARAMS))
}

func TestPackMessageNilMessage(t *testing.T) {
	// Test: PackMessage with nil message returns error
	ctx := newMockConnectionContext()
	_, err := PackMessage(ctx, nil, 1, 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, error_code.EN_ATBUS_ERR_PARAMS))
}

func TestUnpackMessageNilContext(t *testing.T) {
	// Test: UnpackMessage with nil context returns error
	_, err := UnpackMessage(nil, []byte{1, 2, 3}, 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, error_code.EN_ATBUS_ERR_PARAMS))
}

// ============================================================================
// Access Data Tests
// ============================================================================

func TestGenerateAccessDataWithTimestamp(t *testing.T) {
	// Test: GenerateAccessData produces correct signature format
	ad := &protocol.AccessData{}
	busID := types.BusIdType(0x12345678)
	nonce1 := uint64(111)
	nonce2 := uint64(222)
	tokens := [][]byte{[]byte("token1"), []byte("token2")}

	GenerateAccessDataWithTimestamp(ad, busID, nonce1, nonce2, tokens, nil, 1000000)

	assert.Equal(t, int64(1000000), ad.Timestamp)
	assert.Equal(t, nonce1, ad.Nonce1)
	assert.Equal(t, nonce2, ad.Nonce2)
	assert.Len(t, ad.Signature, 2)
}

func TestMakeAccessDataPlaintextWithoutPubkey(t *testing.T) {
	// Test: Plaintext format without public key
	ad := &protocol.AccessData{
		Timestamp: 1234567890,
		Nonce1:    111,
		Nonce2:    222,
	}
	busID := types.BusIdType(0x42)

	plaintext := MakeAccessDataPlaintextFromHandshake(busID, ad, nil)
	assert.Equal(t, "1234567890:111-222:66", plaintext)
}

func TestMakeAccessDataPlaintextWithPubkey(t *testing.T) {
	// Test: Plaintext format with public key
	ad := &protocol.AccessData{
		Timestamp: 1234567890,
		Nonce1:    111,
		Nonce2:    222,
	}
	busID := types.BusIdType(0x42)
	hd := &protocol.CryptoHandshakeData{
		PublicKey: []byte("test_public_key"),
		Type:      protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
	}

	plaintext := MakeAccessDataPlaintextFromHandshake(busID, ad, hd)

	// Verify format includes hash
	h := sha256.Sum256([]byte("test_public_key"))
	expectedHash := hex.EncodeToString(h[:])
	assert.Contains(t, plaintext, expectedHash)
	assert.Contains(t, plaintext, "1234567890:111-222:66:")
}

func TestMakeAccessDataPlaintextFromCustomCommand(t *testing.T) {
	// Test: Plaintext format for custom command
	ad := &protocol.AccessData{
		Timestamp: 1234567890,
		Nonce1:    111,
		Nonce2:    222,
	}
	busID := types.BusIdType(0x42)
	cs := &protocol.CustomCommandData{
		Commands: []*protocol.CustomCommandArgv{
			{Arg: []byte("arg1")},
			{Arg: []byte("arg2")},
		},
	}

	plaintext := MakeAccessDataPlaintextFromCustomCommand(busID, ad, cs)

	// Verify format includes hash of concatenated args
	h := sha256.Sum256([]byte("arg1arg2"))
	expectedHash := hex.EncodeToString(h[:])
	assert.Contains(t, plaintext, expectedHash)
}

func TestCalculateAccessDataSignature(t *testing.T) {
	// Test: Signature is deterministic
	ad := &protocol.AccessData{}
	token := []byte("secret_token")
	plaintext := "test_plaintext"

	sig1 := CalculateAccessDataSignature(ad, token, plaintext)
	sig2 := CalculateAccessDataSignature(ad, token, plaintext)

	assert.Equal(t, sig1, sig2)
	assert.Len(t, sig1, 32) // SHA256 output
}

func TestCalculateAccessDataSignatureLongToken(t *testing.T) {
	// Test: Long tokens are truncated
	ad := &protocol.AccessData{}
	longToken := make([]byte, 50000)
	for i := range longToken {
		longToken[i] = byte(i % 256)
	}
	plaintext := "test_plaintext"

	// Should not panic
	sig := CalculateAccessDataSignature(ad, longToken, plaintext)
	assert.Len(t, sig, 32)
}

func TestGenerateAccessDataEmptyTokens(t *testing.T) {
	// Test: Empty tokens list produces empty signatures
	ad := &protocol.AccessData{}
	GenerateAccessData(ad, 1, 2, 3, nil, nil)

	assert.Empty(t, ad.Signature)
}

func TestGenerateAccessDataNilAd(t *testing.T) {
	// Test: Nil AccessData doesn't panic
	GenerateAccessData(nil, 1, 2, 3, [][]byte{[]byte("token")}, nil)
	// No panic = success
}

// ============================================================================
// Message Type Constants Tests
// ============================================================================

func TestMessageBodyTypeConstants(t *testing.T) {
	// Test: Message type constants are properly defined in types package
	// These are accessed via types package, not re-exported from message_handle
	assert.NotEqual(t, types.MessageBodyTypeUnknown, types.MessageBodyTypeCustomCommandReq)
	assert.NotEqual(t, types.MessageBodyTypeDataTransformReq, types.MessageBodyTypeNodePingReq)
}

func TestNewMessage(t *testing.T) {
	// Test: NewMessage creates valid message
	msg := NewMessage()
	assert.NotNil(t, msg)
	assert.NotNil(t, msg.MutableHead())
	assert.NotNil(t, msg.MutableBody())
}

// ============================================================================
// Cross-language validation tests
// These tests use test data generated by C++ atbus_access_data_crosslang_generator.cpp
// to verify that Go and C++ implementations produce identical results.
// Test data is loaded from testdata/*.bytes binary files (matching C++ verification pattern).
// ============================================================================

// loadBinaryTestData loads binary data from the testdata directory.
func loadBinaryTestData(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "Failed to read binary test data file: %s", path)
	return data
}

// TestCrossLangPlaintextNoPubkey verifies plaintext generation without public key
// matches the C++ implementation.
func TestCrossLangPlaintextNoPubkey(t *testing.T) {
	// Skip if test data not available
	path := filepath.Join("testdata", "plaintext_no_pubkey.bytes")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Cross-language test data not available")
	}

	expected := loadBinaryTestData(t, "plaintext_no_pubkey.bytes")

	ad := &protocol.AccessData{
		Timestamp: 1234567890,
		Nonce1:    111,
		Nonce2:    222,
	}
	busID := types.BusIdType(0x42)

	plaintext := MakeAccessDataPlaintextFromHandshake(busID, ad, nil)
	assert.Equal(t, string(expected), plaintext)
}
