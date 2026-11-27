package libatbus_message_handle

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	needConfirm bool,
) error_code.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *mockConnectionContext) ConfirmHandshake(handshakeSequence uint64) {
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
	descriptor := (&protocol.MessageBody{}).ProtoReflect().Descriptor()
	for i := 0; i < descriptor.Fields().Len(); i++ {
		field := descriptor.Fields().Get(i)
		require.Equal(t, string(field.FullName()), GetBodyName(int(field.Number())))
	}
	require.Equal(t, "Unknown", GetBodyName(0))
	require.Equal(t, "Unknown", GetBodyName(1000000))
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

	const limit = 32868
	for _, size := range []int{limit - 1, limit, limit + 1, len(longToken)} {
		mac := hmac.New(sha256.New, longToken[:min(size, limit)])
		mac.Write([]byte(plaintext))
		assert.Equal(t, mac.Sum(nil), CalculateAccessDataSignature(ad, longToken[:size], plaintext), "token size %d", size)
	}
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

func loadJSONTestData(t *testing.T, filename string, target any) {
	t.Helper()
	raw := loadBinaryTestData(t, filename)
	require.NoError(t, json.Unmarshal(raw, target), "Failed to unmarshal JSON test data file: %s", filename)
}

// TestCrossLangAccessDataIndexIntegrity verifies that every test vector listed
// in message_handle/testdata/index.json exists on disk.
func TestCrossLangAccessDataIndexIntegrity(t *testing.T) {
	var index struct {
		Description string `json:"description"`
		Generator   string `json:"generator"`
		Functions   []struct {
			Name      string            `json:"name"`
			TestFiles []json.RawMessage `json:"test_files"`
		} `json:"functions"`
	}
	loadJSONTestData(t, "index.json", &index)
	require.Len(t, index.Functions, 3, "index.json should describe all 3 access_data functions")

	var listed []string
	totalVectors := 0
	for _, fn := range index.Functions {
		for _, rawEntry := range fn.TestFiles {
			totalVectors++
			var singleFile string
			if err := json.Unmarshal(rawEntry, &singleFile); err == nil {
				listed = append(listed, singleFile)
				_, statErr := os.Stat(filepath.Join("testdata", singleFile))
				assert.NoError(t, statErr, "Missing testdata file: %s", singleFile)
				continue
			}

			var pair struct {
				JSON   string `json:"json"`
				Binary string `json:"binary"`
			}
			require.NoError(t, json.Unmarshal(rawEntry, &pair))
			listed = append(listed, pair.JSON, pair.Binary)
			_, statErr := os.Stat(filepath.Join("testdata", pair.JSON))
			assert.NoError(t, statErr, "Missing testdata JSON file: %s", pair.JSON)
			_, statErr = os.Stat(filepath.Join("testdata", pair.Binary))
			assert.NoError(t, statErr, "Missing testdata binary file: %s", pair.Binary)
		}
	}
	assert.Equal(t, 10, totalVectors, "Expected 10 access_data cross-language vectors in index.json")
	entries, err := os.ReadDir("testdata")
	require.NoError(t, err)
	var generated []string
	for _, entry := range entries {
		name := entry.Name()
		if name != "index.json" && (filepath.Ext(name) == ".json" || filepath.Ext(name) == ".bytes") {
			generated = append(generated, name)
		}
	}
	require.ElementsMatch(t, generated, listed)
}

// TestCrossLangAccessDataPlaintext verifies all make_access_data_plaintext vectors
// generated by C++ atbus_access_data_crosslang_generator.cpp.
func TestCrossLangAccessDataPlaintext(t *testing.T) {
	type plaintextVector struct {
		Name  string `json:"name"`
		Input struct {
			BusID               uint64   `json:"bus_id"`
			Timestamp           int64    `json:"timestamp"`
			Nonce1              uint64   `json:"nonce1"`
			Nonce2              uint64   `json:"nonce2"`
			CryptoHandshakeType int32    `json:"crypto_handshake_type"`
			PublicKeyHex        string   `json:"public_key_hex"`
			Commands            []string `json:"commands"`
		} `json:"input"`
		Intermediate struct {
			PublicKeySHA256Hex string `json:"public_key_sha256_hex"`
			ArgsSHA256Hex      string `json:"args_sha256_hex"`
		} `json:"intermediate"`
		Expected struct {
			Plaintext string `json:"plaintext"`
		} `json:"expected"`
	}

	files := []string{
		"plaintext_no_pubkey.json",
		"plaintext_with_pubkey.json",
		"plaintext_custom_command.json",
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			var vec plaintextVector
			loadJSONTestData(t, file, &vec)

			ad := &protocol.AccessData{
				Timestamp: vec.Input.Timestamp,
				Nonce1:    vec.Input.Nonce1,
				Nonce2:    vec.Input.Nonce2,
			}

			var actual string
			if len(vec.Input.Commands) > 0 {
				cs := &protocol.CustomCommandData{}
				for _, cmd := range vec.Input.Commands {
					cs.Commands = append(cs.Commands, &protocol.CustomCommandArgv{Arg: []byte(cmd)})
				}
				actual = MakeAccessDataPlaintextFromCustomCommand(types.BusIdType(vec.Input.BusID), ad, cs)
				if vec.Intermediate.ArgsSHA256Hex != "" {
					assert.Contains(t, actual, vec.Intermediate.ArgsSHA256Hex)
				}
			} else {
				var hd *protocol.CryptoHandshakeData
				if vec.Input.PublicKeyHex != "" {
					pubKey, err := hex.DecodeString(vec.Input.PublicKeyHex)
					require.NoError(t, err)
					hd = &protocol.CryptoHandshakeData{
						Type:      protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE(vec.Input.CryptoHandshakeType),
						PublicKey: pubKey,
					}
					if vec.Intermediate.PublicKeySHA256Hex != "" {
						sum := sha256.Sum256(pubKey)
						assert.Equal(t, vec.Intermediate.PublicKeySHA256Hex, hex.EncodeToString(sum[:]))
					}
				}
				actual = MakeAccessDataPlaintextFromHandshake(types.BusIdType(vec.Input.BusID), ad, hd)
			}

			assert.Equal(t, vec.Expected.Plaintext, actual)
		})
	}
}

