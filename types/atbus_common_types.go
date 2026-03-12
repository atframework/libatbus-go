package libatbus_types

import (
	"context"
	"time"

	buffer "github.com/atframework/libatbus-go/buffer"
	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
)

type ErrorType = error_code.ErrorType

type BusIdType uint64

const (
	ATBUS_MACRO_MAX_FRAME_HEADER uint64 = 4096 // 最大数据包头长度
)

type NodeConfigure struct {
	EventLoopContext context.Context

	UpstreamAddress        string            // 上游节点地址
	TopologyLabels         map[string]string // 拓扑标签
	LoopTimes              int32             // 消息循环次数限制，防止某些通道繁忙把其他通道堵死
	TTL                    int32             // 消息转发跳转限制
	ProtocolVersion        int32
	ProtocolMinimalVersion int32

	// ===== 连接配置 =====
	BackLog              int32
	FirstIdleTimeout     time.Duration // 第一个包允许的空闲时间
	PingInterval         time.Duration // ping包间隔
	RetryInterval        time.Duration // 重试包间隔
	FaultTolerant        uint32        // 容错次数（次）
	AccessTokenMaxNumber uint32        // 最大 access token 数量，请不要设置的太大，验证次数最大可能是 N^2
	AccessTokens         [][]byte      // access token 列表
	OverwriteListenPath  bool          // 是否覆盖已存在的 listen path(unix/pipe socket)

	// ===== 加密算法配置 =====
	CryptoKeyExchangeType    protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
	CryptoKeyRefreshInterval time.Duration
	CryptoAllowAlgorithms    []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE

	// ===== 压缩算法配置 =====
	CompressionAllowAlgorithms []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
	CompressionLevel           protocol.ATBUS_COMPRESSION_LEVEL

	// ===== 缓冲区配置 =====
	MessageSize      uint64 // max message size
	RecvBufferSize   uint64 // 接收缓冲区，和数据包大小有关
	SendBufferSize   uint64 // 发送缓冲区限制
	SendBufferNumber uint64 // 发送缓冲区静态 Buffer 数量限制，0 则为动态缓冲区
}

type StartConfigure struct {
	TimerTimepoint time.Time
}

func SetDefaultNodeConfigure(conf *NodeConfigure) {
	if conf == nil {
		return
	}

	conf.EventLoopContext = nil
	conf.UpstreamAddress = ""
	clear(conf.TopologyLabels)
	conf.LoopTimes = 256
	conf.TTL = 16
	conf.ProtocolVersion = int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_VERSION)
	conf.ProtocolMinimalVersion = int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_MINIMAL_VERSION)

	conf.FirstIdleTimeout = 30 * time.Second
	conf.PingInterval = 8 * time.Second
	conf.FaultTolerant = 2 // 允许最多失败2次，第3次直接失败，默认配置里3次ping包无响应则是最多24s可以发现节点下线
	conf.BackLog = 256
	conf.AccessTokenMaxNumber = 5
	if conf.AccessTokens != nil {
		clear(conf.AccessTokens)
		conf.AccessTokens = conf.AccessTokens[:0]
	}
	conf.OverwriteListenPath = false

	// 加密算法
	conf.CryptoKeyExchangeType = protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE
	conf.CryptoKeyRefreshInterval = 3 * time.Hour
	if conf.CryptoAllowAlgorithms != nil {
		clear(conf.CryptoAllowAlgorithms)
		conf.CryptoAllowAlgorithms = conf.CryptoAllowAlgorithms[:0]
	} else {
		conf.CryptoAllowAlgorithms = make([]protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, 0, 1)
	}

	var zeroCryptoAlgs protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	cryptoAlgs := zeroCryptoAlgs.Descriptor().Values()
	for i := 0; i < cryptoAlgs.Len(); i++ {
		if cryptoAlgs.Get(i).Number() == 0 {
			continue
		}
		conf.CryptoAllowAlgorithms = append(conf.CryptoAllowAlgorithms, protocol.ATBUS_CRYPTO_ALGORITHM_TYPE(cryptoAlgs.Get(i).Number()))
	}

	// 压缩算法
	if conf.CompressionAllowAlgorithms != nil {
		clear(conf.CompressionAllowAlgorithms)
		conf.CompressionAllowAlgorithms = conf.CompressionAllowAlgorithms[:0]
	} else {
		conf.CompressionAllowAlgorithms = make([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE, 0, 1)
	}
	var zeroCompress protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
	compressAlgs := zeroCompress.Descriptor().Values()
	for i := 0; i < compressAlgs.Len(); i++ {
		if compressAlgs.Get(i).Number() == 0 {
			continue
		}
		conf.CompressionAllowAlgorithms = append(conf.CompressionAllowAlgorithms, protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE(compressAlgs.Get(i).Number()))
	}
	conf.CompressionLevel = protocol.ATBUS_COMPRESSION_LEVEL_ATBUS_COMPRESSION_LEVEL_DEFAULT

	// Message配置
	conf.MessageSize = 2 * 1024 * 1024
	// recv_buffer_size 用于内存/共享内存通道的缓冲区长度，因为本机节点一般数量少所以默认设的大一点
	conf.RecvBufferSize = 268435456 // 256MB接收缓冲区

	// send_buffer_size 用于IO流通道的发送缓冲区长度，远程节点可能数量很多所以设的小一点
	conf.SendBufferSize = 8 * 1024 * 1024
	// 默认不使用静态缓冲区，所以设为0
	conf.SendBufferNumber = 0
}

