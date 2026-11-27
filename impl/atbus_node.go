package libatbus_impl

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	lu "github.com/atframework/atframe-utils-go/lang_utility"
	utils_log "github.com/atframework/atframe-utils-go/log"
	utils_memory "github.com/atframework/atframe-utils-go/memory"

	buffer "github.com/atframework/libatbus-go/buffer"
	io_stream "github.com/atframework/libatbus-go/channel/io_stream"
	error_code "github.com/atframework/libatbus-go/error_code"
	message_handle "github.com/atframework/libatbus-go/message_handle"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

var _ types.Node = (*Node)(nil)

type nodeUpstreamInfo struct {
	node *Endpoint
}

type nodeStatistic struct {
	DispatchTimes uint64
}

type nodeEventTimer struct {
	tick time.Time

	upstreamOpTimepoint     time.Time
	upstreamOpRetryInterval time.Duration
	pingListMu              sync.Mutex
	pingList                types.TimerDescType[*Endpoint, *Endpoint]
	connectingListMu        sync.Mutex
	connectingListByChannel map[string]*Connection
	connectingListByPtr     types.TimerDescType[*Connection, *Connection]
	pendingEndpointGcList   list.List
	pendingConnectionGcList list.List
}

type endpointCollection struct {
	endpointInstance  map[types.BusIdType]*Endpoint
	endpointInterface map[types.BusIdType]types.Endpoint
}

type nodeIoStreamEvent struct {
	kind   types.IoStreamCallbackEventType
	conn   types.IoStreamConnection
	result int32
	data   []byte
	size   int
}

type Node struct {
	eventTimer    nodeEventTimer
	eventCancelFn context.CancelFunc
	flags         uint16
	configure     types.NodeConfigure
	hashCode      string
	selfHostname  string
	selfPid       int32

	ioStreamConfigure *types.IoStreamConfigure
	ioStreamChannel   types.IoStreamChannel
	ioStreamEvents    chan nodeIoStreamEvent

	// 加密配置
	cryptoKeyExchangeType protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE

	state     types.NodeState
	self      *Endpoint
	upstream  nodeUpstreamInfo
	nodeRoute endpointCollection

	topologyRegistry *TopologyRegistry
	topologyData     *types.TopologyData

	eventHandleset types.NodeEventHandleSet

	messageSequenceAllocator        atomic.Uint64
	logger                          *utils_log.Logger
	loggerEnableDebugMessageVerbose bool

	selfDataMessages    list.List
	selfCommandMessages list.List

	stat nodeStatistic
}

// 2026-01-01 00:00:00 UTC in microseconds (matches C++ kInitialSequenceOffset)
const kInitialSequenceOffset uint64 = 1767225600 * 1000000

// NewNode constructs a new Node instance with initialized sequence offset.
func NewNode() *Node {
	n := &Node{}
	n.initMessageSequenceAllocator()
	return n
}

func (n *Node) initMessageSequenceAllocator() {
	if n == nil || n.messageSequenceAllocator.Load() != 0 {
		return
	}

	nowMicros := time.Now().UnixMicro()
	if nowMicros > int64(kInitialSequenceOffset) {
		n.messageSequenceAllocator.CompareAndSwap(0, uint64(nowMicros)-kInitialSequenceOffset)
	}
}

// nextUpstreamOpRetryInterval computes the next retry delay for upstream operations.
// Matches C++ next_upstream_op_retry_interval: stays at RetryInterval before activation
// or when MaxRetryInterval <= RetryInterval; doubles after activation up to MaxRetryInterval.
func nextUpstreamOpRetryInterval(conf *types.NodeConfigure, current time.Duration, actived bool) time.Duration {
	if conf == nil {
		return 0
	}
	if current < conf.RetryInterval || conf.MaxRetryInterval <= conf.RetryInterval || !actived {
		return conf.RetryInterval
	}

	if current > conf.MaxRetryInterval/2 {
		return conf.MaxRetryInterval
	}
	return current * 2
}

func cloneNodeConfigure(dst *types.NodeConfigure, src *types.NodeConfigure) {
	if dst == nil || src == nil {
		return
	}

	*dst = *src

	if len(src.TopologyLabels) > 0 {
		dst.TopologyLabels = make(map[string]string, len(src.TopologyLabels))
		for k, v := range src.TopologyLabels {
			dst.TopologyLabels[k] = v
		}
	} else {
		dst.TopologyLabels = nil
	}

	if len(src.NodeLabels) > 0 {
		dst.NodeLabels = make(map[string]string, len(src.NodeLabels))
		for k, v := range src.NodeLabels {
			dst.NodeLabels[k] = v
		}
	} else {
		dst.NodeLabels = nil
	}

	dst.Gateway = types.CloneGateways(src.Gateway)

	if len(src.AccessTokens) > 0 {
		dst.AccessTokens = make([][]byte, len(src.AccessTokens))
		for i, tok := range src.AccessTokens {
			if tok != nil {
				copied := make([]byte, len(tok))
				copy(copied, tok)
				dst.AccessTokens[i] = copied
			}
		}
	} else {
		dst.AccessTokens = nil
	}

	if len(src.CryptoAllowAlgorithms) > 0 {
		dst.CryptoAllowAlgorithms = make([]protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, len(src.CryptoAllowAlgorithms))
		copy(dst.CryptoAllowAlgorithms, src.CryptoAllowAlgorithms)
	} else {
		dst.CryptoAllowAlgorithms = nil
	}

	if len(src.CompressionAllowAlgorithms) > 0 {
		dst.CompressionAllowAlgorithms = make([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE, len(src.CompressionAllowAlgorithms))
		copy(dst.CompressionAllowAlgorithms, src.CompressionAllowAlgorithms)
	} else {
		dst.CompressionAllowAlgorithms = nil
	}
}

type sha256Hash struct {
	data []byte
}

func (h sha256Hash) hex() string {
	return hex.EncodeToString(h.data)
}

func sha256String(value string) sha256Hash {
	sum := sha256.Sum256([]byte(value))
	return sha256Hash{data: sum[:]}
}

func isInGetPeerBlacklist(id types.BusIdType, options *types.NodeGetPeerOptions) bool {
	if options == nil {
		return false
	}

	for _, bid := range options.GetBlacklist() {
		if bid == id {
			return true
		}
	}

	return false
}

type nodeFlagGuard struct {
	owner *Node
	flag  types.NodeFlag
}

func (fg *nodeFlagGuard) closeFlag() {
	if fg == nil {
		return
	}

	if fg.owner == nil {
		return
	}

	fg.owner.flags &^= uint16(fg.flag)
	fg.owner = nil
}

func (fg *nodeFlagGuard) openFlag(owner *Node, flag types.NodeFlag) bool {
	if fg == nil {
		return false
	}

	if owner == fg.owner && flag == fg.flag {
		return true
	}

	if owner.flags&uint16(flag) != 0 {
		return true
	}

	if fg.owner != nil {
		fg.closeFlag()
	}

	owner.flags |= uint16(flag)
	fg.owner = owner
	fg.flag = flag
	return false
}

func (n *Node) Init(id types.BusIdType, conf *types.NodeConfigure) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// If not in Created state, reset first
	if n.state != types.NodeState_Created {
		n.Reset()
	}

	n.initMessageSequenceAllocator()
	n.eventTimer.upstreamOpRetryInterval = 0

	// Apply configuration (deep copy maps/slices to avoid aliasing caller's config)
	if conf == nil {
		types.SetDefaultNodeConfigure(&n.configure)
	} else {
		cloneNodeConfigure(&n.configure, conf)
	}

	// Initialize topology
	if n.topologyRegistry == nil {
		n.topologyRegistry = CreateTopologyRegistry()
	}
	if n.topologyData == nil {
		n.topologyData = &types.TopologyData{}
	}
	n.topologyData.Pid = n.GetSelfPid()
	n.topologyData.Hostname = n.GetSelfHostname()
	n.topologyData.Labels = n.configure.TopologyLabels

	if id != 0 {
		// Initialize with self peer data
		n.topologyRegistry.UpdatePeer(id, 0, n.topologyData)
	}

	// Load crypto and compression configuration
	n.ReloadCrypto(n.configure.CryptoKeyExchangeType,
		n.configure.CryptoKeyRefreshInterval,
		n.configure.CryptoAllowAlgorithms)
	n.ReloadCompression(n.configure.CompressionAllowAlgorithms,
		n.configure.CompressionLevel)

	// Ensure access tokens don't exceed max
	if uint32(len(n.configure.AccessTokens)) > n.configure.AccessTokenMaxNumber {
		n.configure.AccessTokens = n.configure.AccessTokens[:n.configure.AccessTokenMaxNumber]
	}

	// Follow protocol version, not input configure
	n.configure.ProtocolVersion = int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_VERSION)
	n.configure.ProtocolMinimalVersion = int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_MINIMAL_VERSION)

	// Create IoStreamChannel
	if n.configure.EventLoopContext == nil {
		n.configure.EventLoopContext, n.eventCancelFn = context.WithCancel(context.Background())
	}
	ioStreamConf := n.GetIoStreamConfigure()
	n.ioStreamChannel = io_stream.NewIoStreamChannel(n.configure.EventLoopContext, ioStreamConf)
	n.setupIoStreamCallbacks()

	// Create self endpoint
	n.self = CreateEndpoint(n, id, int64(n.GetSelfPid()), n.GetSelfHostname())
	if n.self == nil {
		return error_code.EN_ATBUS_ERR_MALLOC
	}
	// 集群隔离配置
	n.ReloadSelfEndpoint(n.configure.Scope, n.configure.NamespaceName, n.configure.NodeLabels, n.configure.Gateway)
	n.self.ClearPingTimer()
	n.eventTimer.pingList = *utils_memory.NewLRUMap[*Endpoint, types.TimerDescPair[*Endpoint]](0)
	n.eventTimer.connectingListByChannel = make(map[string]*Connection)
	n.eventTimer.connectingListByPtr = *utils_memory.NewLRUMap[*Connection, types.TimerDescPair[*Connection]](0)

	// Initialize node route collection
	n.nodeRoute.endpointInstance = make(map[types.BusIdType]*Endpoint)
	n.nodeRoute.endpointInterface = make(map[types.BusIdType]types.Endpoint)

	// Set state to initialized
	n.setState(types.NodeState_Inited)

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) setupIoStreamCallbacks() {
	if n == nil || n.ioStreamChannel == nil {
		return
	}

	events := make(chan nodeIoStreamEvent, max(1, n.configure.LoopTimes))
	n.ioStreamEvents = events
	done := n.ioStreamChannel.GetContext().Done()
	handles := n.ioStreamChannel.GetEventHandleSet()
	// The outgoing connection is initialized synchronously by Connection.Connect.
	handles.SetCallback(types.IoStreamCallbackEventType_Connected, func(types.IoStreamChannel, types.IoStreamConnection, int32, interface{}) {})
	for _, kind := range [...]types.IoStreamCallbackEventType{
		types.IoStreamCallbackEventType_Accepted,
		types.IoStreamCallbackEventType_Received,
		types.IoStreamCallbackEventType_Disconnected,
		types.IoStreamCallbackEventType_Written,
	} {
		handles.SetCallback(kind, func(_ types.IoStreamChannel, conn types.IoStreamConnection, result int32, privateData interface{}) {
			payload, _ := privateData.([]byte)
			event := nodeIoStreamEvent{kind: kind, conn: conn, result: result, size: len(payload)}
			if kind == types.IoStreamCallbackEventType_Received {
				// The stream reuses its read buffer after the callback returns.
				event.data = bytes.Clone(payload)
			}
			select {
			case events <- event:
			case <-done:
			}
		})
	}
}