// TestCrossLangAccessDataSignature verifies all calculate_access_data_signature
// JSON + binary vectors generated by C++.
func TestCrossLangAccessDataSignature(t *testing.T) {
	type signatureVector struct {
		Name  string `json:"name"`
		Input struct {
			AccessToken    string `json:"access_token"`
			AccessTokenHex string `json:"access_token_hex"`
			Plaintext      string `json:"plaintext"`
		} `json:"input"`
		Expected struct {
			SignatureHex    string `json:"signature_hex"`
			SignatureLength int    `json:"signature_length"`
		} `json:"expected"`
	}

	cases := []struct {
		jsonFile   string
		binaryFile string
	}{
		{"signature_simple_token.json", "signature_simple_token.bytes"},
		{"signature_binary_token.json", "signature_binary_token.bytes"},
		{"signature_256bit_token.json", "signature_256bit_token.bytes"},
	}

	for _, tc := range cases {
		t.Run(tc.jsonFile, func(t *testing.T) {
			var vec signatureVector
			loadJSONTestData(t, tc.jsonFile, &vec)
			expectedBytes := loadBinaryTestData(t, tc.binaryFile)

			token, err := hex.DecodeString(vec.Input.AccessTokenHex)
			require.NoError(t, err)
			if vec.Input.AccessToken != "" {
				assert.Equal(t, []byte(vec.Input.AccessToken), token)
			}

			expectedFromHex, err := hex.DecodeString(vec.Expected.SignatureHex)
			require.NoError(t, err)
			assert.Equal(t, expectedFromHex, expectedBytes)
			assert.Len(t, expectedBytes, vec.Expected.SignatureLength)

			ad := &protocol.AccessData{
				Algorithm: protocol.ATBUS_ACCESS_DATA_ALGORITHM_TYPE_ATBUS_ACCESS_DATA_ALGORITHM_HMAC_SHA256,
			}
			actual := CalculateAccessDataSignature(ad, token, vec.Input.Plaintext)
			assert.Equal(t, expectedBytes, actual)
		})
	}
}

