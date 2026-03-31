package libatbus_types

import (
	"context"
	"time"

	utils_log "github.com/atframework/atframe-utils-go/log"
	protocol "github.com/atframework/libatbus-go/protocol"
)

type (
	NodeState              int32
	NodeFlag               uint32
	NodeSendDataOptionFlag uint32
	NodeGetPeerOptionFlag  uint16
)

const (
	NodeState_Created            NodeState = 0
	NodeState_Inited             NodeState = 1
	NodeState_LostUpstream       NodeState = 2
	NodeState_ConnectingUpstream NodeState = 3
	NodeState_Running            NodeState = 4
)

const (
	NodeFlag_None            NodeFlag = 0x0000
	NodeFlag_Resetting       NodeFlag = 0x0001 // 正在重置
	NodeFlag_ResettingGc     NodeFlag = 0x0002 // 正在重置且正准备 GC 或 GC 流程已完成
	NodeFlag_Actived         NodeFlag = 0x0004 // 已激活
	NodeFlag_UpstreamRegDone NodeFlag = 0x0008 // 已通过父节点注册
	NodeFlag_Shutdown        NodeFlag = 0x0010 // 已完成关闭前的资源回收
	NodeFlag_RecvSelfMsg     NodeFlag = 0x0020 // 正在接收发给自己的信息
	NodeFlag_InCallback      NodeFlag = 0x0040 // 在回调函数中
	NodeFlag_InProc          NodeFlag = 0x0080 // 在 Proc 函数中
	NodeFlag_InPoll          NodeFlag = 0x0100 // 在 Poll 函数中
	NodeFlag_InGcEndpoints   NodeFlag = 0x0200 // 在清理 endpoint 过程中
	NodeFlag_InGcConnections NodeFlag = 0x0400 // 在清理 connection 过程中
)

const (
	NodeSendDataOptionFlag_RequiredResponse NodeSendDataOptionFlag = 0x0001 // 是否强制需要回包（默认情况下如果发送成功是没有回包通知的）
	NodeSendDataOptionFlag_NoUpstream       NodeSendDataOptionFlag = 0x0002 // 是否禁止上游转发
)

const (
	NodeGetPeerOptionFlag_NoUpstream NodeGetPeerOptionFlag = 0x0001 // 不允许上游转发
)

type NodeSendDataOptions struct {
	flags    uint32
	sequence uint64
}

func (o *NodeSendDataOptions) GetFlag(f NodeSendDataOptionFlag) bool {
	if o == nil {
		return false
	}

	return (o.flags & uint32(f)) != 0
}

func (o *NodeSendDataOptions) SetFlag(f NodeSendDataOptionFlag, v bool) {
	if o == nil {
		return
	}

	if v {
		o.flags |= uint32(f)
	} else {
		o.flags &^= uint32(f)
	}
}

func (o *NodeSendDataOptions) GetSequence() uint64 {
	if o == nil {
		return 0
	}

	return o.sequence
}

func (o *NodeSendDataOptions) SetSequence(seq uint64) {
	if o == nil {
		return
	}

	o.sequence = seq
}

func CreateNodeSendDataOptions() *NodeSendDataOptions {
	return &NodeSendDataOptions{}
}

type NodeGetPeerOptions struct {
	flags     uint16
	blacklist []BusIdType
}

func (o *NodeGetPeerOptions) GetFlag(f NodeGetPeerOptionFlag) bool {
	if o == nil {
		return false
	}

	return (o.flags & uint16(f)) != 0
}

func (o *NodeGetPeerOptions) SetFlag(f NodeGetPeerOptionFlag, v bool) {
	if o == nil {
		return
	}

	if v {
		o.flags |= uint16(f)
	} else {
		o.flags &^= uint16(f)
	}
}

func (o *NodeGetPeerOptions) SetBlacklist(blacklist []BusIdType) {
	if o == nil {
		return
	}

	o.blacklist = blacklist
}

func (o *NodeGetPeerOptions) GetBlacklist() []BusIdType {
	if o == nil {
		return nil
	}

	return o.blacklist
}

func CreateNodeGetPeerOptions() *NodeGetPeerOptions {
	return &NodeGetPeerOptions{}
}