// dispatchIoStreamEvents keeps registration, routing and disconnect callbacks on the node loop.
func (n *Node) dispatchIoStreamEvents() int32 {
	before := n.stat.DispatchTimes
	var processed int32
	for processed < max(1, n.configure.LoopTimes) && !n.CheckFlag(types.NodeFlag_Shutdown) {
		select {
		case event := <-n.ioStreamEvents:
			switch event.kind {
			case types.IoStreamCallbackEventType_Accepted:
				n.onIoStreamAccepted(event.conn)
			case types.IoStreamCallbackEventType_Received:
				n.onIoStreamReceived(event.conn, event.result, event.data)
			case types.IoStreamCallbackEventType_Disconnected:
				n.onIoStreamDisconnected(event.conn)
			case types.IoStreamCallbackEventType_Written:
				n.onIoStreamWritten(event.conn, event.result, event.size)
			}
			processed++
		default:
			return int32(n.stat.DispatchTimes - before)
		}
	}
	return int32(n.stat.DispatchTimes - before)
}

func (n *Node) onIoStreamAccepted(ioConn types.IoStreamConnection) {
	if n == nil || ioConn == nil {
		return
	}

	if existed, ok := ioConn.GetPrivateData().(*Connection); ok && existed != nil {
		return
	}

	if n.configure.BackLog > 0 && n.GetConnectionTimerSize() >= int(n.configure.BackLog) {
		n.ioStreamChannel.Disconnect(ioConn)
		return
	}

	conn := CreateConnection(n, ioConn.GetAddress().GetAddress(), false)
	if conn == nil {
		if n.ioStreamChannel != nil {
			n.ioStreamChannel.Disconnect(ioConn)
		}
		return
	}

	conn.setFlag(types.ConnectionFlag_RegFd, true)
	conn.setFlag(types.ConnectionFlag_ServerMode, true)
	conn.setStatus(types.ConnectionState_Handshaking)
	if concreteConn, ok := ioConn.(*io_stream.IoStreamConnection); ok {
		conn.SetIoStreamConnection(concreteConn)
	}
	ioConn.SetPrivateData(conn)

	n.OnNewConnection(conn)
}

func (n *Node) onIoStreamReceived(ioConn types.IoStreamConnection, status int32, payload []byte) {
	if n == nil || ioConn == nil {
		return
	}

	conn, ok := ioConn.GetPrivateData().(*Connection)
	if !ok || conn == nil {
		return
	}

	if status < 0 {
		n.onReceiveMessage(conn, nil, int(status), error_code.ErrorType(status))
		return
	}
	if len(payload) == 0 {
		return
	}

	// statistic — match C++ iostream_on_recv_cb pull tracking
	conn.statistic.PullStartTimes++
	conn.statistic.PullStartSize += uint64(len(payload))

	msg := types.NewMessage()
	errCode := conn.GetConnectionContext().UnpackMessage(msg, payload, int(n.GetConfigure().MessageSize))
	n.onReceiveMessage(conn, msg, int(status), errCode)
}

func (n *Node) onIoStreamDisconnected(ioConn types.IoStreamConnection) {
	if n == nil || ioConn == nil {
		return
	}

	conn, ok := ioConn.GetPrivateData().(*Connection)
	if !ok || conn == nil {
		return
	}

	if streamConn, ok := ioConn.(*io_stream.IoStreamConnection); ok && streamConn.IsPeerClosed() {
		conn.setFlag(types.ConnectionFlag_PeerClosed, true)
	}
	conn.SetIoStreamConnection(nil)
	conn.setStatus(types.ConnectionState_Disconnected)
	conn.Reset()
}

func (n *Node) onIoStreamWritten(ioConn types.IoStreamConnection, status int32, payloadSize int) {
	if n == nil || ioConn == nil {
		return
	}

	var conn *Connection
	if existed, ok := ioConn.GetPrivateData().(*Connection); ok {
		conn = existed
	}

	var ep types.Endpoint
	if conn != nil {
		ep = conn.GetBinding()
	}

	connAddr := ""
	if addr := ioConn.GetAddress(); addr != nil {
		connAddr = addr.GetAddress()
	}

	if status != 0 {
		if conn != nil {
			conn.statistic.PushFailedTimes++
			conn.statistic.PushFailedSize += uint64(payloadSize)
		}

		n.LogError(ep, conn, 0, error_code.ErrorType(status),
			fmt.Sprintf("write data(%d bytes) to %s failed", payloadSize, connAddr))
		return
	}

	if conn != nil {
		conn.statistic.PushSuccessTimes++
		conn.statistic.PushSuccessSize += uint64(payloadSize)
	}

	n.LogDebug(ep, conn, nil,
		fmt.Sprintf("write data(%d bytes) to %s success", payloadSize, connAddr))
}

func (n *Node) ReloadCrypto(cryptoKeyExchangeType protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE,
	cryptoKeyRefreshInterval time.Duration,
	cryptoAllowAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE,
) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	n.configure.CryptoKeyRefreshInterval = cryptoKeyRefreshInterval
	if n.cryptoKeyExchangeType != cryptoKeyExchangeType {
		switch cryptoKeyExchangeType {
		case protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
			protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1,
			protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1,
			protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1:
			n.cryptoKeyExchangeType = cryptoKeyExchangeType
		default:
			cryptoKeyExchangeType = protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE
			n.cryptoKeyExchangeType = cryptoKeyExchangeType
		}
	}

	n.configure.CryptoKeyExchangeType = n.cryptoKeyExchangeType
	n.configure.CryptoAllowAlgorithms = cryptoAllowAlgorithms

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) ReloadCompression(compressionAllowAlgorithms []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE,
	compressionLevel protocol.ATBUS_COMPRESSION_LEVEL,
) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	n.configure.CompressionAllowAlgorithms = compressionAllowAlgorithms
	n.configure.CompressionLevel = compressionLevel
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) ReloadSelfEndpoint(scope string, namespaceName string, labels map[string]string, gateways []types.Gateway) {
	if n == nil {
		return
	}

	n.configure.Scope = scope
	n.configure.NamespaceName = namespaceName
	if len(labels) > 0 {
		copiedLabels := make(map[string]string, len(labels))
		for k, v := range labels {
			copiedLabels[k] = v
		}
		n.configure.NodeLabels = copiedLabels
	} else {
		n.configure.NodeLabels = nil
	}
	n.configure.Gateway = types.CloneGateways(gateways)

	if n.self != nil {
		n.self.Reload(n.configure.Scope, n.configure.NamespaceName, n.configure.NodeLabels, n.configure.Gateway)
	}
}

func (n *Node) Start() error_code.ErrorType {
	return n.StartWithConfigure(nil)
}

func (n *Node) StartWithConfigure(conf *types.StartConfigure) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.GetState() == types.NodeState_Created {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	if conf == nil || conf.TimerTimepoint.IsZero() {
		n.eventTimer.tick = time.Now()
	} else {
		n.eventTimer.tick = conf.TimerTimepoint
	}

	n.initHashCode()
	if n.self != nil {
		n.self.UpdateHashCode(n.getHashCode())
		n.self.Reload(n.configure.Scope, n.configure.NamespaceName, n.configure.NodeLabels, n.configure.Gateway)
	}

	// 连接上游节点
	if 0 != n.GetId() && n.configure.UpstreamAddress != "" {
		if n.upstream.node == nil {
			// 如果上游节点被激活了，那么上游节点操作时间必须更新到非0值，以启用这个功能
			if n.Connect(n.configure.UpstreamAddress) == error_code.EN_ATBUS_ERR_SUCCESS {
				// first_idle_timeout 是连接和握手的确认时限，不影响重试退避间隔
				n.eventTimer.upstreamOpTimepoint = n.eventTimer.tick.Add(n.configure.FirstIdleTimeout)
				n.setState(types.NodeState_ConnectingUpstream)
			} else {
				// 首次失败占用一级退避间隔
				n.eventTimer.upstreamOpRetryInterval = n.configure.RetryInterval
				n.eventTimer.upstreamOpTimepoint = n.eventTimer.tick.Add(n.eventTimer.upstreamOpRetryInterval)
				n.setState(types.NodeState_LostUpstream)
			}
		}
	} else {
		n.OnActived()
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) Reset() error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// Use flag guard to prevent reentrance
	fgd := &nodeFlagGuard{}
	if fgd.openFlag(n, types.NodeFlag_Resetting) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	defer fgd.closeFlag()

	// Reset upstream endpoint
	if upstream := n.upstream.node; upstream != nil {
		n.RemoveEndpoint(upstream)
		upstream.Reset()
	}

	// Remove all route endpoints
	n.removeRouteCollection(&n.nodeRoute)

	// Clear connecting list
	for {
		n.eventTimer.connectingListMu.Lock()
		key, pair, exists := n.eventTimer.connectingListByPtr.Front()
		if !exists {
			n.eventTimer.connectingListMu.Unlock()
			break
		}
		if pair.Value == nil {
			n.eventTimer.connectingListByPtr.Delete(key)
			n.eventTimer.connectingListMu.Unlock()
			continue
		}
		conn := pair.Value
		n.eventTimer.connectingListMu.Unlock()
		n.removeConnectionTimer(conn, true)
	}
	n.eventTimer.connectingListMu.Lock()
	clear(n.eventTimer.connectingListByChannel)
	n.eventTimer.connectingListMu.Unlock()

	// Reset self endpoint
	if n.self != nil {
		n.self.Reset()
	}

	// Clear ping list and GC lists
	n.flags |= uint16(types.NodeFlag_ResettingGc)
	n.eventTimer.pendingEndpointGcList.Init()
	n.eventTimer.pendingConnectionGcList.Init()
	n.eventTimer.upstreamOpTimepoint = time.Time{}
	n.eventTimer.upstreamOpRetryInterval = 0

	n.eventTimer.pingListMu.Lock()
	for {
		_, _, exists := n.eventTimer.pingList.PopFront()
		if !exists {
			break
		}
	}
	n.eventTimer.pingListMu.Unlock()

	if n.eventCancelFn != nil {
		n.eventCancelFn()
		n.eventCancelFn = nil
	}
	// Close the IO channel after releasing any producers waiting to enqueue.
	if n.ioStreamChannel != nil {
		n.ioStreamChannel.Close()
		n.ioStreamChannel = nil
	}
	n.ioStreamEvents = nil

	// Clear self-message queues
	n.selfDataMessages.Init()
	n.selfCommandMessages.Init()

	// Reset state
	n.setState(types.NodeState_Created)
	n.flags = 0

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) Proc(now time.Time) (int32, error_code.ErrorType) {
	if n == nil {
		return 0, error_code.EN_ATBUS_ERR_PARAMS
	}

	// Use flag guard to prevent reentrance
	fgd := &nodeFlagGuard{}
	if fgd.openFlag(n, types.NodeFlag_InProc) {
		return 0, error_code.EN_ATBUS_ERR_SUCCESS
	}
	defer fgd.closeFlag()

	// Update tick
	if now.After(n.eventTimer.tick) {
		n.eventTimer.tick = now
	}

	if n.state == types.NodeState_Created {
		return 0, error_code.EN_ATBUS_ERR_NOT_INITED
	}

	ret := int32(0)

	// Check shutdown flag
	if n.CheckFlag(types.NodeFlag_Shutdown) {
		ret = 1 + n.dispatchAllSelfMessages()
		n.Reset()
		return ret, error_code.EN_ATBUS_ERR_SUCCESS
	}

	ret += n.dispatchIoStreamEvents()
	if n.state == types.NodeState_Created {
		return ret, error_code.EN_ATBUS_ERR_SUCCESS
	}
	// Process connection timeouts
	n.processConnectingTimeout(now)

	// Process upstream node operations
	n.processUpstreamOperations(now)

	// Process ping timers
	n.processPingTimers(now)

	// Dispatch self messages before GC (matching C++ proc() order)
	ret += n.dispatchAllSelfMessages()

	// Execute GC after dispatch
	n.executeGC()

	return ret, error_code.EN_ATBUS_ERR_SUCCESS
}