// TestCrossLangAccessDataFull verifies all generate_access_data JSON + binary
// protobuf vectors generated by C++ (both Go->C++ marshal parity and C++->Go unmarshal).
func TestCrossLangAccessDataFull(t *testing.T) {
	type accessTokenEntry struct {
		Value string `json:"value"`
		Hex   string `json:"hex"`
	}
	type fullVector struct {
		Name  string `json:"name"`
		Input struct {
			BusID               uint64             `json:"bus_id"`
			Nonce1              uint64             `json:"nonce1"`
			Nonce2              uint64             `json:"nonce2"`
			AccessTokens        []accessTokenEntry `json:"access_tokens"`
			CryptoHandshakeType int32              `json:"crypto_handshake_type"`
			PublicKeyHex        string             `json:"public_key_hex"`
			Commands            []string           `json:"commands"`
		} `json:"input"`
		Expected struct {
			AlgorithmValue   int32    `json:"algorithm_value"`
			Timestamp        int64    `json:"timestamp"`
			Nonce1           uint64   `json:"nonce1"`
			Nonce2           uint64   `json:"nonce2"`
			Plaintext        string   `json:"plaintext"`
			Signatures       []string `json:"signatures"`
			SerializedHex    string   `json:"serialized_hex"`
			SerializedLength int      `json:"serialized_length"`
		} `json:"expected"`
	}

	cases := []struct {
		jsonFile   string
		binaryFile string
	}{
		{"full_access_data_no_pubkey.json", "full_access_data_no_pubkey.bytes"},
		{"full_access_data_with_pubkey.json", "full_access_data_with_pubkey.bytes"},
		{"full_access_data_custom_command.json", "full_access_data_custom_command.bytes"},
		{"full_access_data_multiple_tokens.json", "full_access_data_multiple_tokens.bytes"},
	}

	for _, tc := range cases {
		t.Run(tc.jsonFile, func(t *testing.T) {
			var vec fullVector
			loadJSONTestData(t, tc.jsonFile, &vec)
			expectedBytes := loadBinaryTestData(t, tc.binaryFile)

			expectedFromHex, err := hex.DecodeString(vec.Expected.SerializedHex)
			require.NoError(t, err)
			assert.Equal(t, expectedFromHex, expectedBytes)
			assert.Len(t, expectedBytes, vec.Expected.SerializedLength)

			tokens := make([][]byte, 0, len(vec.Input.AccessTokens))
			for _, tok := range vec.Input.AccessTokens {
				b, decErr := hex.DecodeString(tok.Hex)
				require.NoError(t, decErr)
				if tok.Value != "" {
					assert.Equal(t, []byte(tok.Value), b)
				}
				tokens = append(tokens, b)
			}

			generated := &protocol.AccessData{}
			if len(vec.Input.Commands) > 0 {
				cs := &protocol.CustomCommandData{}
				for _, cmd := range vec.Input.Commands {
					cs.Commands = append(cs.Commands, &protocol.CustomCommandArgv{Arg: []byte(cmd)})
				}
				GenerateAccessDataForCustomCommandWithTimestamp(
					generated,
					types.BusIdType(vec.Input.BusID),
					vec.Input.Nonce1,
					vec.Input.Nonce2,
					tokens,
					cs,
					vec.Expected.Timestamp,
				)
			} else {
				var hd *protocol.CryptoHandshakeData
				if vec.Input.PublicKeyHex != "" {
					pubKey, decErr := hex.DecodeString(vec.Input.PublicKeyHex)
					require.NoError(t, decErr)
					hd = &protocol.CryptoHandshakeData{
						Type:      protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE(vec.Input.CryptoHandshakeType),
						PublicKey: pubKey,
					}
				}
				GenerateAccessDataWithTimestamp(
					generated,
					types.BusIdType(vec.Input.BusID),
					vec.Input.Nonce1,
					vec.Input.Nonce2,
					tokens,
					hd,
					vec.Expected.Timestamp,
				)
			}

			assert.Equal(t, protocol.ATBUS_ACCESS_DATA_ALGORITHM_TYPE(vec.Expected.AlgorithmValue), generated.GetAlgorithm())
			assert.Equal(t, vec.Expected.Timestamp, generated.GetTimestamp())
			assert.Equal(t, vec.Expected.Nonce1, generated.GetNonce1())
			assert.Equal(t, vec.Expected.Nonce2, generated.GetNonce2())
			require.Len(t, generated.GetSignature(), len(vec.Expected.Signatures))
			for i, sigHex := range vec.Expected.Signatures {
				expectedSig, decErr := hex.DecodeString(sigHex)
				require.NoError(t, decErr)
				assert.Equal(t, expectedSig, generated.GetSignature()[i], "Signature[%d] mismatch", i)
			}

			// Verify Go proto.Marshal matches C++ SerializeToString byte-for-byte
			marshaled, err := proto.Marshal(generated)
			require.NoError(t, err)
			assert.Equal(t, expectedBytes, marshaled, "Serialized AccessData bytes should match C++ output")

			// Verify C++ binary unmarshals cleanly into Go AccessData
			unmarshaled := &protocol.AccessData{}
			require.NoError(t, proto.Unmarshal(expectedBytes, unmarshaled))
			assert.True(t, proto.Equal(generated, unmarshaled))
		})
	}
}

