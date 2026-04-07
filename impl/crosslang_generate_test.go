package libatbus_impl

// This file contains helpers to (re-)generate cross-language test vectors for
// algorithms that cannot be easily produced from the C++ side yet.
//
// The generated test fixtures use the same format as the C++ atbus_connection_context_crosslang_generator.
//
// Execute with:
//   go test -run TestGenerateCrossLangPureChaCha20 -v
//   go test -run TestGenerateCrossLangXChaCha20Poly1305 -v

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	buffer "github.com/atframework/libatbus-go/buffer"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// crossLangGenParam holds parameters for generating a single cross-language test vector.
type crossLangGenParam struct {
	Name        string
	Description string
	Algorithm   protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	AlgoName    string
	KeyHex      string
	IVHex       string
	AADHex      string
}

// buildDataTransformReqBody creates the standard test body used by all cross-lang data_transform_req vectors.
func buildDataTransformReqBody() *protocol.MessageBody {
	return &protocol.MessageBody{
		MessageType: &protocol.MessageBody_DataTransformReq{
			DataTransformReq: &protocol.ForwardData{
				From:    0x123456789ABCDEF0,
				To:      0x0FEDCBA987654321,
				Content: []byte("Hello, encrypted atbus!"),
				Flags:   1,
			},
		},
	}
}

// buildCustomCmdBody creates the standard test body used by all cross-lang custom_cmd vectors.
func buildCustomCmdBody() *protocol.MessageBody {
	return &protocol.MessageBody{
		MessageType: &protocol.MessageBody_CustomCommandReq{
			CustomCommandReq: &protocol.CustomCommandData{
				From: 0xABCDEF0123456789,
				Commands: []*protocol.CustomCommandArgv{
					{Arg: []byte("cmd1")},
					{Arg: []byte("arg1")},
					{Arg: []byte("arg2")},
				},
			},
		},
	}
}

// generateTestVector packs a message body with the given crypto parameters, writes .bytes and .json to testdata/.
func generateTestVector(t *testing.T, param crossLangGenParam, bodyType types.MessageBodyType, body *protocol.MessageBody, bodyTypeName string) {
	t.Helper()

	key, err := hex.DecodeString(param.KeyHex)
	require.NoError(t, err)
	iv, err := hex.DecodeString(param.IVHex)
	require.NoError(t, err)
	var aad []byte
	if param.AADHex != "" {
		aad, err = hex.DecodeString(param.AADHex)
		require.NoError(t, err)
	}

	// Create crypto session and set key
	session := NewCryptoSession()
	require.NoError(t, session.SetKey(key, iv, param.Algorithm))

	// Serialize body
	bodyBytes, err := proto.Marshal(body)
	require.NoError(t, err)
	bodySize := len(bodyBytes)

	// Encrypt
	var encrypted []byte
	head := &protocol.MessageHead{
		Version:  3,
		BodySize: uint64(bodySize),
	}

	if param.Algorithm == protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA {
		encrypted, err = session.Encrypt(bodyBytes)
		require.NoError(t, err)
		head.Crypto = &protocol.MessageHeadCrypto{
			Algorithm: param.Algorithm,
		}
	} else if cryptoAlgorithmIsAEAD(param.Algorithm) {
		encrypted, err = session.EncryptWithIVAndAAD(bodyBytes, iv, aad)
		require.NoError(t, err)
		head.Crypto = &protocol.MessageHeadCrypto{
			Algorithm: param.Algorithm,
			Iv:        iv,
			Aad:       aad,
		}
	} else {
		encrypted, err = session.EncryptWithIV(bodyBytes, iv)
		require.NoError(t, err)
		head.Crypto = &protocol.MessageHeadCrypto{
			Algorithm: param.Algorithm,
			Iv:        iv,
		}
	}

	// Determine body_type_case from body type enum
	bodyTypeCase := int(bodyType)

	// Pack: vint(headLen) + head + encrypted_body
	headBytes, err := proto.Marshal(head)
	require.NoError(t, err)
	headVintSize := buffer.VintEncodedSize(uint64(len(headBytes)))
	totalSize := headVintSize + len(headBytes) + len(encrypted)
	packed := make([]byte, totalSize)
	buffer.WriteVint(uint64(len(headBytes)), packed)
	copy(packed[headVintSize:], headBytes)
	copy(packed[headVintSize+len(headBytes):], encrypted)

	// Build metadata JSON
	meta := crossLangTestMetadata{
		Name:                param.Name,
		Description:         param.Description,
		ProtocolVersion:     3,
		BodyType:            bodyTypeName,
		BodyTypeCase:        bodyTypeCase,
		CryptoAlgorithm:     param.AlgoName,
		CryptoAlgorithmType: int(param.Algorithm),
		KeyHex:              param.KeyHex,
		KeySize:             len(key),
		IVHex:               param.IVHex,
		IVSize:              len(iv),
		AADHex:              param.AADHex,
		AADSize:             len(aad),
		PackedSize:          totalSize,
		PackedHex:           hex.EncodeToString(packed),
	}
	meta.Expected.Flags = 1
	if bodyTypeName == "data_transform_req" {
		meta.Expected.From = 0x123456789ABCDEF0
		meta.Expected.To = 0x0FEDCBA987654321
		meta.Expected.Content = "Hello, encrypted atbus!"
	} else if bodyTypeName == "custom_command_req" {
		meta.Expected.From = 0xABCDEF0123456789
		meta.Expected.Commands = []string{"cmd1", "arg1", "arg2"}
	}

	// Write binary
	binPath := filepath.Join("testdata", param.Name+".bytes")
	require.NoError(t, os.WriteFile(binPath, packed, 0o644))

	// Write JSON
	jsonBytes, err := json.MarshalIndent(meta, "", "  ")
	require.NoError(t, err)
	jsonPath := filepath.Join("testdata", param.Name+".json")
	require.NoError(t, os.WriteFile(jsonPath, jsonBytes, 0o644))

	t.Logf("Generated %s (%d bytes)", param.Name, totalSize)
}