// processConnectingTimeout handles timeout for connections that are still connecting
func (n *Node) processConnectingTimeout(now time.Time) {
	for {
		n.eventTimer.connectingListMu.Lock()
		key, pair, exists := n.eventTimer.connectingListByPtr.Front()
		if !exists {
			n.eventTimer.connectingListMu.Unlock()
			break
		}

		if pair.Value == nil {
			n.eventTimer.connectingListByPtr.Delete(key)
			n.eventTimer.connectingListMu.Unlock()
			continue
		}

		conn := pair.Value
		timeout := pair.Timeout
		n.eventTimer.connectingListMu.Unlock()

		if conn.IsConnected() {
			n.removeConnectionTimer(conn, false)
			continue
		}

		if !timeout.Before(now) {
			break
		}

		// Connection timeout
		if !conn.CheckFlag(types.ConnectionFlag_Temporary) {
			n.LogError(nil, conn, 0, error_code.EN_ATBUS_ERR_NODE_TIMEOUT,
				fmt.Sprintf("connection %s timeout", conn.GetAddress().GetAddress()))
		}
		n.removeConnectionTimer(conn, true)
	}
}

// processUpstreamOperations handles upstream node reconnection and ping
func (n *Node) processUpstreamOperations(now time.Time) {
	if n.GetId() == 0 || n.configure.UpstreamAddress == "" {
		return
	}

	if n.eventTimer.upstreamOpTimepoint.IsZero() || !n.eventTimer.upstreamOpTimepoint.Before(now) {
		return
	}

	// Get control connection
	var ctrlConn types.Connection
	if n.upstream.node != nil && n.self != nil {
		ctrlConn = n.self.GetCtrlConnection(n.upstream.node)
	}

	// Upstream node reconnect
	if ctrlConn == nil {
		res := n.Connect(n.configure.UpstreamAddress)
		if res != error_code.EN_ATBUS_ERR_SUCCESS {
			n.LogError(nil, nil, 0, res,
				fmt.Sprintf("reconnect upstream node %s failed", n.configure.UpstreamAddress))
			n.eventTimer.upstreamOpRetryInterval = nextUpstreamOpRetryInterval(
				&n.configure, n.eventTimer.upstreamOpRetryInterval, n.CheckFlag(types.NodeFlag_Actived))
			n.eventTimer.upstreamOpTimepoint = now.Add(n.eventTimer.upstreamOpRetryInterval)
		} else {
			// 下一次判定上游节点连接超时再重新连接
			// first_idle_timeout 是连接和握手的确认时限，不影响重试退避间隔
			n.eventTimer.upstreamOpTimepoint = now.Add(n.configure.FirstIdleTimeout)
			n.setState(types.NodeState_ConnectingUpstream)
		}
	} else {
		// Check if upstream is available
		if n.upstream.node != nil && !n.upstream.node.IsAvailable() {
			createdTime := n.upstream.node.GetStatisticCreatedTime()
			if createdTime.Add(n.configure.FirstIdleTimeout).Before(now) {
				n.AddEndpointGcList(n.upstream.node)
			}
		} else if n.upstream.node != nil {
			res := n.pingEndpoint(n.upstream.node)
			if res != error_code.EN_ATBUS_ERR_SUCCESS {
				n.LogError(nil, nil, 0, res,
					fmt.Sprintf("ping upstream node %s failed", n.configure.UpstreamAddress))
			}
		}

		// ping包不需要重试
		n.eventTimer.upstreamOpTimepoint = now.Add(n.configure.PingInterval)
	}
}

// processPingTimers handles ping timer operations
func (n *Node) processPingTimers(now time.Time) {
	for {
		n.eventTimer.pingListMu.Lock()
		_, pair, exists := n.eventTimer.pingList.Front()
		if !exists {
			n.eventTimer.pingListMu.Unlock()
			break
		}

		if pair.Value == nil {
			n.eventTimer.pingList.PopFront()
			n.eventTimer.pingListMu.Unlock()
			continue
		}

		if !pair.Timeout.Before(now) {
			n.eventTimer.pingListMu.Unlock()
			break
		}

		ep := pair.Value
		n.eventTimer.pingList.PopFront()
		n.eventTimer.pingListMu.Unlock()

		// Ping前检测有效性，超出最大首次空闲时间后还处于不可用状态（没有数据连接）需要回收
		if !ep.IsAvailable() && ep.GetStatisticCreatedTime().Add(n.configure.FirstIdleTimeout).Before(now) {
			n.AddEndpointGcList(ep)
			ep.AddPingTimer()
			continue
		}

		// 上游节点使用 processUpstreamOperations 定时 ping
		if ep != n.upstream.node {
			n.pingEndpoint(ep)
		}

		if n.GetEndpoint(ep.GetId()) == ep {
			ep.AddPingTimer()
		}
	}
}

// executeGC executes garbage collection for pending endpoints and connections.
// Matches C++ node::proc / node::poll GC sections:
//   - For each pending endpoint, if it is no longer available, reset and
//     remove it from the routing table (which also handles upstream state
//     transitions such as Running → LostUpstream).
//   - Connection GC simply clears the pending list (connections are cleaned
//     up when their owning endpoint is reset).
func (n *Node) executeGC() {
	// GC endpoints
	if !n.CheckFlag(types.NodeFlag_InGcEndpoints) {
		var fgGc nodeFlagGuard
		fgGc.openFlag(n, types.NodeFlag_InGcEndpoints)

		// Swap the current list into a local slice so that RemoveEndpoint /
		// Reset calls that re-add entries do not cause an infinite loop.
		checked := make([]types.Endpoint, 0, n.eventTimer.pendingEndpointGcList.Len())
		for n.eventTimer.pendingEndpointGcList.Len() > 0 {
			front := n.eventTimer.pendingEndpointGcList.Front()
			if front == nil {
				break
			}
			if ep, ok := front.Value.(types.Endpoint); ok && !lu.IsNil(ep) {
				checked = append(checked, ep)
			}
			n.eventTimer.pendingEndpointGcList.Remove(front)
		}

		for _, ep := range checked {
			if !ep.IsAvailable() {
				ep.Reset()
				n.LogInfo(ep, nil, "endpoint gc and remove")
				n.RemoveEndpoint(ep)
			}
		}

		// Clear any entries that were re-added during the loop above.
		n.eventTimer.pendingEndpointGcList.Init()

		fgGc.closeFlag()
	}

	// GC connections — just clear the pending list (C++ does the same).
	if !n.CheckFlag(types.NodeFlag_InGcConnections) {
		var fgGc nodeFlagGuard
		fgGc.openFlag(n, types.NodeFlag_InGcConnections)

		n.eventTimer.pendingConnectionGcList.Init()
		fgGc.closeFlag()
	}
}

// dispatchAllSelfMessages dispatches all pending self messages.
// Matches C++ node::dispatch_all_self_messages.
func (n *Node) dispatchAllSelfMessages() int32 {
	ret := int32(0)

	// recursive call will be ignored
	if n.CheckFlag(types.NodeFlag_RecvSelfMsg) || n.CheckFlag(types.NodeFlag_InCallback) {
		return ret
	}
	var fgd nodeFlagGuard
	fgd.openFlag(n, types.NodeFlag_RecvSelfMsg)
	defer fgd.closeFlag()

	loopLeft := int(n.configure.LoopTimes)
	if loopLeft <= 0 {
		loopLeft = 10240
	}

	// Process self data messages
	for loopLeft > 0 && n.selfDataMessages.Len() > 0 {
		loopLeft--
		front := n.selfDataMessages.Front()
		if front == nil {
			break
		}
		m := n.selfDataMessages.Remove(front).(*types.Message)

		head := m.GetHead()
		bodyType := m.GetBodyType()
		if head == nil || bodyType == types.MessageBodyTypeUnknown {
			n.LogError(n.GetSelfEndpoint(), nil, 0, error_code.EN_ATBUS_ERR_UNPACK,
				"head or body type unset")
			continue
		}

		if bodyType == types.MessageBodyTypeDataTransformReq {
			fwdData := m.GetBody().GetDataTransformReq()
			if fwdData == nil {
				continue
			}
			n.OnReceiveData(n.GetSelfEndpoint(), nil, m, fwdData.GetContent())
			ret++

			// fake response
			if fwdData.GetFlags()&uint32(protocol.ATBUS_FORWARD_DATA_FLAG_TYPE_FORWARD_DATA_FLAG_REQUIRE_RSP) != 0 {
				m.MutableHead().ResultCode = 0
				// Move data_transform_req to data_transform_rsp (same ForwardData object)
				reqWrapper, ok := m.MutableBody().MessageType.(*protocol.MessageBody_DataTransformReq)
				if ok && reqWrapper != nil {
					m.MutableBody().MessageType = &protocol.MessageBody_DataTransformRsp{
						DataTransformRsp: reqWrapper.DataTransformReq,
					}
				}
				n.OnReceiveForwardResponse(n.GetSelfEndpoint(), nil, m)
			}
		}
	}

	// Process self command messages
	for loopLeft > 0 && n.selfCommandMessages.Len() > 0 {
		loopLeft--
		front := n.selfCommandMessages.Front()
		if front == nil {
			break
		}
		m := n.selfCommandMessages.Remove(front).(*types.Message)

		head := m.GetHead()
		bodyType := m.GetBodyType()
		if head == nil || bodyType == types.MessageBodyTypeUnknown {
			n.LogError(n.GetSelfEndpoint(), nil, 0, error_code.EN_ATBUS_ERR_UNPACK,
				"head or body type unset")
			continue
		}

		n.onReceiveMessage(nil, m, 0, error_code.EN_ATBUS_ERR_SUCCESS)
		ret++
	}

	return ret
}