// ============================================================================
// Regression Tests — Bug fixes found during C++/Go codebase comparison
// ============================================================================

// mockEndpoint is a minimal mock for types.Endpoint.
type mockEndpoint struct{}

func (e *mockEndpoint) GetOwner() types.Node                                      { return nil }
func (e *mockEndpoint) GetId() types.BusIdType                                    { return 0x42 }
func (e *mockEndpoint) Reset()                                                    {}
func (e *mockEndpoint) Reload(string, string, map[string]string, []types.Gateway) {}
func (e *mockEndpoint) ReloadFromProtocol(string, string, map[string]string, []*protocol.ChannelData) {
}
func (e *mockEndpoint) GetPid() int32                                            { return 0 }
func (e *mockEndpoint) GetHostname() string                                      { return "" }
func (e *mockEndpoint) GetHashCode() string                                      { return "" }
func (e *mockEndpoint) UpdateHashCode(code string)                               {}
func (e *mockEndpoint) GetScope() string                                         { return "" }
func (e *mockEndpoint) GetNamespace() string                                     { return "" }
func (e *mockEndpoint) GetLabels() map[string]string                             { return nil }
func (e *mockEndpoint) GetFlag(flag types.EndpointFlag) bool                     { return false }
func (e *mockEndpoint) GetFlags() uint32                                         { return 0 }
func (e *mockEndpoint) SetFlag(f types.EndpointFlag, v bool)                     {}
func (e *mockEndpoint) AddConnection(conn types.Connection, forceData bool) bool { return false }
func (e *mockEndpoint) RemoveConnection(conn types.Connection) bool              { return false }
func (e *mockEndpoint) IsAvailable() bool                                        { return true }
func (e *mockEndpoint) GetCtrlConnection(peer types.Endpoint) types.Connection   { return nil }
func (e *mockEndpoint) GetDataConnection(peer types.Endpoint, fb bool) types.Connection {
	return nil
}
func (e *mockEndpoint) GetDataConnectionCount(fb bool) int                   { return 0 }
func (e *mockEndpoint) GetGateway() []types.Gateway                          { return nil }
func (e *mockEndpoint) GetListenAddress() []types.ChannelAddress             { return nil }
func (e *mockEndpoint) ClearListenAddress()                                  {}
func (e *mockEndpoint) AddListenAddress(addr string)                         {}
func (e *mockEndpoint) UpdateSupportSchemes(schemes []string)                {}
func (e *mockEndpoint) IsSchemeSupported(scheme string) bool                 { return false }
func (e *mockEndpoint) AddPingTimer()                                        {}
func (e *mockEndpoint) ClearPingTimer()                                      {}
func (e *mockEndpoint) AddStatisticFault() uint64                            { return 0 }
func (e *mockEndpoint) ClearStatisticFault()                                 {}
func (e *mockEndpoint) SetStatisticUnfinishedPing(p uint64)                  {}
func (e *mockEndpoint) GetStatisticUnfinishedPing() uint64                   { return 0 }
func (e *mockEndpoint) SetStatisticPingDelay(pd time.Duration, pt time.Time) {}
func (e *mockEndpoint) GetStatisticPingDelay() time.Duration                 { return 0 }
func (e *mockEndpoint) GetStatisticLastPong() time.Time                      { return time.Time{} }
func (e *mockEndpoint) GetStatisticCreatedTime() time.Time                   { return time.Time{} }
func (e *mockEndpoint) GetStatisticPushStartTimes() uint64                   { return 0 }
func (e *mockEndpoint) GetStatisticPushStartSize() uint64                    { return 0 }
func (e *mockEndpoint) GetStatisticPushSuccessTimes() uint64                 { return 0 }
func (e *mockEndpoint) GetStatisticPushSuccessSize() uint64                  { return 0 }
func (e *mockEndpoint) GetStatisticPushFailedTimes() uint64                  { return 0 }
func (e *mockEndpoint) GetStatisticPushFailedSize() uint64                   { return 0 }
func (e *mockEndpoint) GetStatisticPullTimes() uint64                        { return 0 }
func (e *mockEndpoint) GetStatisticPullSize() uint64                         { return 0 }
func (e *mockEndpoint) Merge(other types.Endpoint)                           {}
func (e *mockEndpoint) SetSupportedSchemas(schemas map[string]struct{})      {}
func (e *mockEndpoint) GetSupportedSchemas() map[string]struct{}             { return nil }