func TestGenerateCrossLangPureChaCha20(t *testing.T) {
	// Only run when explicitly requested (e.g. to regenerate test data)
	if os.Getenv("GENERATE_CROSSLANG_TESTDATA") == "" {
		// Check if the files already exist; if so, skip generation
		if _, err := os.Stat(filepath.Join("testdata", "enc_chacha20_data_transform_req.bytes")); err == nil {
			t.Skip("Test data already exists; set GENERATE_CROSSLANG_TESTDATA=1 to regenerate")
		}
	}

	// Pure ChaCha20: 32-byte key, 12-byte IV, no AAD (stream cipher, not AEAD)
	param := crossLangGenParam{
		AlgoName:  "chacha20",
		Algorithm: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20,
		KeyHex:    "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		IVHex:     "b0b1b2b3b4b5b6b7b8b9babb",
	}

	// data_transform_req
	param.Name = "enc_chacha20_data_transform_req"
	param.Description = "Data transform request with pure chacha20 encryption"
	generateTestVector(t, param, types.MessageBodyTypeDataTransformReq, buildDataTransformReqBody(), "data_transform_req")

	// custom_cmd
	param.Name = "enc_chacha20_custom_cmd"
	param.Description = "Custom command request with pure chacha20 encryption"
	generateTestVector(t, param, types.MessageBodyTypeCustomCommandReq, buildCustomCmdBody(), "custom_command_req")
}

func TestGenerateCrossLangXChaCha20Poly1305(t *testing.T) {
	if os.Getenv("GENERATE_CROSSLANG_TESTDATA") == "" {
		if _, err := os.Stat(filepath.Join("testdata", "enc_xchacha20_poly1305_data_transform_req.bytes")); err == nil {
			t.Skip("Test data already exists; set GENERATE_CROSSLANG_TESTDATA=1 to regenerate")
		}
	}

	// XChaCha20-Poly1305: 32-byte key, 24-byte IV, 16-byte AAD (AEAD)
	param := crossLangGenParam{
		AlgoName:  "xchacha20_poly1305",
		Algorithm: protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF,
		KeyHex:    "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		IVHex:     "c0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7",
		AADHex:    fmt.Sprintf("%032x", uint64(0xdeadbeefcafebabe)),
	}

	// data_transform_req
	param.Name = "enc_xchacha20_poly1305_data_transform_req"
	param.Description = "Data transform request with xchacha20-poly1305 encryption"
	generateTestVector(t, param, types.MessageBodyTypeDataTransformReq, buildDataTransformReqBody(), "data_transform_req")

	// custom_cmd
	param.Name = "enc_xchacha20_poly1305_custom_cmd"
	param.Description = "Custom command request with xchacha20-poly1305 encryption"
	generateTestVector(t, param, types.MessageBodyTypeCustomCommandReq, buildCustomCmdBody(), "custom_command_req")
}