// pingEndpoint sends a ping to an endpoint
func (n *Node) pingEndpoint(ep *Endpoint) error_code.ErrorType {
	if n == nil || ep == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// 检测上一次ping是否返回
	if ep.GetStatisticUnfinishedPing() != 0 {
		if n.addEndpointFault(ep, nil) {
			return error_code.EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT
		}
	}

	if n.self == nil {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	// 临时节点不需要对外发送ping包
	if n.GetId() == 0 {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	// 允许跳过未连接或握手完成的endpoint
	conn := n.self.GetCtrlConnection(ep)
	if lu.IsNil(conn) {
		n.addEndpointFault(ep, nil)
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	// 在发送前记录未完成ping序列号，避免Go异步IO协程在SendPing返回前就收到pong并比对序列号
	pingSeq := n.AllocateMessageSequence()
	ep.SetStatisticUnfinishedPing(pingSeq)

	// 出错则增加错误计数
	ret := message_handle.SendPing(n, conn, pingSeq)
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		ep.SetStatisticUnfinishedPing(0)
		n.LogError(ep, conn, 0, ret, "send ping message failed")
		n.addEndpointFault(ep, nil)
		return ret
	}

	// no data channel is also an error
	if lu.IsNil(n.self.GetDataConnection(ep, false)) {
		n.addEndpointFault(ep, nil)
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) Poll() (int32, error_code.ErrorType) {
	if n == nil {
		return 0, error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.CheckFlag(types.NodeFlag_InCallback) {
		return 0, error_code.EN_ATBUS_ERR_SUCCESS
	}

	eventCount := int32(0)

	var fg nodeFlagGuard
	if fg.openFlag(n, types.NodeFlag_InPoll) {
		return 0, error_code.EN_ATBUS_ERR_SUCCESS
	}
	defer fg.closeFlag()

	if n.CheckFlag(types.NodeFlag_Shutdown) {
		eventCount += 1 + n.dispatchAllSelfMessages()
		n.Reset()
		return eventCount, error_code.EN_ATBUS_ERR_SUCCESS
	}

	eventCount += n.dispatchIoStreamEvents()

	// dispatcher all self messages
	eventCount += n.dispatchAllSelfMessages()

	// GC - endpoint/connections
	n.executeGC()

	// stop action happened in any callback
	if n.CheckFlag(types.NodeFlag_Shutdown) {
		n.Reset()
		eventCount += 1
	}

	return eventCount, error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) Listen(address string) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.GetState() == types.NodeState_Created {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	if n.self == nil {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	conn := CreateConnection(n, address, true)
	if conn == nil {
		return error_code.EN_ATBUS_ERR_MALLOC
	}

	ret := conn.Listen()
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		return ret
	}

	// 添加到self_里
	if false == n.self.AddConnection(conn, false) {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	// 记录监听地址
	n.self.AddListenAddress(conn.GetAddress().GetAddress())

	n.LogDebug(n.self, conn, nil, fmt.Sprintf("listen to %s success", address))

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) Connect(address string) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.GetState() == types.NodeState_Created {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	if existingConn := n.getPendingConnection(address); existingConn != nil && !existingConn.IsConnected() {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	conn := CreateConnection(n, address, false)
	if conn == nil {
		return error_code.EN_ATBUS_ERR_MALLOC
	}

	// golang实现暂未支持共享内存通道和内存通道

	ret := conn.Connect()
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		return ret
	}

	n.LogDebug(nil, conn, nil, fmt.Sprintf("connect to %s success", address))

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) ConnectWithEndpoint(address string, ep types.Endpoint) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.GetState() == types.NodeState_Created {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	if lu.IsNil(ep) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if existingConn := n.getPendingConnection(address); existingConn != nil &&
		!existingConn.IsConnected() && existingConn.GetBinding() == ep {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	conn := CreateConnection(n, address, false)
	if conn == nil {
		return error_code.EN_ATBUS_ERR_MALLOC
	}

	ret := conn.connect(ep)
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		return ret
	}

	n.LogDebug(nil, conn, nil, fmt.Sprintf("connect to %s and bind to a endpoint %d success", address, ep.GetId()))

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) Disconnect(id types.BusIdType) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.upstream.node != nil && n.upstream.node.GetId() == id {
		ep := n.upstream.node
		n.upstream.node = nil
		ep.ClearPingTimer()

		// event
		handle := n.eventHandleset.OnEndpointRemoved
		if handle != nil {
			fgd := &nodeFlagGuard{}
			fgd.openFlag(n, types.NodeFlag_InCallback)
			handle(n, ep, error_code.EN_ATBUS_ERR_SUCCESS)
			fgd.closeFlag()
		}

		ep.Reset()
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	ep := n.findRouteEndpoint(&n.nodeRoute, id)
	if ep == nil || ep.GetId() != id {
		return error_code.EN_ATBUS_ERR_ATNODE_NOT_FOUND
	}
	n.removeChild(&n.nodeRoute, id, ep, false)
	ep.Reset()

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) GetCryptoKeyExchangeType() protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE {
	if n == nil {
		return protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE
	}

	return n.cryptoKeyExchangeType
}

func (n *Node) SendData(tid types.BusIdType, t int32, data []byte) error_code.ErrorType {
	opts := types.CreateNodeSendDataOptions()
	return n.SendDataWithOptions(tid, t, data, opts)
}

func (n *Node) SendDataWithOptions(tid types.BusIdType, t int32, data []byte, options *types.NodeSendDataOptions) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.GetState() == types.NodeState_Created {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	if uint64(len(data)) > n.configure.MessageSize {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	m := types.NewMessage()
	head := m.MutableHead()
	body := m.MutableBody().MutableDataTransformReq()
	if head == nil || body == nil {
		n.LogError(nil, nil, 0, error_code.EN_ATBUS_ERR_MALLOC, "create message failed")
		return error_code.EN_ATBUS_ERR_MALLOC
	}

	selfId := n.GetId()
	flags := uint32(0)
	if options.GetFlag(types.NodeSendDataOptionFlag_RequiredResponse) {
		flags |= uint32(protocol.ATBUS_FORWARD_DATA_FLAG_TYPE_FORWARD_DATA_FLAG_REQUIRE_RSP)
	}

	head.Version = n.GetProtocolVersion()
	head.Type = t
	head.SourceBusId = uint64(selfId)
	if options.GetSequence() == 0 {
		head.Sequence = n.AllocateMessageSequence()
		if options != nil {
			options.SetSequence(head.Sequence)
		}
	} else {
		head.Sequence = options.GetSequence()
	}

	body.From = uint64(selfId)
	body.To = uint64(tid)
	body.AppendRouter(uint64(selfId))
	body.Content = data
	body.Flags = flags

	ret, _, _ := n.SendDataMessage(tid, m, options)
	return ret
}

func (n *Node) SendCustomCommand(tid types.BusIdType, args [][]byte) error_code.ErrorType {
	opts := types.CreateNodeSendDataOptions()
	return n.SendCustomCommandWithOptions(tid, args, opts)
}

func (n *Node) SendCustomCommandWithOptions(tid types.BusIdType, args [][]byte, options *types.NodeSendDataOptions) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.GetState() == types.NodeState_Created {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	sumLen := uint64(0)
	for _, b := range args {
		sumLen += uint64(len(b))
	}
	if sumLen > n.configure.MessageSize {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	m := types.NewMessage()
	head := m.MutableHead()
	body := m.MutableBody().MutableCustomCommandReq()
	if head == nil || body == nil {
		n.LogError(nil, nil, 0, error_code.EN_ATBUS_ERR_MALLOC, "create message failed")
		return error_code.EN_ATBUS_ERR_MALLOC
	}

	selfId := n.GetId()

	head.Version = n.GetProtocolVersion()
	head.SourceBusId = uint64(selfId)
	if options.GetSequence() == 0 {
		head.Sequence = n.AllocateMessageSequence()
		if options != nil {
			options.SetSequence(head.Sequence)
		}
	} else {
		head.Sequence = options.GetSequence()
	}

	body.From = uint64(selfId)
	body.Commands = make([]*protocol.CustomCommandArgv, 0, len(args))
	for _, b := range args {
		body.Commands = append(body.Commands, &protocol.CustomCommandArgv{
			Arg: b,
		})
	}

	message_handle.GenerateAccessDataForCustomCommand(body.MutableAccessKey(), selfId,
		rand.Uint64(), rand.Uint64(), n.configure.AccessTokens, body,
	)

	ret, _, _ := n.SendDataMessage(tid, m, options)
	return ret
}

func (n *Node) GetPeerChannel(tid types.BusIdType, fn func(from types.Endpoint, to types.Endpoint) types.Connection, options *types.NodeGetPeerOptions) (error_code.ErrorType, types.Endpoint, types.Connection, types.TopologyPeer) {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS, nil, nil, nil
	}

	if n.self == nil || n.GetState() == types.NodeState_Created {
		return error_code.EN_ATBUS_ERR_NOT_INITED, nil, nil, nil
	}

	if tid == n.GetId() {
		return error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, nil, nil, nil
	}

	ret, ep, conn := func() (error_code.ErrorType, types.Endpoint, types.Connection) {
		// 上游节点
		if n.upstream.node != nil && tid == n.upstream.node.GetId() {
			if isInGetPeerBlacklist(tid, options) {
				return error_code.EN_ATBUS_ERR_SUCCESS, nil, nil
			}
			connection := fn(n.self, n.upstream.node)
			return error_code.EN_ATBUS_ERR_SUCCESS, n.upstream.node, connection
		}

		// 直连节点
		target := n.findRouteEndpoint(&n.nodeRoute, tid)
		if target != nil {
			if isInGetPeerBlacklist(tid, options) {
				return error_code.EN_ATBUS_ERR_SUCCESS, nil, nil
			}
			connection := fn(n.self, target)
			return error_code.EN_ATBUS_ERR_SUCCESS, target, connection
		}

		relation, nextHopPeer := n.GetTopologyRelation(tid)
		// 子节点
		if relation == types.TopologyRelationType_ImmediateDownstream ||
			relation == types.TopologyRelationType_TransitiveDownstream {
			if nextHopPeer == nil {
				return error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, nil, nil
			}

			target = n.findRouteEndpoint(&n.nodeRoute, nextHopPeer.GetBusId())
			if target != nil {
				if isInGetPeerBlacklist(nextHopPeer.GetBusId(), options) {
					return error_code.EN_ATBUS_ERR_SUCCESS, nil, nil
				}

				connection := fn(n.self, target)
				return error_code.EN_ATBUS_ERR_SUCCESS, target, connection
			} else {
				return error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, nil, nil
			}
		}

		// 只有邻居节点,远方节点,间接上游都可以走上游节点。有个特殊情况是未注册拓扑关系视为远方节点，也允许走上游节点
		if relation == types.TopologyRelationType_Self {
			return error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, nil, nil
		}

		// 自动发现邻居路由
		//
		//     F1 ----主动连接---- F2
		//    /  \                /  \
		//  C11  C12            C21  C22
		// 当F1发往C21或C22时触发这种情况
		//
		if relation == types.TopologyRelationType_OtherUpstreamPeer && n.GetTopologyRegistry() != nil {
			findNearestNeightboutPeer := n.GetTopologyRegistry().GetPeer(tid)
			if findNearestNeightboutPeer != nil {
				findNearestNeightboutPeer = findNearestNeightboutPeer.GetUpstream()
			}
			for findNearestNeightboutPeer != nil {
				if isInGetPeerBlacklist(findNearestNeightboutPeer.GetBusId(), options) {
					findNearestNeightboutPeer = findNearestNeightboutPeer.GetUpstream()
					continue
				}

				target = n.findRouteEndpoint(&n.nodeRoute, findNearestNeightboutPeer.GetBusId())
				if target != nil {
					connection := fn(n.self, target)
					return error_code.EN_ATBUS_ERR_SUCCESS, target, connection
				}

				findNearestNeightboutPeer = findNearestNeightboutPeer.GetUpstream()
			}
		}

		// Fallback到上游转发
		//
		//     F1     |    F1 ----主动连接---- F2
		//    /  \    |   /  \                /  \
		//  C11  C12  | C11  C12            C21  C22
		// 当C11发往C12和C11/C12发往C21/C22时触发这种情况
		//
		if !options.GetFlag(types.NodeGetPeerOptionFlag_NoUpstream) && n.upstream.node != nil {
			if isInGetPeerBlacklist(n.upstream.node.GetId(), options) {
				return error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, nil, nil
			}

			connection := fn(n.self, n.upstream.node)
			return error_code.EN_ATBUS_ERR_SUCCESS, n.upstream.node, connection
		}

		return error_code.EN_ATBUS_ERR_SUCCESS, nil, nil
	}()

	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		var peer types.TopologyPeer = nil
		if ep != nil {
			peer = n.GetTopologyRegistry().GetPeer(ep.GetId())
		}
		return ret, ep, conn, peer
	}

	if ep == nil {
		return error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, ep, conn, nil
	}

	if conn == nil {
		return error_code.EN_ATBUS_ERR_ATNODE_NO_CONNECTION, ep, conn, n.GetTopologyRegistry().GetPeer(ep.GetId())
	}

	return ret, ep, conn, n.GetTopologyRegistry().GetPeer(ep.GetId())
}

func (n *Node) SetTopologyUpstream(tid types.BusIdType) {
	if n == nil {
		return
	}

	// 上游节点已经设置了，那拓扑关系也一定是一样的
	if n.upstream.node != nil && n.upstream.node.GetId() == tid {
		return
	}

	// 当前节点是临时节点的话，不用处理拓扑关系
	selfId := n.GetId()
	if selfId == 0 || n.topologyRegistry == nil {
		return
	}

	// 初始化先复制一份，后面再更新
	selfTopology := n.topologyRegistry.GetPeerInstance(selfId)

	// 上游未变化则直接跳过
	if selfTopology != nil {
		if tid == 0 && selfTopology.GetUpstream() == nil {
			return
		} else if tid != 0 && selfTopology.GetUpstream() != nil && selfTopology.GetUpstream().GetBusId() == tid {
			return
		}
	}

	// 不合法的上游关系则跳过
	if !n.topologyRegistry.UpdatePeer(selfId, tid, nil) {
		return
	}

	// 更新上游 endpoint
	if n.upstream.node != nil {
		n.insertChild(&n.nodeRoute, n.upstream.node, true)
	}

	if tid == 0 {
		n.upstream.node = nil
	} else {
		n.upstream.node = n.findRouteEndpoint(&n.nodeRoute, tid)
		if n.upstream.node != nil {
			n.removeChild(&n.nodeRoute, tid, nil, true)
		}
	}

	// event
	handle := n.eventHandleset.OnTopologyUpdateUpstream
	if handle != nil {
		var up *TopologyPeer = nil
		if tid != 0 {
			up = n.topologyRegistry.GetPeerInstance(tid)
		}
		if selfTopology == nil {
			selfTopology = n.topologyRegistry.GetPeerInstance(selfId)
		}

		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		handle(n, selfTopology, up, n.topologyData)
		fgd.closeFlag()
	}
}

func (n *Node) CreateEndpoint(tid types.BusIdType, hostName string, pid int) types.Endpoint {
	if n == nil {
		return nil
	}

	return CreateEndpoint(n, tid, int64(pid), hostName)
}

func (n *Node) GetEndpoint(tid types.BusIdType) types.Endpoint {
	if n == nil {
		return nil
	}

	if n.upstream.node != nil && n.upstream.node.GetId() == tid {
		return n.upstream.node
	}

	ep := n.findRouteEndpoint(&n.nodeRoute, tid)
	if ep != nil {
		return ep
	}

	return nil
}

func (n *Node) AddEndpoint(ep types.Endpoint) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.CheckFlag(types.NodeFlag_Resetting) {
		return error_code.EN_ATBUS_ERR_CLOSING
	}

	if n.self == nil {
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	if lu.IsNil(ep) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if ep.GetOwner() != n {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// 快速更新上游
	isUpdateUpstream := false
	if n.upstream.node != nil {
		if n.upstream.node == ep {
			return error_code.EN_ATBUS_ERR_SUCCESS
		}

		if n.upstream.node.GetId() == ep.GetId() {
			isUpdateUpstream = true
		}
	}

	if !isUpdateUpstream && 0 != n.GetId() && n.topologyRegistry != nil {
		// C++ uses get_peer(get_id()) — query the SELF peer to check if ep is our upstream.
		selfPeer := n.topologyRegistry.GetPeerInstance(n.GetId())
		if selfPeer != nil && selfPeer.GetUpstream() != nil {
			upStreamId := selfPeer.GetUpstream().GetBusId()
			if upStreamId == ep.GetId() {
				isUpdateUpstream = true
			}
		}
	}

	// 上游节点单独判定
	if isUpdateUpstream {
		n.upstream.node = ep.(*Endpoint)

		if !ep.GetFlag(types.EndpointFlag_HasPingTimer) {
			ep.AddPingTimer()
		}

		state := n.GetState()
		if (state == types.NodeState_LostUpstream || state == types.NodeState_ConnectingUpstream) &&
			n.CheckFlag(types.NodeFlag_UpstreamRegDone) {
			// 这里是自己先注册到上游节点，然后才完成上游节点对自己的注册流程，在message_handler::on_recv_node_reg_rsp里已经标记
			// EN_FT_UPSTREAM_REG_DONE 了
			n.OnActived()
		}

		// event
		handle := n.eventHandleset.OnEndpointAdded
		if handle != nil {
			fgd := &nodeFlagGuard{}
			fgd.openFlag(n, types.NodeFlag_InCallback)
			handle(n, ep, error_code.EN_ATBUS_ERR_SUCCESS)
			fgd.closeFlag()
		}

		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	if n.insertChild(&n.nodeRoute, ep.(*Endpoint), false) {
		ep.AddPingTimer()

		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	return error_code.EN_ATBUS_ERR_ATNODE_MASK_CONFLICT
}

func (n *Node) RemoveEndpoint(ep types.Endpoint) error_code.ErrorType {
	if n == nil || ep == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// 上游节点单独判定
	if n.upstream.node != nil && n.upstream.node.GetId() == ep.GetId() {
		if n.upstream.node != ep {
			return error_code.EN_ATBUS_ERR_ATNODE_NOT_FOUND
		}

		ep := n.upstream.node
		n.upstream.node = nil
		ep.ClearPingTimer()

		if n.GetState() == types.NodeState_Running || n.GetState() == types.NodeState_ConnectingUpstream {
			n.setState(types.NodeState_LostUpstream)

			// Immediately try to reconnect upstream when first lost upstream
			n.eventTimer.upstreamOpTimepoint = n.GetTimerTick()
		} else {
			n.eventTimer.upstreamOpRetryInterval = nextUpstreamOpRetryInterval(
				&n.configure, n.eventTimer.upstreamOpRetryInterval, n.CheckFlag(types.NodeFlag_Actived))
			n.eventTimer.upstreamOpTimepoint = n.GetTimerTick().Add(n.eventTimer.upstreamOpRetryInterval)
		}

		// event
		handle := n.eventHandleset.OnEndpointRemoved
		if handle != nil {
			fgd := &nodeFlagGuard{}
			fgd.openFlag(n, types.NodeFlag_InCallback)
			handle(n, ep, error_code.EN_ATBUS_ERR_SUCCESS)
			fgd.closeFlag()
		}

		// if not activited, shutdown
		if !n.CheckFlag(types.NodeFlag_Actived) {
			n.FatalShutdown(ep, nil, error_code.EN_ATBUS_ERR_ATNODE_MASK_CONFLICT,
				fmt.Errorf("upstream endpoint removed before node actived"))
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	if ep.GetId() == n.GetId() {
		return error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID
	}

	if n.removeChild(&n.nodeRoute, ep.GetId(), ep.(*Endpoint), false) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	} else {
		return error_code.EN_ATBUS_ERR_ATNODE_NOT_FOUND
	}
}

func (n *Node) RemoveEndpointByID(tid types.BusIdType) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if tid == n.GetId() {
		return error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID
	}

	ep := n.GetEndpoint(tid)
	if ep == nil {
		return error_code.EN_ATBUS_ERR_ATNODE_NOT_FOUND
	}

	return n.RemoveEndpoint(ep)
}

func (n *Node) IsEndpointAvailable(tid types.BusIdType) bool {
	if n == nil {
		return false
	}

	if !n.CheckFlag(types.NodeFlag_Actived) {
		return false
	}

	if n.self == nil {
		return false
	}

	ep := n.GetEndpoint(tid)
	if ep == nil {
		return false
	}

	return 0 == n.GetId() || nil != n.self.GetDataConnection(ep, false)
}

func (n *Node) CheckAccessHash(accessKey *protocol.AccessData, plainText string, conn types.Connection) bool {
	if n == nil {
		return false
	}

	if accessKey == nil {
		return false
	}

	var ep types.Endpoint = nil
	if conn != nil {
		ep = conn.GetBinding()
	}

	if accessKey.GetAlgorithm() != protocol.ATBUS_ACCESS_DATA_ALGORITHM_TYPE_ATBUS_ACCESS_DATA_ALGORITHM_HMAC_SHA256 {
		n.LogError(ep, conn, 0, error_code.EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT,
			fmt.Sprintf("access hash algorithm %v not supported", accessKey.GetAlgorithm()))
		return false
	}

	if len(n.configure.AccessTokens) == 0 && len(accessKey.GetSignature()) == 0 {
		return true
	}

	if len(n.configure.AccessTokens) == 0 {
		n.LogError(ep, conn, 0, error_code.EN_ATBUS_ERR_ACCESS_DENY,
			"access hash configuration is empty; we do not allow handshaking an endpoint with a signature.")
		return false
	}

	if len(accessKey.GetSignature()) == 0 {
		n.LogError(ep, conn, 0, error_code.EN_ATBUS_ERR_ACCESS_DENY,
			"access hash configuration is not empty; signature is required.")
		return false
	}

	// TODO(owent): 如果要阻挡重放攻击，需要验证和记录近期的nonce重复，也需要保证生成nonce的算法保证在一段时间内不重复
	for _, token := range n.configure.AccessTokens {
		realSignature := message_handle.CalculateAccessDataSignature(
			accessKey, token, plainText,
		)

		for _, expectSignature := range accessKey.GetSignature() {
			if len(expectSignature) != len(realSignature) {
				continue
			}

			if bytes.Equal(expectSignature, realSignature) {
				return true
			}
		}
	}

	return false
}

func (n *Node) GetHashCode() string {
	if n == nil {
		return ""
	}

	return n.hashCode
}

func (n *Node) GetIoStreamChannel() types.IoStreamChannel {
	if n == nil {
		return nil
	}

	return n.ioStreamChannel
}

func (n *Node) GetSelfEndpoint() types.Endpoint {
	ret := n.GetSelfEndpointInstance()
	if ret == nil {
		return nil
	}

	return ret
}

func (n *Node) GetSelfEndpointInstance() types.Endpoint {
	if n == nil {
		return nil
	}

	if n.self == nil {
		return nil
	}

	return n.self
}

func (n *Node) GetUpstreamEndpoint() types.Endpoint {
	ret := n.GetUpstreamEndpointInstance()
	if ret == nil {
		return nil
	}

	return ret
}

func (n *Node) GetUpstreamEndpointInstance() types.Endpoint {
	if n == nil {
		return nil
	}

	if n.upstream.node == nil {
		return nil
	}

	return n.upstream.node
}

func (n *Node) GetTopologyRelation(tid types.BusIdType) (types.TopologyRelationType, types.TopologyPeer) {
	if n == nil {
		return types.TopologyRelationType_Invalid, nil
	}

	if 0 == n.GetId() || n.self == nil {
		return types.TopologyRelationType_OtherUpstreamPeer, nil
	}

	if tid == n.GetId() {
		return types.TopologyRelationType_Self, n.GetTopologyRegistry().GetPeer(tid)
	}

	ret := types.TopologyRelationType_Invalid
	var nextHopPeer types.TopologyPeer
	register := n.GetTopologyRegistry()
	if register != nil {
		ret, nextHopPeer = register.GetRelation(n.GetId(), tid)
	}

	if ret == types.TopologyRelationType_Invalid {
		if n.GetUpstreamEndpointInstance() != nil && n.GetUpstreamEndpointInstance().GetId() == tid {
			ret = types.TopologyRelationType_ImmediateUpstream
		} else if nil != n.GetEndpoint(tid) {
			ret = types.TopologyRelationType_OtherUpstreamPeer
		}
	}

	return ret, nextHopPeer
}

func (n *Node) GetImmediateEndpointSet() types.EndpointCollectionType {
	if n == nil {
		return nil
	}

	return n.nodeRoute.endpointInterface
}

func (n *Node) SendDataMessage(tid types.BusIdType, message *types.Message, options *types.NodeSendDataOptions) (error_code.ErrorType, types.Endpoint, types.Connection) {
	return n.sendMessage(tid, message, func(from types.Endpoint, to types.Endpoint) types.Connection {
		return from.GetDataConnection(to, true)
	}, options)
}

func (n *Node) SendCtrlMessage(tid types.BusIdType, message *types.Message, options *types.NodeSendDataOptions) (error_code.ErrorType, types.Endpoint, types.Connection) {
	return n.sendMessage(tid, message, func(from types.Endpoint, to types.Endpoint) types.Connection {
		return from.GetCtrlConnection(to)
	}, options)
}

func (n *Node) sendMessage(tid types.BusIdType, message *types.Message, fn func(from types.Endpoint, to types.Endpoint) types.Connection, options *types.NodeSendDataOptions) (error_code.ErrorType, types.Endpoint, types.Connection) {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS, nil, nil
	}

	if n.state == types.NodeState_Created {
		return error_code.EN_ATBUS_ERR_NOT_INITED, nil, nil
	}

	// Self message handling
	if tid == n.GetId() {
		head := message.GetHead()
		bodyType := message.GetBodyType()

		if head == nil || bodyType == types.MessageBodyTypeUnknown {
			n.LogError(n.GetSelfEndpoint(), nil, 0, error_code.EN_ATBUS_ERR_UNPACK,
				"head or body type unset")
			return error_code.EN_ATBUS_ERR_UNPACK, nil, nil
		}

		if head.GetSequence() == 0 {
			message.MutableHead().Sequence = n.AllocateMessageSequence()
		}

		// Validate body type for self messages
		if bodyType != types.MessageBodyTypeDataTransformReq &&
			bodyType != types.MessageBodyTypeDataTransformRsp &&
			bodyType != types.MessageBodyTypeCustomCommandReq &&
			bodyType != types.MessageBodyTypeCustomCommandRsp {
			n.LogError(n.GetSelfEndpoint(), nil, 0, error_code.EN_ATBUS_ERR_ATNODE_INVALID_MSG,
				fmt.Sprintf("invalid body type %d", int(bodyType)))
			return error_code.EN_ATBUS_ERR_ATNODE_INVALID_MSG, nil, nil
		}

		// Queue self data/command messages
		if bodyType == types.MessageBodyTypeDataTransformReq || bodyType == types.MessageBodyTypeDataTransformRsp {
			n.selfDataMessages.PushBack(message)
		}
		if bodyType == types.MessageBodyTypeCustomCommandReq || bodyType == types.MessageBodyTypeCustomCommandRsp {
			n.selfCommandMessages.PushBack(message)
		}

		n.dispatchAllSelfMessages()
		return error_code.EN_ATBUS_ERR_SUCCESS, nil, nil
	}

	// Get peer connection
	getPeerOptions := &types.NodeGetPeerOptions{}
	if options != nil && options.GetFlag(types.NodeSendDataOptionFlag_NoUpstream) {
		getPeerOptions.SetFlag(types.NodeGetPeerOptionFlag_NoUpstream, true)
	}

	res, ep, conn, _ := n.GetPeerChannel(tid, fn, getPeerOptions)
	if res != error_code.EN_ATBUS_ERR_SUCCESS {
		return res, ep, conn
	}

	if lu.IsNil(conn) {
		return error_code.EN_ATBUS_ERR_ATNODE_NO_CONNECTION, ep, conn
	}

	head := message.GetHead()
	bodyType := message.GetBodyType()
	if head == nil || bodyType == types.MessageBodyTypeUnknown {
		n.LogError(ep, conn, 0, error_code.EN_ATBUS_ERR_UNPACK,
			"head or body type unset")
		return error_code.EN_ATBUS_ERR_UNPACK, ep, conn
	}

	if head.GetSequence() == 0 {
		message.MutableHead().Sequence = n.AllocateMessageSequence()
	}

	return message_handle.SendMessage(n, conn, message), ep, conn
}

func (n *Node) OnReceiveData(ep types.Endpoint, conn types.Connection, message *types.Message, data []byte) {
	if n == nil {
		return
	}

	if lu.IsNil(ep) && conn != nil {
		ep = conn.GetBinding()
	}

	handle := n.eventHandleset.OnForwardRequest
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		handle(n, ep, conn, message, data)
		fgd.closeFlag()
	}
}

func (n *Node) OnReceiveForwardResponse(ep types.Endpoint, conn types.Connection, message *types.Message) {
	if n == nil {
		return
	}

	handle := n.eventHandleset.OnForwardResponse
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		handle(n, ep, conn, message)
		fgd.closeFlag()
	}
}

func (n *Node) OnDisconnect(ep types.Endpoint, conn types.Connection) error_code.ErrorType {
	if n == nil || lu.IsNil(conn) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.state == types.NodeState_ConnectingUpstream && n.configure.UpstreamAddress != "" &&
		(n.configure.UpstreamAddress == conn.GetAddress().GetAddress() || n.configure.UpstreamAddress == conn.GetOriginAddress()) {
		n.setState(types.NodeState_LostUpstream)

		// set reconnect to upstream into retry interval
		n.eventTimer.upstreamOpRetryInterval = nextUpstreamOpRetryInterval(
			&n.configure, n.eventTimer.upstreamOpRetryInterval, n.CheckFlag(types.NodeFlag_Actived))
		n.eventTimer.upstreamOpTimepoint = n.GetTimerTick().Add(n.eventTimer.upstreamOpRetryInterval)

		// if not activited, shutdown
		if !n.CheckFlag(types.NodeFlag_Actived) {
			n.FatalShutdown(ep, conn, error_code.EN_ATBUS_ERR_ATNODE_MASK_CONFLICT,
				fmt.Errorf("upstream connection disconnected before node actived"))
		}
	}

	ret := error_code.EN_ATBUS_ERR_SUCCESS
	// event
	handle := n.eventHandleset.OnCloseConnection
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		ret = handle(n, ep, conn)
		fgd.closeFlag()
	}

	return ret
}

func (n *Node) OnNewConnection(conn types.Connection) error_code.ErrorType {
	if n == nil || lu.IsNil(conn) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	ret := error_code.EN_ATBUS_ERR_SUCCESS
	handle := n.eventHandleset.OnNewConnection
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		ret = handle(n, conn)
		fgd.closeFlag()
	}

	// 如果ID有效，且是IO流连接，则发送注册协议
	// ID为0则是临时节点，不需要注册
	if conn.CheckFlag(types.ConnectionFlag_RegFd) &&
		false == conn.CheckFlag(types.ConnectionFlag_ListenFd) &&
		conn.CheckFlag(types.ConnectionFlag_ClientMode) {
		ret = message_handle.SendRegister(types.MessageBodyTypeNodeRegisterReq, n, conn,
			error_code.EN_ATBUS_ERR_SUCCESS, n.AllocateMessageSequence())
		if ret != error_code.EN_ATBUS_ERR_SUCCESS {
			n.LogError(nil, conn, 0, ret,
				fmt.Sprintf("send node register message to %s failed", conn.GetAddress().GetAddress()))
			conn.Reset()
		}
	}

	return ret
}

func (n *Node) OnRegister(ep types.Endpoint, conn types.Connection, code error_code.ErrorType) {
	if n == nil {
		return
	}

	handle := n.eventHandleset.OnRegister
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		handle(n, ep, conn, code)
		fgd.closeFlag()
	}
}

func (n *Node) OnActived() {
	if n == nil {
		return
	}

	n.setState(types.NodeState_Running)

	// 每次激活成功（包括断线重连后的再次激活）都要重置重试退避间隔
	n.eventTimer.upstreamOpRetryInterval = 0

	if n.CheckFlag(types.NodeFlag_Actived) {
		return
	}
	n.flags |= uint16(types.NodeFlag_Actived)

	handle := n.eventHandleset.OnNodeUp
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		handle(n, error_code.EN_ATBUS_ERR_SUCCESS)
		fgd.closeFlag()
	}
}

func (n *Node) OnShutdown(code error_code.ErrorType) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if !n.CheckFlag(types.NodeFlag_Actived) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	handle := n.eventHandleset.OnNodeDown
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		code = handle(n, code)
		fgd.closeFlag()
	}

	return code
}