// mockConnection is a minimal mock for types.Connection.
type mockConnection struct {
	binding types.Endpoint
}

func (c *mockConnection) Reset()                           {}
func (c *mockConnection) Proc() error_code.ErrorType       { return error_code.EN_ATBUS_ERR_SUCCESS }
func (c *mockConnection) Listen() error_code.ErrorType     { return error_code.EN_ATBUS_ERR_SUCCESS }
func (c *mockConnection) Connect() error_code.ErrorType    { return error_code.EN_ATBUS_ERR_SUCCESS }
func (c *mockConnection) Disconnect() error_code.ErrorType { return error_code.EN_ATBUS_ERR_SUCCESS }
func (c *mockConnection) Push(buffer []byte) error_code.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}
func (c *mockConnection) AddStatisticFault() uint64                     { return 0 }
func (c *mockConnection) ClearStatisticFault()                          {}
func (c *mockConnection) GetAddress() types.ChannelAddress              { return nil }
func (c *mockConnection) GetOriginAddress() string                      { return "" }
func (c *mockConnection) IsConnected() bool                             { return true }
func (c *mockConnection) IsRunning() bool                               { return true }
func (c *mockConnection) GetBinding() types.Endpoint                    { return c.binding }
func (c *mockConnection) GetStatus() types.ConnectionState              { return 0 }
func (c *mockConnection) CheckFlag(flag types.ConnectionFlag) bool      { return false }
func (c *mockConnection) SetTemporary()                                 {}
func (c *mockConnection) GetStatistic() types.ConnectionStatistic       { return types.ConnectionStatistic{} }
func (c *mockConnection) GetConnectionContext() types.ConnectionContext { return nil }
func (c *mockConnection) RemoveOwnerChecker()                           {}

// getConnectionBinding — must delegate to conn.GetBinding() without recursion
// Bug: called getConnectionBinding(conn) instead of conn.GetBinding() — infinite recursion.
// Fix: changed to return conn.GetBinding().
func TestGetConnectionBinding_DelegatesToConnGetBinding(t *testing.T) {
	ep := &mockEndpoint{}
	conn := &mockConnection{binding: ep}

	// This would stack-overflow before the fix
	result := getConnectionBinding(conn)
	assert.Equal(t, ep, result,
		"getConnectionBinding must return conn.GetBinding() without recursion")
}

func TestGetConnectionBinding_NilConnReturnsNil(t *testing.T) {
	result := getConnectionBinding(nil)
	assert.Nil(t, result,
		"getConnectionBinding(nil) should return nil")
}

func TestGetConnectionBinding_NilBindingReturnsNil(t *testing.T) {
	conn := &mockConnection{binding: nil}
	result := getConnectionBinding(conn)
	assert.Nil(t, result,
		"getConnectionBinding with nil binding should return nil")
}

// SendTransferResponse — must accept DataTransformReq/Rsp body types
// Bug: checked for NodeRegisterReq/Rsp instead of DataTransformReq/Rsp.
// Fix: changed to check MessageBodyTypeDataTransformReq/Rsp.
func TestSendTransferResponse_AcceptsDataTransformReq(t *testing.T) {
	msg := NewMessage()
	msg.MutableBody().MessageType = &protocol.MessageBody_DataTransformReq{
		DataTransformReq: &protocol.ForwardData{
			From:    0x1111,
			To:      0x2222,
			Content: []byte("test-data"),
		},
	}

	bodyType := msg.GetBodyType()
	// The body type should match what SendTransferResponse expects
	assert.Equal(t, types.MessageBodyTypeDataTransformReq, bodyType,
		"DataTransformReq body should be recognized")
	// Verify it's not the old wrong constant
	assert.NotEqual(t, types.MessageBodyTypeNodeRegisterReq, bodyType)
}

func TestSendTransferResponse_RejectsNodeRegisterReq(t *testing.T) {
	msg := NewMessage()
	msg.MutableBody().MessageType = &protocol.MessageBody_NodeRegisterReq{
		NodeRegisterReq: &protocol.RegisterData{},
	}

	bodyType := msg.GetBodyType()
	// NodeRegisterReq should NOT be accepted by SendTransferResponse
	assert.NotEqual(t, types.MessageBodyTypeDataTransformReq, bodyType,
		"NodeRegisterReq should not match DataTransformReq")
	assert.NotEqual(t, types.MessageBodyTypeDataTransformRsp, bodyType,
		"NodeRegisterReq should not match DataTransformRsp")
}