func SetDefaultStartConfigure(conf *StartConfigure) {
	if conf == nil {
		return
	}

	conf.TimerTimepoint = time.Unix(0, 0)
}

type IoStreamCallbackEventType int32

const (
	IoStreamCallbackEventType_Accepted     IoStreamCallbackEventType = 0
	IoStreamCallbackEventType_Connected    IoStreamCallbackEventType = 1
	IoStreamCallbackEventType_Disconnected IoStreamCallbackEventType = 2
	IoStreamCallbackEventType_Received     IoStreamCallbackEventType = 3
	IoStreamCallbackEventType_Written      IoStreamCallbackEventType = 4
	IoStreamCallbackEventType_Max          IoStreamCallbackEventType = 5
)

type IoStreamCallbackFunc func(channel IoStreamChannel, conn IoStreamConnection, status int32, privData interface{})

type IoStreamCallbackEventHandleSet struct {
	callbacks [IoStreamCallbackEventType_Max]IoStreamCallbackFunc
}

// GetCallback returns the callback for the specified event type.
func (h *IoStreamCallbackEventHandleSet) GetCallback(eventType IoStreamCallbackEventType) IoStreamCallbackFunc {
	if eventType < 0 || eventType >= IoStreamCallbackEventType_Max {
		return nil
	}
	return h.callbacks[eventType]
}

// SetCallback sets the callback for the specified event type.
func (h *IoStreamCallbackEventHandleSet) SetCallback(eventType IoStreamCallbackEventType, callback IoStreamCallbackFunc) {
	if eventType < 0 || eventType >= IoStreamCallbackEventType_Max {
		return
	}
	h.callbacks[eventType] = callback
}

type IoStreamConnectionFlag uint16

const (
	IoStreamConnectionFlag_Listen  IoStreamConnectionFlag = 0x01
	IoStreamConnectionFlag_Connect IoStreamConnectionFlag = 0x02
	IoStreamConnectionFlag_Accept  IoStreamConnectionFlag = 0x04
	IoStreamConnectionFlag_Writing IoStreamConnectionFlag = 0x08
	IoStreamConnectionFlag_Closing IoStreamConnectionFlag = 0x10
)

type IoStreamConnectionStatus uint16

const (
	IoStreamConnectionStatus_Created       IoStreamConnectionStatus = 0
	IoStreamConnectionStatus_Connected     IoStreamConnectionStatus = 1
	IoStreamConnectionStatus_Disconnecting IoStreamConnectionStatus = 2
	IoStreamConnectionStatus_Disconnected  IoStreamConnectionStatus = 3
)