func (n *Node) OnUpstreamRegisterDone() {
	if n == nil {
		return
	}

	n.flags |= uint16(types.NodeFlag_UpstreamRegDone)
	pingTimepoint := n.GetTimerTick().Add(n.configure.PingInterval)
	if pingTimepoint.Before(n.eventTimer.upstreamOpTimepoint) {
		n.eventTimer.upstreamOpTimepoint = pingTimepoint
	}
}

func (n *Node) OnCustomCommandRequest(ep types.Endpoint, conn types.Connection, from types.BusIdType, argv [][]byte) (error_code.ErrorType, [][]byte) {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS, nil
	}

	var res [][]byte
	code := error_code.EN_ATBUS_ERR_SUCCESS
	handle := n.eventHandleset.OnCustomCommandRequest
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		code, res = handle(n, ep, conn, from, argv)
		fgd.closeFlag()
	}

	return code, res
}

func (n *Node) OnCustomCommandResponse(ep types.Endpoint, conn types.Connection, from types.BusIdType, argv [][]byte, sequence uint64) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	code := error_code.EN_ATBUS_ERR_SUCCESS
	handle := n.eventHandleset.OnCustomCommandResponse
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		code = handle(n, ep, conn, from, argv, sequence)
		fgd.closeFlag()
	}

	return code
}

