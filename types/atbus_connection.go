package libatbus_types

import (
	"time"

	utils_memory "github.com/atframework/atframe-utils-go/memory"
)

type (
	ConnectionState int32
	ConnectionFlag  uint32
)

const (
	ConnectionState_Disconnected  ConnectionState = 0
	ConnectionState_Connecting    ConnectionState = 1
	ConnectionState_Handshaking   ConnectionState = 2
	ConnectionState_Connected     ConnectionState = 3
	ConnectionState_Disconnecting ConnectionState = 4
)

const (
	ConnectionFlag_RegProc         ConnectionFlag = 0x0001 /** 注册了proc记录到node，清理的时候需要移除 **/
	ConnectionFlag_RegFd           ConnectionFlag = 0x0002 /** 关联了fd到node或endpoint，清理的时候需要移除 **/
	ConnectionFlag_AccessShareAddr ConnectionFlag = 0x0004 /** 共享内部地址（内存通道的地址共享） **/
	ConnectionFlag_AccessShareHost ConnectionFlag = 0x0008 /** 共享物理机（共享内存通道的物理机共享） **/
	ConnectionFlag_Resetting       ConnectionFlag = 0x0010 /** 正在执行重置（防止递归死循环） **/
	ConnectionFlag_Destructing     ConnectionFlag = 0x0020 /** 正在执行析构（屏蔽某些接口） **/
	ConnectionFlag_ListenFd        ConnectionFlag = 0x0040 /** 是否是用于listen的连接 **/
	ConnectionFlag_Temporary       ConnectionFlag = 0x0080 /** 是否是临时连接 **/
	ConnectionFlag_PeerClosed      ConnectionFlag = 0x0100 /** 对端已关闭 **/
	ConnectionFlag_ServerMode      ConnectionFlag = 0x0200 /** 连接处于服务端模式 **/
	ConnectionFlag_ClientMode      ConnectionFlag = 0x0400 /** 连接处于客户端模式 **/
)

type TimerDescPair[V any] struct {
	Timeout time.Time
	Value   V
}

type TimerDescType[K comparable, V any] = utils_memory.LRUMap[K, TimerDescPair[V]]

type ConnectionStatistic struct {
	PushStartTimes   uint64
	PushStartSize    uint64
	PushSuccessTimes uint64
	PushSuccessSize  uint64
	PushFailedTimes  uint64
	PushFailedSize   uint64

	PullStartTimes uint64
	PullStartSize  uint64

	FaultCount uint64
}

type Connection interface {
	Reset()

	Proc() ErrorType

	Listen() ErrorType

	Connect() ErrorType

	Disconnect() ErrorType

	Push(buffer []byte) ErrorType

	AddStatFault() uint64

	ClearStatFault()

	GetAddress() ChannelAddress

	IsConnected() bool

	IsRunning() bool

	GetBinding() Endpoint

	GetStatus() ConnectionState

	CheckFlag(flag ConnectionFlag) bool

	SetTemporary()

	GetStatistic() ConnectionStatistic

	GetConnectionContext() ConnectionContext

	RemoveOwnerChecker()
}
