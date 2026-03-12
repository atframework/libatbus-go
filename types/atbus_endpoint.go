package libatbus_types

import "time"

type (
	EndpointFlag uint32
)

const (
	EndpointFlag_Resetting        EndpointFlag = 0x0001 /** 正在执行重置（防止递归死循环） **/
	EndpointFlag_ConnectionSorted EndpointFlag = 0x0002
	EndpointFlag_Destructing      EndpointFlag = 0x0004 /** 正在执行析构 **/
	EndpointFlag_HasListenPorc    EndpointFlag = 0x0008 /** 是否有proc类的listen地址 **/
	EndpointFlag_HasListenFd      EndpointFlag = 0x0010 /** 是否有fd类的listen地址 **/

	EndpointFlag_MutableFlags EndpointFlag = 0x0020 /** 可动态变化的属性起始边界 **/
	EndpointFlag_HasPingTimer EndpointFlag = 0x0040 /** 是否设置了ping定时器 **/
)

type Endpoint interface {
	GetOwner() Node

	Reset()

	GetId() BusIdType

	GetPid() int32

	GetHostname() string

	GetHashCode() string

	UpdateHashCode(code string)

	AddConnection(conn Connection, forceData bool) bool

	RemoveConnection(conn Connection) bool

	IsAvailable() bool

	GetFlags() uint32

	GetFlag(f EndpointFlag) bool

	SetFlag(f EndpointFlag, v bool)

	GetListenAddress() []ChannelAddress

	ClearListenAddress()

	AddListenAddress(addr string)

	UpdateSupportSchemes(schemes []string)

	IsSchemeSupported(scheme string) bool

	AddPingTimer()

	ClearPingTimer()

	// ============== connection functions ==============
	GetCtrlConnection(peer Endpoint) Connection

	GetDataConnection(peer Endpoint, enableFallbackCtrl bool) Connection

	GetDataConnectionCount(enableFallbackCtrl bool) int

	// ============== statistic functions ==============
	AddStatisticFault() uint64

	ClearStatisticFault()

	SetStatisticUnfinishedPing(p uint64)

	GetStatisticUnfinishedPing() uint64

	SetStatisticPingDelay(pd time.Duration, pongTimepoint time.Time)

	GetStatisticPingDelay() time.Duration

	GetStatisticLastPong() time.Time

	GetStatisticCreatedTime() time.Time

	GetStatisticPushStartTimes() uint64
	GetStatisticPushStartSize() uint64
	GetStatisticPushSuccessTimes() uint64
	GetStatisticPushSuccessSize() uint64
	GetStatisticPushFailedTimes() uint64
	GetStatisticPushFailedSize() uint64
	GetStatisticPullTimes() uint64
	GetStatisticPullSize() uint64
}