func (n *Node) OnPing(ep types.Endpoint, message *types.Message, body *protocol.PingData) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	code := error_code.EN_ATBUS_ERR_SUCCESS
	handle := n.eventHandleset.OnEndpointPing
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		code = handle(n, ep, message, body)
		fgd.closeFlag()
	}

	return code
}

func (n *Node) OnPong(ep types.Endpoint, message *types.Message, body *protocol.PingData) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	code := error_code.EN_ATBUS_ERR_SUCCESS
	handle := n.eventHandleset.OnEndpointPong
	if handle != nil {
		fgd := &nodeFlagGuard{}
		fgd.openFlag(n, types.NodeFlag_InCallback)
		code = handle(n, ep, message, body)
		fgd.closeFlag()
	}

	return code
}

func (n *Node) addEndpointFault(ep types.Endpoint, conn types.Connection) bool {
	if n == nil || lu.IsNil(ep) {
		return false
	}

	faultCount := ep.AddStatisticFault()
	if faultCount >= uint64(n.configure.FaultTolerant) {
		n.LogError(ep, conn, 0, error_code.EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT,
			fmt.Sprintf("endpoint %d fault count %d exceeds threshold", ep.GetId(), faultCount))
		n.RemoveEndpoint(ep)
		return true
	}

	return false
}

func (n *Node) addConnectionFault(conn types.Connection) bool {
	if n == nil || lu.IsNil(conn) {
		return false
	}

	faultCount := conn.AddStatisticFault()
	if faultCount >= uint64(n.configure.FaultTolerant) {
		n.LogError(conn.GetBinding(), conn, 0, error_code.EN_ATBUS_ERR_ATNODE_FAULT_TOLERANT,
			fmt.Sprintf("connection fault count %d reaches threshold", faultCount))
		conn.Reset()
		return true
	}

	return false
}

// onReceiveMessage handles an incoming message, dispatching it and managing fault counters.
// Matches C++ node::on_receive_message.
func (n *Node) onReceiveMessage(conn types.Connection, m *types.Message, status int, errcode error_code.ErrorType) {
	if int32(status) < 0 || int32(errcode) < 0 {
		connAddr := ""
		if !lu.IsNil(conn) && conn.GetAddress() != nil {
			connAddr = conn.GetAddress().GetAddress()
		}
		n.LogError(nil, conn, status, errcode, fmt.Sprintf("receive message from %s failed", connAddr))
		if !lu.IsNil(conn) {
			n.addEndpointFault(conn.GetBinding(), conn)
			n.addConnectionFault(conn)
		}
		return
	}

	res := message_handle.DispatchMessage(n, conn, m, status, errcode)
	if int32(res) < 0 {
		if !lu.IsNil(conn) {
			n.addEndpointFault(conn.GetBinding(), conn)
			n.addConnectionFault(conn)
		}
		return
	}

	if !lu.IsNil(conn) {
		ep := conn.GetBinding()
		if !lu.IsNil(ep) {
			ep.ClearStatisticFault()
		}
		conn.ClearStatisticFault()
	}
}