// NodeEventHandleSet is the Go equivalent of the C++ `event_handle_set_t`.
//
// It holds callbacks for node/endpoint/connection events.
// All callback function signatures are declared as named types for consistency and reuse.

// NodeOnForwardRequestFunc is called when a message is received.
// Parameters: node, source endpoint, source connection, decoded message, raw payload bytes.
type NodeOnForwardRequestFunc func(n Node, ep Endpoint, conn Connection, msg *Message, data []byte) ErrorType

// NodeOnForwardResponseFunc is called when a forwarded message send fails or (optionally) succeeds.
//
// Note: unless send is marked as requiring a response/notification, success usually does not trigger a callback.
// Parameters: node, source endpoint, source connection, message (may be nil depending on context).
type NodeOnForwardResponseFunc func(n Node, ep Endpoint, conn Connection, msg *Message) ErrorType

// NodeOnRegisterFunc is called when a new peer endpoint is registered.
// Parameters: node, endpoint, connection, status/error code.
type NodeOnRegisterFunc func(n Node, ep Endpoint, conn Connection, errCode ErrorType) ErrorType

// NodeOnNodeDownFunc is called when the node is shutting down.
// Parameters: node, reason.
type NodeOnNodeDownFunc func(n Node, errCode ErrorType) ErrorType

// NodeOnNodeUpFunc is called when the node starts serving.
// Parameters: node, status (typically EN_ATBUS_ERR_SUCCESS).
type NodeOnNodeUpFunc func(n Node, errCode ErrorType) ErrorType

// NodeOnInvalidConnectionFunc is called when a connection becomes invalid.
// Parameters: node, connection, error code (typically EN_ATBUS_ERR_NODE_TIMEOUT).
type NodeOnInvalidConnectionFunc func(n Node, conn Connection, errCode ErrorType) ErrorType

// NodeOnNewConnectionFunc is called when a new connection is created.
// Parameters: node, connection.
type NodeOnNewConnectionFunc func(n Node, conn Connection) ErrorType

// NodeOnCloseConnectionFunc is called when a connection is closed.
// Parameters: node, endpoint, connection.
type NodeOnCloseConnectionFunc func(n Node, ep Endpoint, conn Connection) ErrorType

// NodeOnCustomCommandRequestFunc is called when a custom command request is received.
//
// Parameters:
//   - node, endpoint, connection
//   - source bus id
//   - argv: list of raw arg byte slices
//   - rsp: output response lines (some transports may ignore cross-node responses)
type NodeOnCustomCommandRequestFunc func(n Node, ep Endpoint, conn Connection, from BusIdType, argv [][]byte) (result ErrorType, response [][]byte)

// NodeOnCustomCommandResponseFunc is called when a custom command response is received.
// Parameters: node, endpoint, connection, source bus id, response data list, sequence of the request.
type (
	NodeOnCustomCommandResponseFunc func(n Node, ep Endpoint, conn Connection, from BusIdType, rspData [][]byte, sequence uint64) ErrorType
	// NodeOnEndpointEventFunc is called when an endpoint is added/removed.
	// Parameters: node, endpoint, status/error code.
	NodeOnEndpointEventFunc func(n Node, ep Endpoint, status ErrorType) ErrorType
)

// NodeOnPingPongEndpointFunc is called when a ping/pong message is received from an endpoint.
// Parameters: node, endpoint, decoded message, ping data.
type NodeOnPingPongEndpointFunc func(n Node, ep Endpoint, msg *Message, ping *protocol.PingData) ErrorType

// NodeOnTopologyUpdateUpstreamFunc is called when the topology upstream is updated.
// Parameters: node, self, new upstream, topology data of self.
type NodeOnTopologyUpdateUpstreamFunc func(n Node, self TopologyPeer, upstream TopologyPeer, data *TopologyData) ErrorType