type IoStreamChannelFlag uint16

const (
	IoStreamChannelFlag_IsLoopOwner IoStreamChannelFlag = 0x01
	IoStreamChannelFlag_Closing     IoStreamChannelFlag = 0x02
	IoStreamChannelFlag_InCallback  IoStreamChannelFlag = 0x04
)

type IoStreamConnection interface {
	GetAddress() ChannelAddress
	GetStatus() IoStreamConnectionStatus
	SetFlag(f IoStreamConnectionFlag, v bool)
	GetFlag(f IoStreamConnectionFlag) bool

	GetChannel() IoStreamChannel

	// 事件响应函数集合
	GetEventHandleSet() *IoStreamCallbackEventHandleSet

	// 主动关闭连接的回调（为了减少额外分配而采用的缓存策略）
	GetProactivelyDisconnectCallback() IoStreamCallbackFunc

	// 数据区域
	// 读数据缓冲区(两种Buffer管理方式，一种动态，一种静态)
	// Note: 由于大多数数据包都比较小，
	// 当数据包比较小时会和动态 int 的数据包一起直接放在动态缓冲区中，这样可以减少内存拷贝次数。

	GetReadBufferManager() *buffer.BufferManager

	// 自定义数据区域
	SetPrivateData(data interface{})

	GetPrivateData() interface{}
}

type IoStreamConfigure struct {
	Keepalive time.Duration

	NoBlock bool
	NoDelay bool

	SendBufferStatic       uint64
	ReceiveBufferStatic    uint64
	SendBufferMaxSize      uint64
	SendBufferLimitSize    uint64
	ReceiveBufferMaxSize   uint64
	ReceiveBufferLimitSize uint64

	Backlog        int32
	ConfirmTimeout time.Duration

	MaxReadNetEgainCount             uint64
	MaxReadCheckBlockSizeFailedCount uint64
	MaxReadCheckHashFailedCount      uint64
}

// SetDefaultIoStreamConfigure sets the default values for IoStreamConfigure.
func SetDefaultIoStreamConfigure(conf *IoStreamConfigure) {
	if conf == nil {
		return
	}

	conf.Keepalive = 60 * time.Second
	conf.NoBlock = true
	conf.NoDelay = true

	conf.SendBufferStatic = 0
	conf.ReceiveBufferStatic = 0

	// 8MB max send buffer
	conf.SendBufferMaxSize = 8 * 1024 * 1024
	// 2MB max message size
	conf.SendBufferLimitSize = 2 * 1024 * 1024

	// 256MB max receive buffer
	conf.ReceiveBufferMaxSize = 256 * 1024 * 1024
	// 2MB max message size
	conf.ReceiveBufferLimitSize = 2 * 1024 * 1024

	conf.Backlog = 256
	conf.ConfirmTimeout = 30 * time.Second

	conf.MaxReadNetEgainCount = 1000
	conf.MaxReadCheckBlockSizeFailedCount = 3
	conf.MaxReadCheckHashFailedCount = 3
}

type IoStreamChannel interface {
	GetContext() context.Context

	SetFlag(f IoStreamConnectionFlag, v bool)

	GetFlag(f IoStreamConnectionFlag) bool

	// 事件响应函数集合
	GetEventHandleSet() *IoStreamCallbackEventHandleSet

	GetStatisticActiveRequestCount() uint64
	GetStatisticReadNetEgainCount() uint64
	GetStatisticCheckBlockSizeFailedCount() uint64
	GetStatisticCheckHashFailedCount() uint64

	// 自定义数据区域
	SetPrivateData(data interface{})

	GetPrivateData() interface{}

	// 核心操作方法
	Listen(addr string) ErrorType
	Connect(addr string) (IoStreamConnection, ErrorType)
	Send(conn IoStreamConnection, data []byte) ErrorType
	Disconnect(conn IoStreamConnection) ErrorType
	Close() ErrorType
}