func (n *Node) DispatchAllSelfMessages() int32 {
	return n.dispatchAllSelfMessages()
}

func (n *Node) GetContext() context.Context {
	if n == nil {
		return nil
	}

	return n.configure.EventLoopContext
}

func (n *Node) Shutdown(reason error_code.ErrorType) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.CheckFlag(types.NodeFlag_Shutdown) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	n.flags |= uint16(types.NodeFlag_Shutdown)

	return n.OnShutdown(reason)
}

func (n *Node) FatalShutdown(ep types.Endpoint, conn types.Connection, code error_code.ErrorType, err error) error_code.ErrorType {
	if n == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if n.CheckFlag(types.NodeFlag_Shutdown) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	n.Shutdown(code)

	n.LogError(ep, conn, 0, code, fmt.Sprintf("fatal shutdown error %v", err))
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (n *Node) SetLogger(logger *utils_log.Logger) {
	if n == nil {
		return
	}

	n.logger = logger
}

func (n *Node) GetLogger() *utils_log.Logger {
	if n == nil {
		return nil
	}

	return n.logger
}

func (n *Node) IsDebugMessageVerboseEnabled() bool {
	if n == nil {
		return false
	}

	return n.loggerEnableDebugMessageVerbose
}

func (n *Node) EnableDebugMessageVerbose() {
	if n == nil {
		return
	}

	n.loggerEnableDebugMessageVerbose = true
}

func (n *Node) DisableDebugMessageVerbose() {
	if n == nil {
		return
	}

	n.loggerEnableDebugMessageVerbose = false
}

func (n *Node) AddEndpointGcList(ep types.Endpoint) {
	if n == nil {
		return
	}

	// 重置过程中不需要再加进来了，反正等会也会移除
	// 这个代码加不加一样，只不过会少一些废操作
	if n.CheckFlag(types.NodeFlag_ResettingGc) || n.CheckFlag(types.NodeFlag_InGcEndpoints) {
		return
	}

	if !lu.IsNil(ep) {
		n.eventTimer.pendingEndpointGcList.PushBack(ep)
	}
}

// addPingTimer adds an endpoint to the ping timer list
func (n *Node) addPingTimer(ep *Endpoint, nextPingTime time.Time) bool {
	if n == nil || ep == nil {
		return false
	}

	// Self node doesn't need ping
	if ep.GetId() == n.GetId() {
		return false
	}

	if n.configure.PingInterval <= 0 {
		return false
	}

	if n.CheckFlag(types.NodeFlag_ResettingGc) {
		return false
	}

	n.eventTimer.pingListMu.Lock()
	n.eventTimer.pingList.Put(ep, types.TimerDescPair[*Endpoint]{
		Timeout: nextPingTime,
		Value:   ep,
	})
	n.eventTimer.pingListMu.Unlock()
	return true
}

// removePingTimer removes an endpoint from the ping timer list
func (n *Node) removePingTimer(ep *Endpoint) {
	if n == nil || ep == nil {
		return
	}

	n.eventTimer.pingListMu.Lock()
	n.eventTimer.pingList.Delete(ep)
	n.eventTimer.pingListMu.Unlock()
}

func (n *Node) AddConnectionGcList(conn types.Connection) {
	if n == nil {
		return
	}

	if n.CheckFlag(types.NodeFlag_ResettingGc) || n.CheckFlag(types.NodeFlag_InGcConnections) {
		return
	}

	if !lu.IsNil(conn) {
		n.eventTimer.pendingConnectionGcList.PushBack(conn)
	}
}

func (n *Node) SetEventHandleOnForwardRequest(handle types.NodeOnForwardRequestFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnForwardRequest = handle
}

func (n *Node) GetEventHandleOnForwardRequest() types.NodeOnForwardRequestFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnForwardRequest
}

func (n *Node) SetEventHandleOnForwardResponse(handle types.NodeOnForwardResponseFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnForwardResponse = handle
}

func (n *Node) GetEventHandleOnForwardResponse() types.NodeOnForwardResponseFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnForwardResponse
}

func (n *Node) SetEventHandleOnRegister(handle types.NodeOnRegisterFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnRegister = handle
}

func (n *Node) GetEventHandleOnRegister() types.NodeOnRegisterFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnRegister
}

func (n *Node) SetEventHandleOnShutdown(handle types.NodeOnNodeDownFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnNodeDown = handle
}

func (n *Node) GetEventHandleOnShutdown() types.NodeOnNodeDownFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnNodeDown
}

func (n *Node) SetEventHandleOnAvailable(handle types.NodeOnNodeUpFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnNodeUp = handle
}

func (n *Node) GetEventHandleOnAvailable() types.NodeOnNodeUpFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnNodeUp
}

func (n *Node) SetEventHandleOnInvalidConnection(handle types.NodeOnInvalidConnectionFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnInvalidConnection = handle
}

func (n *Node) GetEventHandleOnInvalidConnection() types.NodeOnInvalidConnectionFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnInvalidConnection
}

func (n *Node) SetEventHandleOnNewConnection(handle types.NodeOnNewConnectionFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnNewConnection = handle
}

func (n *Node) GetEventHandleOnNewConnection() types.NodeOnNewConnectionFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnNewConnection
}

func (n *Node) SetEventHandleOnCloseConnection(handle types.NodeOnCloseConnectionFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnCloseConnection = handle
}

func (n *Node) GetEventHandleOnCloseConnection() types.NodeOnCloseConnectionFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnCloseConnection
}

func (n *Node) SetEventHandleOnCustomCommandRequest(handle types.NodeOnCustomCommandRequestFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnCustomCommandRequest = handle
}

func (n *Node) GetEventHandleOnCustomCommandRequest() types.NodeOnCustomCommandRequestFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnCustomCommandRequest
}

func (n *Node) SetEventHandleOnCustomCommandResponse(handle types.NodeOnCustomCommandResponseFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnCustomCommandResponse = handle
}

func (n *Node) GetEventHandleOnCustomCommandResponse() types.NodeOnCustomCommandResponseFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnCustomCommandResponse
}

func (n *Node) SetEventHandleOnAddEndpoint(handle types.NodeOnEndpointEventFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnEndpointAdded = handle
}

func (n *Node) GetEventHandleOnAddEndpoint() types.NodeOnEndpointEventFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnEndpointAdded
}

func (n *Node) SetEventHandleOnRemoveEndpoint(handle types.NodeOnEndpointEventFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnEndpointRemoved = handle
}

func (n *Node) GetEventHandleOnRemoveEndpoint() types.NodeOnEndpointEventFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnEndpointRemoved
}

func (n *Node) SetEventHandleOnPingEndpoint(handle types.NodeOnPingPongEndpointFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnEndpointPing = handle
}

func (n *Node) GetEventHandleOnPingEndpoint() types.NodeOnPingPongEndpointFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnEndpointPing
}

func (n *Node) SetEventHandleOnPongEndpoint(handle types.NodeOnPingPongEndpointFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnEndpointPong = handle
}

func (n *Node) GetEventHandleOnPongEndpoint() types.NodeOnPingPongEndpointFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnEndpointPong
}

func (n *Node) SetEventHandleOnTopologyUpdateUpstream(handle types.NodeOnTopologyUpdateUpstreamFunc) {
	if n == nil {
		return
	}

	n.eventHandleset.OnTopologyUpdateUpstream = handle
}

func (n *Node) GetEventHandleOnTopologyUpdateUpstream() types.NodeOnTopologyUpdateUpstreamFunc {
	if n == nil {
		return nil
	}

	return n.eventHandleset.OnTopologyUpdateUpstream
}

func (n *Node) GetIoStreamConfigure() *types.IoStreamConfigure {
	if n == nil {
		return nil
	}

	if n.ioStreamConfigure == nil {
		n.ioStreamConfigure = &types.IoStreamConfigure{
			Keepalive:           60 * time.Second,
			NoBlock:             true,
			NoDelay:             true,
			SendBufferStatic:    n.configure.SendBufferNumber,
			ReceiveBufferStatic: 0, // 默认动态缓冲区

			SendBufferMaxSize:   n.configure.SendBufferSize,
			SendBufferLimitSize: n.configure.MessageSize + types.ATBUS_MACRO_MAX_FRAME_HEADER + uint64(buffer.PaddingSize(20)),

			ReceiveBufferMaxSize:   n.configure.MessageSize + n.configure.MessageSize + types.ATBUS_MACRO_MAX_FRAME_HEADER + 1024,
			ReceiveBufferLimitSize: n.configure.MessageSize + types.ATBUS_MACRO_MAX_FRAME_HEADER,

			Backlog: n.configure.BackLog,

			ConfirmTimeout:                   n.configure.FirstIdleTimeout,
			MaxReadNetEgainCount:             256,
			MaxReadCheckBlockSizeFailedCount: 10,
			MaxReadCheckHashFailedCount:      10,

			OverwriteListenPath: n.configure.OverwriteListenPath,
		}
	}

	return n.ioStreamConfigure
}

func (n *Node) GetId() types.BusIdType {
	if n == nil {
		return 0
	}

	if n.self == nil {
		return 0
	}

	return n.self.GetId()
}

func (n *Node) GetConfigure() *types.NodeConfigure {
	if n == nil {
		return nil
	}

	return &n.configure
}

func (n *Node) CheckFlag(f types.NodeFlag) bool {
	if n == nil {
		return false
	}

	return n.flags&uint16(f) != 0
}

func (n *Node) GetState() types.NodeState {
	if n == nil {
		return types.NodeState_Created
	}

	return n.state
}

func (n *Node) setState(s types.NodeState) {
	if n == nil {
		return
	}

	n.state = s
}

func (n *Node) GetTimerTick() time.Time {
	if n == nil {
		return time.Time{}
	}

	return n.eventTimer.tick
}

func (n *Node) GetTopologyRegistry() types.TopologyRegistry {
	if n == nil {
		return nil
	}

	if n.topologyRegistry == nil {
		return nil
	}

	return n.topologyRegistry
}

func (n *Node) GetPid() int {
	return os.Getpid()
}

var (
	hostName   string
	hostNameMu sync.Mutex
)

func activeInterfaceMACs() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	addresses := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&(net.FlagUp|net.FlagLoopback) != net.FlagUp || len(iface.HardwareAddr) != 6 {
			continue
		}
		unicast, err := iface.Addrs()
		if err != nil || len(unicast) == 0 {
			continue
		}
		mac := bytes.TrimLeft(iface.HardwareAddr, "\x00")
		if len(mac) > 0 {
			addresses = append(addresses, hex.EncodeToString(mac))
		}
	}
	slices.Sort(addresses)
	return slices.Compact(addresses)
}

func buildHostname() string {
	if macs := activeInterfaceMACs(); len(macs) > 0 {
		return sha256String(strings.Join(macs, ":")).hex()
	}
	host, _ := os.Hostname()
	if host != "" {
		return sha256String(host).hex()
	}
	return ""
}

func (n *Node) GetHostname() string {
	hostNameMu.Lock()
	defer hostNameMu.Unlock()
	if hostName == "" {
		hostName = buildHostname()
	}
	return hostName
}

func (n *Node) SetHostname(hostname string, force bool) bool {
	hostNameMu.Lock()
	defer hostNameMu.Unlock()
	if hostName == "" || force {
		hostName = hostname
		return true
	}
	return false
}

func (n *Node) GetSelfHostname() string {
	if n != nil && n.selfHostname != "" {
		return n.selfHostname
	}

	return n.GetHostname()
}

