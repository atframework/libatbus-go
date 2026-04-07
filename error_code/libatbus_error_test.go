package libatbus_types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLibatbusStrerrorKnownCodes(t *testing.T) {
	// Arrange - test all known error codes
	cases := []struct {
		code ErrorType
		want string
	}{
		// Success
		{EN_ATBUS_ERR_SUCCESS, "EN_ATBUS_ERR_SUCCESS(0): success"},

		// General errors (-1 to -17)
		{EN_ATBUS_ERR_PARAMS, "EN_ATBUS_ERR_PARAMS(-1): ATBUS parameter error"},
		{EN_ATBUS_ERR_INNER, "EN_ATBUS_ERR_INNER(-2): ATBUS inner error"},
		{EN_ATBUS_ERR_NO_DATA, "EN_ATBUS_ERR_NO_DATA(-3): no data"},
		{EN_ATBUS_ERR_BUFF_LIMIT, "EN_ATBUS_ERR_BUFF_LIMIT(-4): buffer limit"},
		{EN_ATBUS_ERR_MALLOC, "EN_ATBUS_ERR_MALLOC(-5): memory allocation failed"},
		{EN_ATBUS_ERR_SCHEME, "EN_ATBUS_ERR_SCHEME(-6): protocol error"},
		{EN_ATBUS_ERR_BAD_DATA, "EN_ATBUS_ERR_BAD_DATA(-7): bad data"},
		{EN_ATBUS_ERR_INVALID_SIZE, "EN_ATBUS_ERR_INVALID_SIZE(-8): invalid size"},
		{EN_ATBUS_ERR_NOT_INITED, "EN_ATBUS_ERR_NOT_INITED(-9): not initialized"},
		{EN_ATBUS_ERR_ALREADY_INITED, "EN_ATBUS_ERR_ALREADY_INITED(-10): already initialized"},
		{EN_ATBUS_ERR_ACCESS_DENY, "EN_ATBUS_ERR_ACCESS_DENY(-11): access denied"},
		{EN_ATBUS_ERR_UNPACK, "EN_ATBUS_ERR_UNPACK(-12): unpack failed"},
		{EN_ATBUS_ERR_PACK, "EN_ATBUS_ERR_PACK(-13): pack failed"},
		{EN_ATBUS_ERR_UNSUPPORTED_VERSION, "EN_ATBUS_ERR_UNSUPPORTED_VERSION(-14): unsupported version"},
		{EN_ATBUS_ERR_CLOSING, "EN_ATBUS_ERR_CLOSING(-15): closing"},
		{EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT, "EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT(-16): algorithm not supported"},
		{EN_ATBUS_ERR_MESSAGE_NOT_FINISH_YET, "EN_ATBUS_ERR_MESSAGE_NOT_FINISH_YET(-17): message not finished yet"},

		// Node errors (-65 to -75)
		{EN_ATBUS_ERR_ATNODE_NOT_FOUND, "EN_ATBUS_ERR_ATNODE_NOT_FOUND(-65): target node not found"},
		{EN_ATBUS_ERR_ATNODE_INVALID_ID, "EN_ATBUS_ERR_ATNODE_INVALID_ID(-66): invalid node id"},
		{EN_ATBUS_ERR_ATNODE_NO_CONNECTION, "EN_ATBUS_ERR_ATNODE_NO_CONNECTION(-67): no connection"},
		{EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT, "EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT(-68): exceeded fault tolerant"},
		{EN_ATBUS_ERR_ATNODE_INVALID_MSG, "EN_ATBUS_ERR_ATNODE_INVALID_MSG(-69): invalid message"},
		{EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH, "EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH(-70): bus id not match"},
		{EN_ATBUS_ERR_ATNODE_TTL, "EN_ATBUS_ERR_ATNODE_TTL(-71): ttl limited"},
		{EN_ATBUS_ERR_ATNODE_MASK_CONFLICT, "EN_ATBUS_ERR_ATNODE_MASK_CONFLICT(-72): mask conflict"},
		{EN_ATBUS_ERR_ATNODE_ID_CONFLICT, "EN_ATBUS_ERR_ATNODE_ID_CONFLICT(-73): id conflict"},
		{EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME, "EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME(-75): source and destination are the same"},

		// Channel errors (-101 to -108)
		{EN_ATBUS_ERR_CHANNEL_SIZE_TOO_SMALL, "EN_ATBUS_ERR_CHANNEL_SIZE_TOO_SMALL(-101): channel size too small"},
		{EN_ATBUS_ERR_CHANNEL_BUFFER_INVALID, "EN_ATBUS_ERR_CHANNEL_BUFFER_INVALID(-102): channel buffer invalid"},
		{EN_ATBUS_ERR_CHANNEL_ADDR_INVALID, "EN_ATBUS_ERR_CHANNEL_ADDR_INVALID(-103): channel address invalid"},
		{EN_ATBUS_ERR_CHANNEL_CLOSING, "EN_ATBUS_ERR_CHANNEL_CLOSING(-104): channel closing"},
		{EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT, "EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT(-105): channel not supported"},
		{EN_ATBUS_ERR_CHANNEL_UNSUPPORTED_VERSION, "EN_ATBUS_ERR_CHANNEL_UNSUPPORTED_VERSION(-106): channel unsupported version"},
		{EN_ATBUS_ERR_CHANNEL_ALIGN_SIZE_MISMATCH, "EN_ATBUS_ERR_CHANNEL_ALIGN_SIZE_MISMATCH(-107): channel align size mismatch"},
		{EN_ATBUS_ERR_CHANNEL_ARCH_SIZE_T_MISMATCH, "EN_ATBUS_ERR_CHANNEL_ARCH_SIZE_T_MISMATCH(-108): channel architecture size_t mismatch"},

		// Node block errors (-202 to -211)
		{EN_ATBUS_ERR_NODE_BAD_BLOCK_NODE_NUM, "EN_ATBUS_ERR_NODE_BAD_BLOCK_NODE_NUM(-202): corrupted node block - node count error"},
		{EN_ATBUS_ERR_NODE_BAD_BLOCK_BUFF_SIZE, "EN_ATBUS_ERR_NODE_BAD_BLOCK_BUFF_SIZE(-203): corrupted node block - buffer size error"},
		{EN_ATBUS_ERR_NODE_BAD_BLOCK_WSEQ_ID, "EN_ATBUS_ERR_NODE_BAD_BLOCK_WSEQ_ID(-204): corrupted node block - write sequence error"},
		{EN_ATBUS_ERR_NODE_BAD_BLOCK_CSEQ_ID, "EN_ATBUS_ERR_NODE_BAD_BLOCK_CSEQ_ID(-205): corrupted node block - check sequence error"},
		{EN_ATBUS_ERR_NODE_TIMEOUT, "EN_ATBUS_ERR_NODE_TIMEOUT(-211): operation timeout"},

		// Crypto errors (-231 to -242)
		{EN_ATBUS_ERR_CRYPTO_DECRYPT, "EN_ATBUS_ERR_CRYPTO_DECRYPT(-231): decryption failed"},
		{EN_ATBUS_ERR_CRYPTO_ENCRYPT, "EN_ATBUS_ERR_CRYPTO_ENCRYPT(-232): encryption failed"},
		{EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_SUPPORT, "EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_SUPPORT(-233): crypto algorithm not supported"},
		{EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_MATCH, "EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_MATCH(-234): crypto algorithm not match"},
		{EN_ATBUS_ERR_CRYPTO_INVALID_IV, "EN_ATBUS_ERR_CRYPTO_INVALID_IV(-235): invalid crypto iv/nonce"},
		{EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR(-236): crypto handshake make key pair failed"},
		{EN_ATBUS_ERR_CRYPTO_HANDSHAKE_READ_PEER_KEY, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_READ_PEER_KEY(-237): crypto handshake read peer key failed"},
		{EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET(-238): crypto handshake make secret failed"},
		{EN_ATBUS_ERR_CRYPTO_HANDSHAKE_SEQUENCE_EXPIRED, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_SEQUENCE_EXPIRED(-239): crypto handshake sequence expired"},
		{EN_ATBUS_ERR_CRYPTO_HANDSHAKE_NO_AVAILABLE_ALGORITHM, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_NO_AVAILABLE_ALGORITHM(-240): crypto handshake no available algorithm"},
		{EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_ERROR, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_ERROR(-241): crypto handshake kdf error"},
		{EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_NOT_SUPPORT, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_NOT_SUPPORT(-242): crypto handshake kdf not support"},

		// Compression errors (-251)
		{EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT, "EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT(-251): compression algorithm not supported"},

		// Shared memory errors (-301 to -305)
		{EN_ATBUS_ERR_SHM_GET_FAILED, "EN_ATBUS_ERR_SHM_GET_FAILED(-301): shared memory get failed"},
		{EN_ATBUS_ERR_SHM_NOT_FOUND, "EN_ATBUS_ERR_SHM_NOT_FOUND(-302): shared memory not found"},
		{EN_ATBUS_ERR_SHM_CLOSE_FAILED, "EN_ATBUS_ERR_SHM_CLOSE_FAILED(-303): shared memory close failed"},
		{EN_ATBUS_ERR_SHM_PATH_INVALID, "EN_ATBUS_ERR_SHM_PATH_INVALID(-304): shared memory path invalid"},
		{EN_ATBUS_ERR_SHM_MAP_FAILED, "EN_ATBUS_ERR_SHM_MAP_FAILED(-305): shared memory map failed"},

		// Socket errors (-401 to -403)
		{EN_ATBUS_ERR_SOCK_BIND_FAILED, "EN_ATBUS_ERR_SOCK_BIND_FAILED(-401): socket bind failed"},
		{EN_ATBUS_ERR_SOCK_LISTEN_FAILED, "EN_ATBUS_ERR_SOCK_LISTEN_FAILED(-402): socket listen failed"},
		{EN_ATBUS_ERR_SOCK_CONNECT_FAILED, "EN_ATBUS_ERR_SOCK_CONNECT_FAILED(-403): socket connect failed"},

		// Pipe errors (-501 to -507)
		{EN_ATBUS_ERR_PIPE_BIND_FAILED, "EN_ATBUS_ERR_PIPE_BIND_FAILED(-501): pipe bind failed"},
		{EN_ATBUS_ERR_PIPE_LISTEN_FAILED, "EN_ATBUS_ERR_PIPE_LISTEN_FAILED(-502): pipe listen failed"},
		{EN_ATBUS_ERR_PIPE_CONNECT_FAILED, "EN_ATBUS_ERR_PIPE_CONNECT_FAILED(-503): pipe connect failed"},
		{EN_ATBUS_ERR_PIPE_ADDR_TOO_LONG, "EN_ATBUS_ERR_PIPE_ADDR_TOO_LONG(-504): pipe address too long"},
		{EN_ATBUS_ERR_PIPE_REMOVE_FAILED, "EN_ATBUS_ERR_PIPE_REMOVE_FAILED(-505): pipe remove old socket failed"},
		{EN_ATBUS_ERR_PIPE_PATH_EXISTS, "EN_ATBUS_ERR_PIPE_PATH_EXISTS(-506): pipe path already exists"},
		{EN_ATBUS_ERR_PIPE_LOCK_PATH_FAILED, "EN_ATBUS_ERR_PIPE_LOCK_PATH_FAILED(-507): pipe lock path failed"},

		// Network/IO errors (-601 to -607)
		{EN_ATBUS_ERR_DNS_GETADDR_FAILED, "EN_ATBUS_ERR_DNS_GETADDR_FAILED(-601): dns getaddr failed"},
		{EN_ATBUS_ERR_CONNECTION_NOT_FOUND, "EN_ATBUS_ERR_CONNECTION_NOT_FOUND(-602): connection not found"},
		{EN_ATBUS_ERR_WRITE_FAILED, "EN_ATBUS_ERR_WRITE_FAILED(-603): write failed"},
		{EN_ATBUS_ERR_READ_FAILED, "EN_ATBUS_ERR_READ_FAILED(-604): read failed"},
		{EN_ATBUS_ERR_EV_RUN, "EN_ATBUS_ERR_EV_RUN(-605): event loop run failed"},
		{EN_ATBUS_ERR_NO_LISTEN, "EN_ATBUS_ERR_NO_LISTEN(-606): no listen"},
		{EN_ATBUS_ERR_NOT_READY, "EN_ATBUS_ERR_NOT_READY(-607): not ready"},
	}

	// Act + Assert
	for _, tc := range cases {
		assert.Equal(t, tc.want, LibatbusStrerror(tc.code))
		assert.Equal(t, tc.want, tc.code.String())
		assert.Equal(t, tc.want, tc.code.Error())
	}
}

func TestLibatbusStrerrorUnknownCode(t *testing.T) {
	// Arrange
	unknown := ErrorType(-123456)

	// Act
	got := LibatbusStrerror(unknown)

	// Assert
	assert.Equal(t, "ATBUS_ERROR_TYPE(-123456): unknown", got)
}