// NodeEventHandleSet collects all callbacks.
type NodeEventHandleSet struct {
	OnForwardRequest         NodeOnForwardRequestFunc
	OnForwardResponse        NodeOnForwardResponseFunc
	OnRegister               NodeOnRegisterFunc
	OnNodeDown               NodeOnNodeDownFunc
	OnNodeUp                 NodeOnNodeUpFunc
	OnInvalidConnection      NodeOnInvalidConnectionFunc
	OnNewConnection          NodeOnNewConnectionFunc
	OnCloseConnection        NodeOnCloseConnectionFunc
	OnCustomCommandRequest   NodeOnCustomCommandRequestFunc
	OnCustomCommandResponse  NodeOnCustomCommandResponseFunc
	OnEndpointAdded          NodeOnEndpointEventFunc
	OnEndpointRemoved        NodeOnEndpointEventFunc
	OnEndpointPing           NodeOnPingPongEndpointFunc
	OnEndpointPong           NodeOnPingPongEndpointFunc
	OnTopologyUpdateUpstream NodeOnTopologyUpdateUpstreamFunc
}

type EndpointCollectionType = map[BusIdType]Endpoint

type Node interface {
	Init(id BusIdType, conf *NodeConfigure) ErrorType

	ReloadCrypto(cryptoKeyExchangeType protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE,
		cryptoKeyRefreshInterval time.Duration,
		cryptoAllowAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) ErrorType

	ReloadCompression(compressionAllowAlgorithms []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE,
		compressionLevel protocol.ATBUS_COMPRESSION_LEVEL) ErrorType

	Start() ErrorType

	StartWithConfigure(conf *StartConfigure) ErrorType

	Reset() ErrorType

	Proc(now time.Time) (int32, ErrorType)

	Poll() (int32, ErrorType)

	Listen(address string) ErrorType

	Connect(address string) ErrorType

	ConnectWithEndpoint(address string, ep Endpoint) ErrorType

	Disconnect(id BusIdType) ErrorType

	GetCryptoKeyExchangeType() protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE

	// ================= send/receive functions =================
	// SendData 发送数据。
	//
	// Parameters:
	//   - tid: 发送目标 ID
	//   - t: 自定义类型，将作为 message.head.type 字段传递，可用于业务区分服务类型
	//   - data: 要发送的数据块
	//
	// Return: 0 或错误码。
	//
	// Note: 接收端收到的数据很可能不是地址对齐的，所以不建议发送内存数据对象（struct/class）。
	// 如果必须发送内存数据对象，接收端一定要 memcpy，不能直接类型转换，除非手动设置了地址对齐规则。
	SendData(tid BusIdType, t int32, data []byte) ErrorType

	// SendDataWithOptions 发送数据（带选项）。
	//
	// Parameters:
	//   - tid: 发送目标 ID
	//   - t: 自定义类型，将作为 message.head.type 字段传递，可用于业务区分服务类型
	//   - data: 要发送的数据块
	//   - options: 发送选项；如果未设置 sequence，会自动分配并写回
	//
	// Return: 0 或错误码。
	//
	// Note: 接收端收到的数据很可能不是地址对齐的，所以不建议发送内存数据对象（struct/class）。
	// 如果必须发送内存数据对象，接收端一定要 memcpy，不能直接类型转换，除非手动设置了地址对齐规则。
	SendDataWithOptions(tid BusIdType, t int32, data []byte, options *NodeSendDataOptions) ErrorType

	// SendCustomCommand 发送自定义命令。
	//
	// Parameters:
	//   - tid: 发送目标 ID
	//   - args: 自定义消息内容数组
	//
	// Return: 0 或错误码。
	SendCustomCommand(tid BusIdType, args [][]byte) ErrorType

	// SendCustomCommandWithOptions 发送自定义命令（带选项）。
	//
	// Parameters:
	//   - tid: 发送目标 ID
	//   - args: 自定义消息内容数组
	//   - options: 发送选项；如果未设置 sequence，会自动分配并写回
	//
	// Return: 0 或错误码。
	SendCustomCommandWithOptions(tid BusIdType, args [][]byte, options *NodeSendDataOptions) ErrorType

	// ================= endpoint management functions =================
	// GetPeerChannel 获取远程发送目标信息。
	//
	// Parameters:
	//   - tid: 发送目标 ID，不能是自己的 BUS ID
	//   - fn: 获取有效连接的接口，用以区分数据通道和控制通道
	//   - options: 获取选项
	//
	// Return: (0 或错误码, 发送目标 endpoint, 发送连接 connection)。
	GetPeerChannel(tid BusIdType, fn func(from Endpoint, to Endpoint) Connection, options *NodeGetPeerOptions) (ErrorType, Endpoint, Connection, TopologyPeer)

	// SetTopologyUpstream 设置节点的上游节点信息。
	//
	// Parameters:
	//   - tid: 上游节点 ID
	SetTopologyUpstream(tid BusIdType)

	// CreateEndpoint 创建 endpoint 对象，并设置生命周期检查。
	// 并不会自动添加到本地，请在初始化完成后调用 AddEndpoint 添加。
	//
	// Parameters:
	//   - tid: 目标 endpoint 的 bus id
	//   - hostName: 目标 endpoint 的主机名
	//   - pid: 目标 endpoint 的进程 ID
	// Return: endpoint 对象，可能为 nil。
	CreateEndpoint(tid BusIdType, hostName string, pid int) Endpoint

	// GetEndpoint 获取本地 endpoint 对象。
	// Return: endpoint 对象，可能为 nil。
	GetEndpoint(tid BusIdType) Endpoint

	// AddEndpoint 添加目标端点。
	// Return: 0 或错误码。
	AddEndpoint(ep Endpoint) ErrorType

	// RemoveEndpoint 移除目标端点。
	// Return: 0 或错误码。
	RemoveEndpoint(ep Endpoint) ErrorType

	// IsEndpointAvailable 是否有到对端的数据通道（可以向对端发送数据）。
	// Note: 如果只有控制通道没有数据通道，返回 false。
	IsEndpointAvailable(tid BusIdType) bool

	// CheckAccessHash 检查 access token 集合的有效性。
	// Return: 没有检查通过的 access token 则返回 false。
	CheckAccessHash(accessKey *protocol.AccessData, plainText string, conn Connection) bool

	// GetHashCode 获取节点的 hash code。
	GetHashCode() string

	GetIoStreamChannel() IoStreamChannel

	GetSelfEndpoint() Endpoint

	GetUpstreamEndpoint() Endpoint

	GetTopologyRelation(tid BusIdType) (TopologyRelationType, TopologyPeer)

	GetImmediateEndpointSet() EndpointCollectionType

	// ================= internal message functions =================
	// SendDataMessage 发送数据消息。
	// Return: (0 或错误码, 发送目标 endpoint, 发送连接 connection)。
	SendDataMessage(tid BusIdType, message *Message, options *NodeSendDataOptions) (ErrorType, Endpoint, Connection)

	// SendCtrlMessage 发送控制消息。
	// Return: (0 或错误码, 发送目标 endpoint, 发送连接 connection)。
	SendCtrlMessage(tid BusIdType, message *Message, options *NodeSendDataOptions) (ErrorType, Endpoint, Connection)

	OnReceiveData(ep Endpoint, conn Connection, message *Message, data []byte)

	OnReceiveForwardResponse(ep Endpoint, conn Connection, message *Message)

	OnDisconnect(ep Endpoint, conn Connection) ErrorType

	OnNewConnection(conn Connection) ErrorType

	OnRegister(ep Endpoint, conn Connection, code ErrorType)

	OnActived()

	OnShutdown(code ErrorType) ErrorType

	OnUpstreamRegisterDone()

	OnCustomCommandRequest(ep Endpoint, conn Connection, from BusIdType, argv [][]byte) (ErrorType, [][]byte)

	OnCustomCommandResponse(ep Endpoint, conn Connection, from BusIdType, argv [][]byte, sequence uint64) ErrorType

	OnPing(ep Endpoint, message *Message, body *protocol.PingData) ErrorType

	OnPong(ep Endpoint, message *Message, body *protocol.PingData) ErrorType

	DispatchAllSelfMessages() int32

	// ================= lifetime management functions =================

	GetContext() context.Context

	// Shutdown 关闭 node。
	//
	// Note: 如果需要在关闭前执行资源回收，可以在 on_node_down_fn_t 回调中返回非 0 值来阻止 node 的 reset 操作，
	// 并在资源释放完成后再调用 Shutdown，在第二次 on_node_down_fn_t 回调中返回 0 值。
	//
	// Note: 或者也可以通过 ref_object 和 unref_object 来标记和解除数据引用，reset 函数会执行事件 loop 直到所有引用的资源被移除。
	Shutdown(reason ErrorType) ErrorType

	FatalShutdown(ep Endpoint, conn Connection, code ErrorType, err error) ErrorType

	SetLogger(logger *utils_log.Logger)

	GetLogger() *utils_log.Logger

	IsDebugMessageVerboseEnabled() bool

	EnableDebugMessageVerbose()

	DisableDebugMessageVerbose()

	AddEndpointGcList(ep Endpoint)

	AddConnectionGcList(conn Connection)

	// ================= event handle functions =================
	SetEventHandleOnForwardRequest(handle NodeOnForwardRequestFunc)
	GetEventHandleOnForwardRequest() NodeOnForwardRequestFunc

	SetEventHandleOnForwardResponse(handle NodeOnForwardResponseFunc)
	GetEventHandleOnForwardResponse() NodeOnForwardResponseFunc

	SetEventHandleOnRegister(handle NodeOnRegisterFunc)
	GetEventHandleOnRegister() NodeOnRegisterFunc

	SetEventHandleOnShutdown(handle NodeOnNodeDownFunc)
	GetEventHandleOnShutdown() NodeOnNodeDownFunc

	SetEventHandleOnAvailable(handle NodeOnNodeUpFunc)
	GetEventHandleOnAvailable() NodeOnNodeUpFunc

	SetEventHandleOnInvalidConnection(handle NodeOnInvalidConnectionFunc)
	GetEventHandleOnInvalidConnection() NodeOnInvalidConnectionFunc

	SetEventHandleOnNewConnection(handle NodeOnNewConnectionFunc)
	GetEventHandleOnNewConnection() NodeOnNewConnectionFunc

	SetEventHandleOnCloseConnection(handle NodeOnCloseConnectionFunc)
	GetEventHandleOnCloseConnection() NodeOnCloseConnectionFunc

	SetEventHandleOnCustomCommandRequest(handle NodeOnCustomCommandRequestFunc)
	GetEventHandleOnCustomCommandRequest() NodeOnCustomCommandRequestFunc

	SetEventHandleOnCustomCommandResponse(handle NodeOnCustomCommandResponseFunc)
	GetEventHandleOnCustomCommandResponse() NodeOnCustomCommandResponseFunc

	SetEventHandleOnAddEndpoint(handle NodeOnEndpointEventFunc)
	GetEventHandleOnAddEndpoint() NodeOnEndpointEventFunc

	SetEventHandleOnRemoveEndpoint(handle NodeOnEndpointEventFunc)
	GetEventHandleOnRemoveEndpoint() NodeOnEndpointEventFunc

	SetEventHandleOnPingEndpoint(handle NodeOnPingPongEndpointFunc)
	GetEventHandleOnPingEndpoint() NodeOnPingPongEndpointFunc

	SetEventHandleOnPongEndpoint(handle NodeOnPingPongEndpointFunc)
	GetEventHandleOnPongEndpoint() NodeOnPingPongEndpointFunc

	SetEventHandleOnTopologyUpdateUpstream(handle NodeOnTopologyUpdateUpstreamFunc)
	GetEventHandleOnTopologyUpdateUpstream() NodeOnTopologyUpdateUpstreamFunc

	// ================= date field setter and getter =================
	GetIoStreamConfigure() *IoStreamConfigure

	GetId() BusIdType

	GetConfigure() *NodeConfigure

	CheckFlag(f NodeFlag) bool

	GetState() NodeState

	GetTimerTick() time.Time

	GetTopologyRegistry() TopologyRegistry

	GetPid() int

	GetHostname() string

	SetHostname(hostname string, force bool) bool

	GetProtocolVersion() int32

	GetProtocolMinimalVersion() int32

	GetListenList() []ChannelAddress

	// ================= statistic functions =================
	AddStatisticDispatchTimes()

	// ================= utility functions =================
	AllocateMessageSequence() uint64

	LogDebug(ep Endpoint, conn Connection, m *Message, msg string, args ...any)
	LogInfo(ep Endpoint, conn Connection, msg string, args ...any)
	LogError(ep Endpoint, conn Connection, status int, errcode ErrorType, msg string, args ...any)
}
