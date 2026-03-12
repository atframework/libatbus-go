package libatbus_types

import "fmt"

// ErrorType is the Go equivalent of C++ enum ATBUS_ERROR_TYPE.
//
// Note: values are kept identical to the C++ header to ensure wire/log compatibility.
type ErrorType int32

const (
	EN_ATBUS_ERR_SUCCESS ErrorType = 0

	EN_ATBUS_ERR_PARAMS                 ErrorType = -1
	EN_ATBUS_ERR_INNER                  ErrorType = -2
	EN_ATBUS_ERR_NO_DATA                ErrorType = -3  // 无数据
	EN_ATBUS_ERR_BUFF_LIMIT             ErrorType = -4  // 缓冲区不足
	EN_ATBUS_ERR_MALLOC                 ErrorType = -5  // 分配失败
	EN_ATBUS_ERR_SCHEME                 ErrorType = -6  // 协议错误
	EN_ATBUS_ERR_BAD_DATA               ErrorType = -7  // 数据校验不通过
	EN_ATBUS_ERR_INVALID_SIZE           ErrorType = -8  // 数据大小异常
	EN_ATBUS_ERR_NOT_INITED             ErrorType = -9  // 未初始化
	EN_ATBUS_ERR_ALREADY_INITED         ErrorType = -10 // 已填充初始数据
	EN_ATBUS_ERR_ACCESS_DENY            ErrorType = -11 // 不允许的操作
	EN_ATBUS_ERR_UNPACK                 ErrorType = -12 // 解包失败
	EN_ATBUS_ERR_PACK                   ErrorType = -13 // 打包失败
	EN_ATBUS_ERR_UNSUPPORTED_VERSION    ErrorType = -14 // 版本不受支持
	EN_ATBUS_ERR_CLOSING                ErrorType = -15 // 正在关闭或已关闭
	EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT  ErrorType = -16 // 算法不受支持
	EN_ATBUS_ERR_MESSAGE_NOT_FINISH_YET ErrorType = -17 // 消息尚未调用finish

	EN_ATBUS_ERR_ATNODE_NOT_FOUND        ErrorType = -65 // 查找不到目标节点
	EN_ATBUS_ERR_ATNODE_INVALID_ID       ErrorType = -66 // 不可用的ID
	EN_ATBUS_ERR_ATNODE_NO_CONNECTION    ErrorType = -67 // 无可用连接
	EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT   ErrorType = -68 // 超出容错值
	EN_ATBUS_ERR_ATNODE_INVALID_MSG      ErrorType = -69 // 错误的消息
	EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH ErrorType = -70 // Bus ID不匹配
	EN_ATBUS_ERR_ATNODE_TTL              ErrorType = -71 // ttl限制
	EN_ATBUS_ERR_ATNODE_MASK_CONFLICT    ErrorType = -72 // 域范围错误或冲突
	EN_ATBUS_ERR_ATNODE_ID_CONFLICT      ErrorType = -73 // ID冲突
	EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME  ErrorType = -75 // 发送源和发送目标不能相同

	EN_ATBUS_ERR_CHANNEL_SIZE_TOO_SMALL       ErrorType = -101
	EN_ATBUS_ERR_CHANNEL_BUFFER_INVALID       ErrorType = -102 // 缓冲区错误（已被其他模块使用或检测冲突）
	EN_ATBUS_ERR_CHANNEL_ADDR_INVALID         ErrorType = -103 // 地址错误
	EN_ATBUS_ERR_CHANNEL_CLOSING              ErrorType = -104 // 正在关闭
	EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT          ErrorType = -105 // 不支持的通道
	EN_ATBUS_ERR_CHANNEL_UNSUPPORTED_VERSION  ErrorType = -106 // 通道版本不受支持
	EN_ATBUS_ERR_CHANNEL_ALIGN_SIZE_MISMATCH  ErrorType = -107 // 对齐参数不一致
	EN_ATBUS_ERR_CHANNEL_ARCH_SIZE_T_MISMATCH ErrorType = -108 // 架构size_t不匹配

	EN_ATBUS_ERR_NODE_BAD_BLOCK_NODE_NUM  ErrorType = -202 // 发现写坏的数据块 - 节点数量错误
	EN_ATBUS_ERR_NODE_BAD_BLOCK_BUFF_SIZE ErrorType = -203 // 发现写坏的数据块 - 节点数量错误
	EN_ATBUS_ERR_NODE_BAD_BLOCK_WSEQ_ID   ErrorType = -204 // 发现写坏的数据块 - 写操作序列错误
	EN_ATBUS_ERR_NODE_BAD_BLOCK_CSEQ_ID   ErrorType = -205 // 发现写坏的数据块 - 检查操作序列错误

	EN_ATBUS_ERR_NODE_TIMEOUT ErrorType = -211 // 操作超时

	EN_ATBUS_ERR_CRYPTO_DECRYPT                          ErrorType = -231 // 解密失败
	EN_ATBUS_ERR_CRYPTO_ENCRYPT                          ErrorType = -232 // 加密失败
	EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_SUPPORT            ErrorType = -233 // 加密算法不支持
	EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_MATCH              ErrorType = -234 // 加密算法不匹配
	EN_ATBUS_ERR_CRYPTO_INVALID_IV                       ErrorType = -235 // 不合法的IV/nonce
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR          ErrorType = -236 // 加密握手生成密钥对失败
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_READ_PEER_KEY          ErrorType = -237 // 加密握手读取对方公钥失败
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET            ErrorType = -238 // 加密握手生成密钥失败
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_SEQUENCE_EXPIRED       ErrorType = -239 // 加密握手序列过期
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_NO_AVAILABLE_ALGORITHM ErrorType = -240 // 加密握手没有可用的加密算法
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_ERROR              ErrorType = -241 // 加密握手KDF错误
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_NOT_SUPPORT        ErrorType = -242 // 加密握手KDF不支持

	EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT ErrorType = -251 // 压缩算法不支持

	EN_ATBUS_ERR_SHM_GET_FAILED   ErrorType = -301 // 连接共享内存出错
	EN_ATBUS_ERR_SHM_NOT_FOUND    ErrorType = -302 // 共享内存未找到
	EN_ATBUS_ERR_SHM_CLOSE_FAILED ErrorType = -303 // 移除共享内存出错
	EN_ATBUS_ERR_SHM_PATH_INVALID ErrorType = -304 // 共享内存地址错误
	EN_ATBUS_ERR_SHM_MAP_FAILED   ErrorType = -305 // 共享内存地址映射错误

	EN_ATBUS_ERR_SOCK_BIND_FAILED    ErrorType = -401 // 绑定地址或端口失败
	EN_ATBUS_ERR_SOCK_LISTEN_FAILED  ErrorType = -402 // 监听失败
	EN_ATBUS_ERR_SOCK_CONNECT_FAILED ErrorType = -403 // 连接失败

	EN_ATBUS_ERR_PIPE_BIND_FAILED      ErrorType = -501 // 绑定地址或端口失败
	EN_ATBUS_ERR_PIPE_LISTEN_FAILED    ErrorType = -502 // 监听失败
	EN_ATBUS_ERR_PIPE_CONNECT_FAILED   ErrorType = -503 // 连接失败
	EN_ATBUS_ERR_PIPE_ADDR_TOO_LONG    ErrorType = -504 // 地址路径过长
	EN_ATBUS_ERR_PIPE_REMOVE_FAILED    ErrorType = -505 // 删除老socket失败
	EN_ATBUS_ERR_PIPE_PATH_EXISTS      ErrorType = -506 // 该地址已被占用
	EN_ATBUS_ERR_PIPE_LOCK_PATH_FAILED ErrorType = -507 // 锁地址失败

	EN_ATBUS_ERR_DNS_GETADDR_FAILED   ErrorType = -601 // DNS解析失败
	EN_ATBUS_ERR_CONNECTION_NOT_FOUND ErrorType = -602 // 找不到连接
	EN_ATBUS_ERR_WRITE_FAILED         ErrorType = -603 // 底层API写失败
	EN_ATBUS_ERR_READ_FAILED          ErrorType = -604 // 底层API读失败
	EN_ATBUS_ERR_EV_RUN               ErrorType = -605 // 底层API事件循环失败
	EN_ATBUS_ERR_NO_LISTEN            ErrorType = -606 // 尚未监听（绑定）
	EN_ATBUS_ERR_NOT_READY            ErrorType = -607 // 未准备好（没有握手完成）

	EN_ATBUS_ERR_MIN ErrorType = -999
)

