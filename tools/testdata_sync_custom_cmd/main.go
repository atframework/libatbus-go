package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	buffer "github.com/atframework/libatbus-go/buffer"
	libatbus_impl "github.com/atframework/libatbus-go/impl"
	"github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
	"google.golang.org/protobuf/proto"
)

type crossLangTestMetadata struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	ProtocolVersion     int    `json:"protocol_version"`
	BodyType            string `json:"body_type"`
	BodyTypeCase        int    `json:"body_type_case"`
	CryptoAlgorithm     string `json:"crypto_algorithm"`
	CryptoAlgorithmType int    `json:"crypto_algorithm_type"`
	KeyHex              string `json:"key_hex"`
	KeySize             int    `json:"key_size"`
	IVHex               string `json:"iv_hex"`
	IVSize              int    `json:"iv_size"`
	AADHex              string `json:"aad_hex"`
	AADSize             int    `json:"aad_size"`
	PackedSize          int    `json:"packed_size"`
	PackedHex           string `json:"packed_hex"`
	Expected            struct {
		From     uint64   `json:"from"`
		Commands []string `json:"commands"`
	} `json:"expected"`
}

func mapCryptoAlgorithmType(cppType int) protocol.ATBUS_CRYPTO_ALGORITHM_TYPE {
	switch cppType {
	case 0:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE
	case 1:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA
	case 11:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC
	case 12:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC
	case 13:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC
	case 14:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM
	case 15:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM
	case 16:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM
	case 31:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20
	case 32:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF
	case 33:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF
	default:
		return protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE
	}
}

func cryptoAlgorithmIsAEAD(alg protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) bool {
	switch alg {
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

func loadMetadata(path string) (*crossLangTestMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var metadata crossLangTestMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func saveMetadata(path string, metadata *crossLangTestMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func buildCustomCmdFrame(metadata *crossLangTestMetadata) ([]byte, error) {
	key, err := hex.DecodeString(metadata.KeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode key_hex: %w", err)
	}
	iv, err := hex.DecodeString(metadata.IVHex)
	if err != nil {
		return nil, fmt.Errorf("decode iv_hex: %w", err)
	}
	var aad []byte
	if metadata.AADHex != "" {
		aad, err = hex.DecodeString(metadata.AADHex)
		if err != nil {
			return nil, fmt.Errorf("decode aad_hex: %w", err)
		}
	}

	cmds := make([]*protocol.CustomCommandArgv, 0, len(metadata.Expected.Commands))
	for _, cmd := range metadata.Expected.Commands {
		cmds = append(cmds, &protocol.CustomCommandArgv{Arg: []byte(cmd)})
	}

	msg := types.NewMessage()
	msg.MutableBody().MessageType = &protocol.MessageBody_CustomCommandReq{
		CustomCommandReq: &protocol.CustomCommandData{
			From:     metadata.Expected.From,
			Commands: cmds,
		},
	}

	bodyBytes, err := proto.Marshal(msg.GetBody())
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	alg := mapCryptoAlgorithmType(metadata.CryptoAlgorithmType)
	head := msg.MutableHead()
	head.Version = int32(metadata.ProtocolVersion)
	head.BodySize = uint64(len(bodyBytes))
	head.Crypto = &protocol.MessageHeadCrypto{
		Algorithm: alg,
		Iv:        iv,
		Aad:       aad,
	}

	session := libatbus_impl.NewCryptoSession()
	if err := session.SetKey(key, iv, alg); err != nil {
		return nil, fmt.Errorf("setup crypto: %w", err)
	}

	var encrypted []byte
	if cryptoAlgorithmIsAEAD(alg) {
		encrypted, err = session.EncryptWithIVAndAAD(bodyBytes, iv, aad)
	} else {
		encrypted, err = session.EncryptWithIV(bodyBytes, iv)
	}
	if err != nil {
		return nil, fmt.Errorf("encrypt body: %w", err)
	}

	headBytes, err := proto.Marshal(head)
	if err != nil {
		return nil, fmt.Errorf("marshal head: %w", err)
	}

	headSize := len(headBytes)
	headVintSize := buffer.VintEncodedSize(uint64(headSize))
	totalSize := headVintSize + headSize + len(encrypted)
	frame := make([]byte, totalSize)
	buffer.WriteVint(uint64(headSize), frame)
	copy(frame[headVintSize:], headBytes)
	copy(frame[headVintSize+headSize:], encrypted)

	metadata.PackedSize = len(frame)
	metadata.PackedHex = hex.EncodeToString(frame)
	metadata.KeySize = len(key)
	metadata.IVSize = len(iv)
	metadata.AADSize = len(aad)

	return frame, nil
}

func main() {
	root := filepath.Join("..", "..", "impl", "testdata")
	customCmdFiles := []string{
		"enc_aes_128_cbc_custom_cmd",
		"enc_aes_192_cbc_custom_cmd",
		"enc_aes_256_cbc_custom_cmd",
		"enc_aes_128_gcm_custom_cmd",
		"enc_aes_192_gcm_custom_cmd",
		"enc_aes_256_gcm_custom_cmd",
		"enc_chacha20_poly1305_custom_cmd",
	}

	for _, name := range customCmdFiles {
		jsonPath := filepath.Join(root, name+".json")
		bytesPath := filepath.Join(root, name+".bytes")

		metadata, err := loadMetadata(jsonPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load %s: %v\n", jsonPath, err)
			os.Exit(1)
		}

		frame, err := buildCustomCmdFrame(metadata)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build %s: %v\n", name, err)
			os.Exit(1)
		}

		if err := os.WriteFile(bytesPath, frame, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", bytesPath, err)
			os.Exit(1)
		}

		if err := saveMetadata(jsonPath, metadata); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", jsonPath, err)
			os.Exit(1)
		}

		fmt.Printf("updated %s\n", name)
	}
}