func (n *Node) SetSelfHostname(hostname string) {
	if n == nil {
		return
	}

	n.selfHostname = hostname
}

func (n *Node) GetSelfPid() int32 {
	if n != nil && n.selfPid != 0 {
		return n.selfPid
	}

	return int32(n.GetPid())
}

func (n *Node) SetSelfPid(pid int32) {
	if n == nil {
		return
	}

	n.selfPid = pid
}

func (n *Node) GetProtocolVersion() int32 {
	if n == nil {
		return int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_VERSION)
	}

	return n.configure.ProtocolVersion
}

func (n *Node) GetProtocolMinimalVersion() int32 {
	if n == nil {
		return int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_MINIMAL_VERSION)
	}

	return n.configure.ProtocolMinimalVersion
}

func (n *Node) GetListenList() []types.ChannelAddress {
	if nil == n {
		return nil
	}

	if nil == n.self {
		return nil
	}

	return n.self.GetListenAddress()
}

func (n *Node) GetGatewayAddress() []types.Gateway {
	if n == nil {
		return nil
	}

	return types.CloneGateways(n.configure.Gateway)
}

func (n *Node) DumpListenToChannelData(listenAddress string, ch *protocol.ChannelData) {
	if n == nil || ch == nil {
		return
	}

	ch.Address = listenAddress
	// 默认监听地址要隔离scope和namespace
	ch.MatchScope = n.configure.Scope
	if n.configure.NamespaceName != "" {
		ch.MatchNamespaces = []string{n.configure.NamespaceName}
	} else {
		ch.MatchNamespaces = nil
	}
}

func (n *Node) GetConnectionTimerSize() int {
	if n == nil {
		return 0
	}

	n.eventTimer.connectingListMu.Lock()
	defer n.eventTimer.connectingListMu.Unlock()
	return n.eventTimer.connectingListByPtr.Len()
}

func (n *Node) AddStatisticDispatchTimes() {
	if nil == n {
		return
	}

	n.stat.DispatchTimes++
}

func (n *Node) AllocateMessageSequence() uint64 {
	if n == nil {
		return 0
	}

	ret := uint64(0)
	for ret == 0 {
		ret = n.messageSequenceAllocator.Add(1)
	}

	return ret
}

func (n *Node) findRouteEndpoint(col *endpointCollection, tid types.BusIdType) *Endpoint {
	if col == nil {
		return nil
	}

	if ep, ok := col.endpointInstance[tid]; ok {
		return ep
	}

	return nil
}

func (n *Node) insertChild(col *endpointCollection, ep *Endpoint, ignoreEvent bool) bool {
	if col == nil || ep == nil {
		return false
	}

	oldEp, exists := col.endpointInstance[ep.GetId()]
	if exists && ep == oldEp {
		return true
	}
	col.endpointInstance[ep.GetId()] = ep
	col.endpointInterface[ep.GetId()] = ep

	if !ignoreEvent && n != nil {
		handle := n.GetEventHandleOnAddEndpoint()
		if handle != nil {
			var fg nodeFlagGuard
			fg.openFlag(n, types.NodeFlag_InCallback)
			handle(n, ep, error_code.EN_ATBUS_ERR_SUCCESS)
			fg.closeFlag()
		}
	}

	return true
}

func (n *Node) removeChild(col *endpointCollection, tid types.BusIdType, expected *Endpoint, ignoreEvent bool) bool {
	if col == nil {
		return false
	}

	ep, exists := col.endpointInstance[tid]
	if !exists {
		return false
	}

	if expected != nil && ep != expected {
		return false
	}

	delete(col.endpointInstance, tid)
	delete(col.endpointInterface, tid)
	ep.ClearPingTimer()

	if !ignoreEvent && n != nil {
		handle := n.GetEventHandleOnRemoveEndpoint()
		if handle != nil {
			var fg nodeFlagGuard
			fg.openFlag(n, types.NodeFlag_InCallback)
			handle(n, ep, error_code.EN_ATBUS_ERR_SUCCESS)
			fg.closeFlag()
		}
	}

	return true
}

// removeRouteCollection removes all endpoints from the collection
func (n *Node) removeRouteCollection(col *endpointCollection) {
	if col == nil {
		return
	}

	// Callbacks can modify the collection while endpoints are detached.
	endpoints := make([]*Endpoint, 0, len(col.endpointInstance))
	for _, ep := range col.endpointInstance {
		endpoints = append(endpoints, ep)
	}

	for _, ep := range endpoints {
		n.removeChild(col, ep.GetId(), ep, false)
		ep.Reset()
	}
}

func (n *Node) initHashCode() {
	if n == nil {
		return
	}

	h := sha256.New()

	// hash all interface mac addresses (sorted, unique)
	for _, mac := range activeInterfaceMACs() {
		_, _ = h.Write([]byte(mac))
	}

	// hash hostname
	_, _ = h.Write([]byte(n.GetSelfHostname()))

	// hash pid (int32 to match C++ int size)
	pid := n.GetSelfPid()
	var pidBuf [4]byte
	binary.LittleEndian.PutUint32(pidBuf[:], uint32(pid))
	_, _ = h.Write(pidBuf[:])

	// hash address of node
	ptr := uintptr(unsafe.Pointer(n))
	if unsafe.Sizeof(ptr) == 4 {
		var ptrBuf [4]byte
		binary.LittleEndian.PutUint32(ptrBuf[:], uint32(ptr))
		_, _ = h.Write(ptrBuf[:])
	} else {
		var ptrBuf [8]byte
		binary.LittleEndian.PutUint64(ptrBuf[:], uint64(ptr))
		_, _ = h.Write(ptrBuf[:])
	}

	// hash id
	id := uint64(n.GetId())
	var idBuf [8]byte
	binary.LittleEndian.PutUint64(idBuf[:], id)
	_, _ = h.Write(idBuf[:])

	n.hashCode = hex.EncodeToString(h.Sum(nil))
}

func (n *Node) getHashCode() string {
	if n == nil {
		return ""
	}

	return n.hashCode
}

func (n *Node) addConnectionTimer(conn *Connection) bool {
	if n == nil || conn == nil {
		return false
	}

	if n.state == types.NodeState_Created {
		return false
	}

	if !conn.IsConnected() {
		n.eventTimer.connectingListMu.Lock()
		if n.eventTimer.connectingListByChannel == nil {
			n.eventTimer.connectingListByChannel = make(map[string]*Connection)
		}
		if _, exists := n.eventTimer.connectingListByPtr.Get(conn, false); exists {
			n.eventTimer.connectingListMu.Unlock()
			return false
		}

		timeout := n.eventTimer.tick.Add(n.configure.FirstIdleTimeout)
		n.eventTimer.connectingListByPtr.Put(conn, types.TimerDescPair[*Connection]{
			Timeout: timeout,
			Value:   conn,
		})
		if addr := conn.GetAddress(); addr != nil {
			n.eventTimer.connectingListByChannel[addr.GetAddress()] = conn
		}
		n.eventTimer.connectingListMu.Unlock()
	}

	return true
}

func (n *Node) getPendingConnection(address string) *Connection {
	normalized, ok := makeConnectionAddress(address, false)
	if !ok {
		return nil
	}
	n.eventTimer.connectingListMu.Lock()
	defer n.eventTimer.connectingListMu.Unlock()
	return n.eventTimer.connectingListByChannel[normalized.GetAddress()]
}

func (n *Node) removeConnectionTimer(conn *Connection, reset bool) bool {
	if n == nil || conn == nil {
		return false
	}

	var connWithLifetime *Connection
	n.eventTimer.connectingListMu.Lock()
	if pair, exists := n.eventTimer.connectingListByPtr.Get(conn, false); exists {
		connWithLifetime = pair.Value
		n.eventTimer.connectingListByPtr.Delete(conn)
	}
	if addr := conn.GetAddress(); addr != nil && n.eventTimer.connectingListByChannel != nil {
		if chConn, exists := n.eventTimer.connectingListByChannel[addr.GetAddress()]; exists && chConn == conn {
			if connWithLifetime == nil {
				connWithLifetime = chConn
			}
			delete(n.eventTimer.connectingListByChannel, addr.GetAddress())
		}
	}
	n.eventTimer.connectingListMu.Unlock()

	// connection::reset 可能重入回调进来，此时已删除则直接返回即可
	if connWithLifetime == nil {
		return false
	}

	peerClosedTemporary := connWithLifetime.CheckFlag(types.ConnectionFlag_Temporary) &&
		connWithLifetime.CheckFlag(types.ConnectionFlag_PeerClosed)
	if reset {
		connWithLifetime.Reset()
	}

	if n.eventHandleset.OnInvalidConnection != nil && !connWithLifetime.IsConnected() && !peerClosedTemporary {
		var fg nodeFlagGuard
		fg.openFlag(n, types.NodeFlag_InCallback)
		n.eventHandleset.OnInvalidConnection(n, connWithLifetime, error_code.EN_ATBUS_ERR_NODE_TIMEOUT)
		fg.closeFlag()
	}

	return true
}

func (n *Node) LogDebug(ep types.Endpoint, conn types.Connection, m *types.Message, msg string, args ...any) {
	logger := n.GetLogger()
	if logger == nil {
		return
	}

	if !logger.Enabled(n.GetContext(), slog.LevelDebug) {
		return
	}

	nodeId := uint64(n.GetId())
	epId := uint64(0)
	if !lu.IsNil(ep) {
		epId = uint64(ep.GetId())
	}
	connAddr := ""
	if !lu.IsNil(conn) {
		connAddr = conn.GetAddress().GetAddress()
	}

	args = append(args,
		slog.Uint64("node", nodeId),
		slog.Uint64("endpoint", epId),
		slog.String("connection", connAddr),
	)

	if m != nil && n.IsDebugMessageVerboseEnabled() {
		args = append(args,
			slog.String("head", m.GetHead().String()),
			slog.String("body", m.GetBody().String()),
		)
	}

	logger.LogInner(time.Now(), utils_log.GetCaller(1), n.GetContext(), slog.LevelDebug, msg, args)
}

func (n *Node) LogInfo(ep types.Endpoint, conn types.Connection, msg string, args ...any) {
	logger := n.GetLogger()
	if logger == nil {
		return
	}

	if !logger.Enabled(n.GetContext(), slog.LevelInfo) {
		return
	}

	nodeId := uint64(n.GetId())
	epId := uint64(0)
	if !lu.IsNil(ep) {
		epId = uint64(ep.GetId())
	}
	connAddr := ""
	if !lu.IsNil(conn) {
		connAddr = conn.GetAddress().GetAddress()
	}

	args = append(args,
		slog.Uint64("node", nodeId),
		slog.Uint64("endpoint", epId),
		slog.String("connection", connAddr),
	)

	logger.LogInner(time.Now(), utils_log.GetCaller(1), n.GetContext(), slog.LevelInfo, msg, args)
}

func (n *Node) LogError(ep types.Endpoint, conn types.Connection, status int, errcode error_code.ErrorType, msg string, args ...any) {
	logger := n.GetLogger()
	if logger == nil {
		return
	}

	if !logger.Enabled(n.GetContext(), slog.LevelError) {
		return
	}

	nodeId := uint64(n.GetId())
	epId := uint64(0)
	if ep != nil {
		epId = uint64(ep.GetId())
	}
	connAddr := ""
	if !lu.IsNil(conn) {
		connAddr = conn.GetAddress().GetAddress()
	}

	args = append(args,
		slog.Uint64("node", nodeId),
		slog.Uint64("endpoint", epId),
		slog.String("connection", connAddr),
		slog.Int("status", status),
		slog.String("error_code", errcode.String()),
	)

	logger.LogInner(time.Now(), utils_log.GetCaller(1), n.GetContext(), slog.LevelError, msg, args)
}