// String returns the same content as LibatbusStrerror.
func (e ErrorType) String() string {
	return libatbusStrerror(e)
}

// Error implements the built-in error interface, so ErrorType can be returned as an error.
func (e ErrorType) Error() string {
	return libatbusStrerror(e)
}

// LibatbusStrerror is the Go equivalent of C++ libatbus_strerror.
//
// It returns:
//   - For known codes: "<ENUM_NAME>(<code>): <message>"
//   - For unknown codes: "ATBUS_ERROR_TYPE(<code>): unknown"
//
// Go naming note: we keep the C++ symbol name in a Go-friendly exported form.
func LibatbusStrerror(errcode ErrorType) string {
	return libatbusStrerror(errcode)
}

func libatbusBuildErrorString(code ErrorType, name, message string) string {
	return fmt.Sprintf("%s(%d): %s", name, int32(code), message)
}

var atbusKnownErrorStringByCode = map[ErrorType]string{
	EN_ATBUS_ERR_SUCCESS: libatbusBuildErrorString(EN_ATBUS_ERR_SUCCESS, "EN_ATBUS_ERR_SUCCESS", "success"),

	EN_ATBUS_ERR_PARAMS:                 libatbusBuildErrorString(EN_ATBUS_ERR_PARAMS, "EN_ATBUS_ERR_PARAMS", "ATBUS parameter error"),
	EN_ATBUS_ERR_INNER:                  libatbusBuildErrorString(EN_ATBUS_ERR_INNER, "EN_ATBUS_ERR_INNER", "ATBUS inner error"),
	EN_ATBUS_ERR_NO_DATA:                libatbusBuildErrorString(EN_ATBUS_ERR_NO_DATA, "EN_ATBUS_ERR_NO_DATA", "no data"),
	EN_ATBUS_ERR_BUFF_LIMIT:             libatbusBuildErrorString(EN_ATBUS_ERR_BUFF_LIMIT, "EN_ATBUS_ERR_BUFF_LIMIT", "buffer limit"),
	EN_ATBUS_ERR_MALLOC:                 libatbusBuildErrorString(EN_ATBUS_ERR_MALLOC, "EN_ATBUS_ERR_MALLOC", "memory allocation failed"),
	EN_ATBUS_ERR_SCHEME:                 libatbusBuildErrorString(EN_ATBUS_ERR_SCHEME, "EN_ATBUS_ERR_SCHEME", "protocol error"),
	EN_ATBUS_ERR_BAD_DATA:               libatbusBuildErrorString(EN_ATBUS_ERR_BAD_DATA, "EN_ATBUS_ERR_BAD_DATA", "bad data"),
	EN_ATBUS_ERR_INVALID_SIZE:           libatbusBuildErrorString(EN_ATBUS_ERR_INVALID_SIZE, "EN_ATBUS_ERR_INVALID_SIZE", "invalid size"),
	EN_ATBUS_ERR_NOT_INITED:             libatbusBuildErrorString(EN_ATBUS_ERR_NOT_INITED, "EN_ATBUS_ERR_NOT_INITED", "not initialized"),
	EN_ATBUS_ERR_ALREADY_INITED:         libatbusBuildErrorString(EN_ATBUS_ERR_ALREADY_INITED, "EN_ATBUS_ERR_ALREADY_INITED", "already initialized"),
	EN_ATBUS_ERR_ACCESS_DENY:            libatbusBuildErrorString(EN_ATBUS_ERR_ACCESS_DENY, "EN_ATBUS_ERR_ACCESS_DENY", "access denied"),
	EN_ATBUS_ERR_UNPACK:                 libatbusBuildErrorString(EN_ATBUS_ERR_UNPACK, "EN_ATBUS_ERR_UNPACK", "unpack failed"),
	EN_ATBUS_ERR_PACK:                   libatbusBuildErrorString(EN_ATBUS_ERR_PACK, "EN_ATBUS_ERR_PACK", "pack failed"),
	EN_ATBUS_ERR_UNSUPPORTED_VERSION:    libatbusBuildErrorString(EN_ATBUS_ERR_UNSUPPORTED_VERSION, "EN_ATBUS_ERR_UNSUPPORTED_VERSION", "unsupported version"),
	EN_ATBUS_ERR_CLOSING:                libatbusBuildErrorString(EN_ATBUS_ERR_CLOSING, "EN_ATBUS_ERR_CLOSING", "closing"),
	EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT:  libatbusBuildErrorString(EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT, "EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT", "algorithm not supported"),
	EN_ATBUS_ERR_MESSAGE_NOT_FINISH_YET: libatbusBuildErrorString(EN_ATBUS_ERR_MESSAGE_NOT_FINISH_YET, "EN_ATBUS_ERR_MESSAGE_NOT_FINISH_YET", "message not finished yet"),

	EN_ATBUS_ERR_ATNODE_NOT_FOUND:        libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_NOT_FOUND, "EN_ATBUS_ERR_ATNODE_NOT_FOUND", "target node not found"),
	EN_ATBUS_ERR_ATNODE_INVALID_ID:       libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_INVALID_ID, "EN_ATBUS_ERR_ATNODE_INVALID_ID", "invalid node id"),
	EN_ATBUS_ERR_ATNODE_NO_CONNECTION:    libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_NO_CONNECTION, "EN_ATBUS_ERR_ATNODE_NO_CONNECTION", "no connection"),
	EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT:   libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT, "EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT", "exceeded fault tolerant"),
	EN_ATBUS_ERR_ATNODE_INVALID_MSG:      libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_INVALID_MSG, "EN_ATBUS_ERR_ATNODE_INVALID_MSG", "invalid message"),
	EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH: libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH, "EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH", "bus id not match"),
	EN_ATBUS_ERR_ATNODE_TTL:              libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_TTL, "EN_ATBUS_ERR_ATNODE_TTL", "ttl limited"),
	EN_ATBUS_ERR_ATNODE_MASK_CONFLICT:    libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_MASK_CONFLICT, "EN_ATBUS_ERR_ATNODE_MASK_CONFLICT", "mask conflict"),
	EN_ATBUS_ERR_ATNODE_ID_CONFLICT:      libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_ID_CONFLICT, "EN_ATBUS_ERR_ATNODE_ID_CONFLICT", "id conflict"),
	EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME:  libatbusBuildErrorString(EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME, "EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME", "source and destination are the same"),

	EN_ATBUS_ERR_CHANNEL_SIZE_TOO_SMALL:       libatbusBuildErrorString(EN_ATBUS_ERR_CHANNEL_SIZE_TOO_SMALL, "EN_ATBUS_ERR_CHANNEL_SIZE_TOO_SMALL", "channel size too small"),
	EN_ATBUS_ERR_CHANNEL_BUFFER_INVALID:       libatbusBuildErrorString(EN_ATBUS_ERR_CHANNEL_BUFFER_INVALID, "EN_ATBUS_ERR_CHANNEL_BUFFER_INVALID", "channel buffer invalid"),
	EN_ATBUS_ERR_CHANNEL_ADDR_INVALID:         libatbusBuildErrorString(EN_ATBUS_ERR_CHANNEL_ADDR_INVALID, "EN_ATBUS_ERR_CHANNEL_ADDR_INVALID", "channel address invalid"),
	EN_ATBUS_ERR_CHANNEL_CLOSING:              libatbusBuildErrorString(EN_ATBUS_ERR_CHANNEL_CLOSING, "EN_ATBUS_ERR_CHANNEL_CLOSING", "channel closing"),
	EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT:          libatbusBuildErrorString(EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT, "EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT", "channel not supported"),
	EN_ATBUS_ERR_CHANNEL_UNSUPPORTED_VERSION:  libatbusBuildErrorString(EN_ATBUS_ERR_CHANNEL_UNSUPPORTED_VERSION, "EN_ATBUS_ERR_CHANNEL_UNSUPPORTED_VERSION", "channel unsupported version"),
	EN_ATBUS_ERR_CHANNEL_ALIGN_SIZE_MISMATCH:  libatbusBuildErrorString(EN_ATBUS_ERR_CHANNEL_ALIGN_SIZE_MISMATCH, "EN_ATBUS_ERR_CHANNEL_ALIGN_SIZE_MISMATCH", "channel align size mismatch"),
	EN_ATBUS_ERR_CHANNEL_ARCH_SIZE_T_MISMATCH: libatbusBuildErrorString(EN_ATBUS_ERR_CHANNEL_ARCH_SIZE_T_MISMATCH, "EN_ATBUS_ERR_CHANNEL_ARCH_SIZE_T_MISMATCH", "channel architecture size_t mismatch"),

	EN_ATBUS_ERR_NODE_BAD_BLOCK_NODE_NUM:  libatbusBuildErrorString(EN_ATBUS_ERR_NODE_BAD_BLOCK_NODE_NUM, "EN_ATBUS_ERR_NODE_BAD_BLOCK_NODE_NUM", "corrupted node block - node count error"),
	EN_ATBUS_ERR_NODE_BAD_BLOCK_BUFF_SIZE: libatbusBuildErrorString(EN_ATBUS_ERR_NODE_BAD_BLOCK_BUFF_SIZE, "EN_ATBUS_ERR_NODE_BAD_BLOCK_BUFF_SIZE", "corrupted node block - buffer size error"),
	EN_ATBUS_ERR_NODE_BAD_BLOCK_WSEQ_ID:   libatbusBuildErrorString(EN_ATBUS_ERR_NODE_BAD_BLOCK_WSEQ_ID, "EN_ATBUS_ERR_NODE_BAD_BLOCK_WSEQ_ID", "corrupted node block - write sequence error"),
	EN_ATBUS_ERR_NODE_BAD_BLOCK_CSEQ_ID:   libatbusBuildErrorString(EN_ATBUS_ERR_NODE_BAD_BLOCK_CSEQ_ID, "EN_ATBUS_ERR_NODE_BAD_BLOCK_CSEQ_ID", "corrupted node block - check sequence error"),
	EN_ATBUS_ERR_NODE_TIMEOUT:             libatbusBuildErrorString(EN_ATBUS_ERR_NODE_TIMEOUT, "EN_ATBUS_ERR_NODE_TIMEOUT", "operation timeout"),

	EN_ATBUS_ERR_CRYPTO_DECRYPT:                          libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_DECRYPT, "EN_ATBUS_ERR_CRYPTO_DECRYPT", "decryption failed"),
	EN_ATBUS_ERR_CRYPTO_ENCRYPT:                          libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_ENCRYPT, "EN_ATBUS_ERR_CRYPTO_ENCRYPT", "encryption failed"),
	EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_SUPPORT:            libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_SUPPORT, "EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_SUPPORT", "crypto algorithm not supported"),
	EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_MATCH:              libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_MATCH, "EN_ATBUS_ERR_CRYPTO_ALGORITHM_NOT_MATCH", "crypto algorithm not match"),
	EN_ATBUS_ERR_CRYPTO_INVALID_IV:                       libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_INVALID_IV, "EN_ATBUS_ERR_CRYPTO_INVALID_IV", "invalid crypto iv/nonce"),
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR:          libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_KEY_PAIR", "crypto handshake make key pair failed"),
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_READ_PEER_KEY:          libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_HANDSHAKE_READ_PEER_KEY, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_READ_PEER_KEY", "crypto handshake read peer key failed"),
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET:            libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_MAKE_SECRET", "crypto handshake make secret failed"),
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_SEQUENCE_EXPIRED:       libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_HANDSHAKE_SEQUENCE_EXPIRED, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_SEQUENCE_EXPIRED", "crypto handshake sequence expired"),
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_NO_AVAILABLE_ALGORITHM: libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_HANDSHAKE_NO_AVAILABLE_ALGORITHM, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_NO_AVAILABLE_ALGORITHM", "crypto handshake no available algorithm"),
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_ERROR:              libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_ERROR, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_ERROR", "crypto handshake kdf error"),
	EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_NOT_SUPPORT:        libatbusBuildErrorString(EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_NOT_SUPPORT, "EN_ATBUS_ERR_CRYPTO_HANDSHAKE_KDF_NOT_SUPPORT", "crypto handshake kdf not support"),

	EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT: libatbusBuildErrorString(EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT, "EN_ATBUS_ERR_COMPRESSION_ALGORITHM_NOT_SUPPORT", "compression algorithm not supported"),

	EN_ATBUS_ERR_SHM_GET_FAILED:   libatbusBuildErrorString(EN_ATBUS_ERR_SHM_GET_FAILED, "EN_ATBUS_ERR_SHM_GET_FAILED", "shared memory get failed"),
	EN_ATBUS_ERR_SHM_NOT_FOUND:    libatbusBuildErrorString(EN_ATBUS_ERR_SHM_NOT_FOUND, "EN_ATBUS_ERR_SHM_NOT_FOUND", "shared memory not found"),
	EN_ATBUS_ERR_SHM_CLOSE_FAILED: libatbusBuildErrorString(EN_ATBUS_ERR_SHM_CLOSE_FAILED, "EN_ATBUS_ERR_SHM_CLOSE_FAILED", "shared memory close failed"),
	EN_ATBUS_ERR_SHM_PATH_INVALID: libatbusBuildErrorString(EN_ATBUS_ERR_SHM_PATH_INVALID, "EN_ATBUS_ERR_SHM_PATH_INVALID", "shared memory path invalid"),
	EN_ATBUS_ERR_SHM_MAP_FAILED:   libatbusBuildErrorString(EN_ATBUS_ERR_SHM_MAP_FAILED, "EN_ATBUS_ERR_SHM_MAP_FAILED", "shared memory map failed"),

	EN_ATBUS_ERR_SOCK_BIND_FAILED:    libatbusBuildErrorString(EN_ATBUS_ERR_SOCK_BIND_FAILED, "EN_ATBUS_ERR_SOCK_BIND_FAILED", "socket bind failed"),
	EN_ATBUS_ERR_SOCK_LISTEN_FAILED:  libatbusBuildErrorString(EN_ATBUS_ERR_SOCK_LISTEN_FAILED, "EN_ATBUS_ERR_SOCK_LISTEN_FAILED", "socket listen failed"),
	EN_ATBUS_ERR_SOCK_CONNECT_FAILED: libatbusBuildErrorString(EN_ATBUS_ERR_SOCK_CONNECT_FAILED, "EN_ATBUS_ERR_SOCK_CONNECT_FAILED", "socket connect failed"),

	EN_ATBUS_ERR_PIPE_BIND_FAILED:      libatbusBuildErrorString(EN_ATBUS_ERR_PIPE_BIND_FAILED, "EN_ATBUS_ERR_PIPE_BIND_FAILED", "pipe bind failed"),
	EN_ATBUS_ERR_PIPE_LISTEN_FAILED:    libatbusBuildErrorString(EN_ATBUS_ERR_PIPE_LISTEN_FAILED, "EN_ATBUS_ERR_PIPE_LISTEN_FAILED", "pipe listen failed"),
	EN_ATBUS_ERR_PIPE_CONNECT_FAILED:   libatbusBuildErrorString(EN_ATBUS_ERR_PIPE_CONNECT_FAILED, "EN_ATBUS_ERR_PIPE_CONNECT_FAILED", "pipe connect failed"),
	EN_ATBUS_ERR_PIPE_ADDR_TOO_LONG:    libatbusBuildErrorString(EN_ATBUS_ERR_PIPE_ADDR_TOO_LONG, "EN_ATBUS_ERR_PIPE_ADDR_TOO_LONG", "pipe address too long"),
	EN_ATBUS_ERR_PIPE_REMOVE_FAILED:    libatbusBuildErrorString(EN_ATBUS_ERR_PIPE_REMOVE_FAILED, "EN_ATBUS_ERR_PIPE_REMOVE_FAILED", "pipe remove old socket failed"),
	EN_ATBUS_ERR_PIPE_PATH_EXISTS:      libatbusBuildErrorString(EN_ATBUS_ERR_PIPE_PATH_EXISTS, "EN_ATBUS_ERR_PIPE_PATH_EXISTS", "pipe path already exists"),
	EN_ATBUS_ERR_PIPE_LOCK_PATH_FAILED: libatbusBuildErrorString(EN_ATBUS_ERR_PIPE_LOCK_PATH_FAILED, "EN_ATBUS_ERR_PIPE_LOCK_PATH_FAILED", "pipe lock path failed"),

	EN_ATBUS_ERR_DNS_GETADDR_FAILED:   libatbusBuildErrorString(EN_ATBUS_ERR_DNS_GETADDR_FAILED, "EN_ATBUS_ERR_DNS_GETADDR_FAILED", "dns getaddr failed"),
	EN_ATBUS_ERR_CONNECTION_NOT_FOUND: libatbusBuildErrorString(EN_ATBUS_ERR_CONNECTION_NOT_FOUND, "EN_ATBUS_ERR_CONNECTION_NOT_FOUND", "connection not found"),
	EN_ATBUS_ERR_WRITE_FAILED:         libatbusBuildErrorString(EN_ATBUS_ERR_WRITE_FAILED, "EN_ATBUS_ERR_WRITE_FAILED", "write failed"),
	EN_ATBUS_ERR_READ_FAILED:          libatbusBuildErrorString(EN_ATBUS_ERR_READ_FAILED, "EN_ATBUS_ERR_READ_FAILED", "read failed"),
	EN_ATBUS_ERR_EV_RUN:               libatbusBuildErrorString(EN_ATBUS_ERR_EV_RUN, "EN_ATBUS_ERR_EV_RUN", "event loop run failed"),
	EN_ATBUS_ERR_NO_LISTEN:            libatbusBuildErrorString(EN_ATBUS_ERR_NO_LISTEN, "EN_ATBUS_ERR_NO_LISTEN", "no listen"),
	EN_ATBUS_ERR_NOT_READY:            libatbusBuildErrorString(EN_ATBUS_ERR_NOT_READY, "EN_ATBUS_ERR_NOT_READY", "not ready"),
}

func libatbusStrerror(errcode ErrorType) string {
	if s, ok := atbusKnownErrorStringByCode[errcode]; ok {
		return s
	}
	return libatbusBuildErrorString(errcode, "ATBUS_ERROR_TYPE", "unknown")
}
