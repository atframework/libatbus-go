package libatbus_impl

import (
	"sort"
	"strings"
	"sync"
	"time"

	channel_utility "github.com/atframework/libatbus-go/channel/utility"
	types "github.com/atframework/libatbus-go/types"
)

var _ types.Endpoint = (*Endpoint)(nil)

type endpointStatistic struct {
	FaultCount     uint64
	UnfinishedPing uint64

	PingDelay    time.Duration
	LastPongTime time.Time
	CreatedTime  time.Time
}

type Endpoint struct {
	id       types.BusIdType
	pid      int32
	hostname string
	hashCode string
	owner    *Node

	flags uint32

	// mu protects ctrlConn, dataConn, and flags against concurrent access
	// from IO goroutines (disconnect callbacks) and the main loop.
	mu       sync.Mutex
	ctrlConn types.Connection
	dataConn []types.Connection

	listenAddress []types.ChannelAddress

	supportedSchemas map[string]struct{}

	stat endpointStatistic
}

func CreateEndpoint(owner *Node, id types.BusIdType, pid int64, hostname string) *Endpoint {
	if owner == nil {
		return nil
	}

	ep := &Endpoint{
		id:               id,
		pid:              int32(pid),
		hostname:         hostname,
		owner:            owner,
		flags:            0,
		dataConn:         make([]types.Connection, 0),
		listenAddress:    make([]types.ChannelAddress, 0),
		supportedSchemas: make(map[string]struct{}),
	}

	ep.stat.CreatedTime = time.Now()

	return ep
}

func (e *Endpoint) GetOwner() types.Node {
	if e == nil {
		return nil
	}

	return e.owner
}

func (e *Endpoint) Reset() {
	if e == nil {
		return
	}

	e.mu.Lock()
	if e.flags&uint32(types.EndpointFlag_Resetting) != 0 {
		e.mu.Unlock()
		return
	}
	e.flags |= uint32(types.EndpointFlag_Resetting)

	// Snapshot and clear connections under the lock, then reset them
	// outside the lock to avoid deadlock with IO goroutine callbacks.
	ctrl := e.ctrlConn
	e.ctrlConn = nil
	data := e.dataConn
	e.dataConn = nil
	e.listenAddress = nil
	e.mu.Unlock()

	if ctrl != nil {
		setConnectionBinding(ctrl, nil)
		ctrl.Reset()
	}

	for _, conn := range data {
		if conn == nil {
			continue
		}
		setConnectionBinding(conn, nil)
		conn.Reset()
	}

	e.ClearPingTimer()

	if e.owner != nil {
		e.owner.AddEndpointGcList(e)
	}

	e.mu.Lock()
	e.flags = 0
	e.mu.Unlock()
}

func (e *Endpoint) GetId() types.BusIdType {
	if e == nil {
		return 0
	}
	return e.id
}

func (e *Endpoint) GetPid() int32 {
	if e == nil {
		return 0
	}
	return e.pid
}

func (e *Endpoint) GetHostname() string {
	if e == nil {
		return ""
	}
	return e.hostname
}

func (e *Endpoint) GetHashCode() string {
	if e == nil {
		return ""
	}
	return e.hashCode
}

func (e *Endpoint) UpdateHashCode(code string) {
	if e == nil {
		return
	}
	if code == "" {
		return
	}
	e.hashCode = code
}

func (e *Endpoint) AddConnection(conn types.Connection, forceData bool) bool {
	if e == nil || conn == nil {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.flags&uint32(types.EndpointFlag_Resetting) != 0 {
		return false
	}

	// 如果进入了handshake流程会第二次添加同一个连接
	if conn.GetBinding() == e {
		if conn.GetStatus() == types.ConnectionState_Handshaking {
			setConnectionStatus(conn, types.ConnectionState_Connected)
		}
		return true
	}

	if conn.GetBinding() != nil {
		return false
	}

	if forceData || e.ctrlConn != nil {
		e.dataConn = append(e.dataConn, conn)
		e.flags &^= uint32(types.EndpointFlag_ConnectionSorted)
	} else {
		e.ctrlConn = conn
	}

	// 已经成功连接可以不需要握手
	// TODO: 注意这里新连接要控制时序，Handshaking检查之后才允许发起连接/响应连接回调流程
	setConnectionBinding(conn, e)
	if conn.GetStatus() == types.ConnectionState_Handshaking {
		setConnectionStatus(conn, types.ConnectionState_Connected)
	}

	return true
}

func (e *Endpoint) RemoveConnection(conn types.Connection) bool {
	if e == nil || conn == nil {
		return false
	}

	e.mu.Lock()

	if conn.GetBinding() != e {
		e.mu.Unlock()
		return false
	}

	if e.flags&uint32(types.EndpointFlag_Resetting) != 0 {
		e.mu.Unlock()
		setConnectionBinding(conn, nil)
		return true
	}

	if conn == e.ctrlConn {
		e.mu.Unlock()
		e.Reset()
		return true
	}

	needReset := false
	for idx, cur := range e.dataConn {
		if cur != conn {
			continue
		}

		setConnectionBinding(conn, nil)
		e.dataConn = append(e.dataConn[:idx], e.dataConn[idx+1:]...)

		if len(e.dataConn) == 0 {
			needReset = true
		}

		e.mu.Unlock()
		if needReset {
			e.Reset()
		}
		return true
	}

	e.mu.Unlock()
	return false
}

func (e *Endpoint) IsAvailable() bool {
	if e == nil {
		return false
	}

	if e.ctrlConn == nil {
		return false
	}

	for _, conn := range e.dataConn {
		if conn != nil && conn.IsRunning() {
			return true
		}
	}

	return false
}

func (e *Endpoint) GetFlags() uint32 {
	if e == nil {
		return 0
	}
	return e.flags
}

func (e *Endpoint) GetFlag(f types.EndpointFlag) bool {
	if e == nil {
		return false
	}

	return e.flags&uint32(f) != 0
}

func (e *Endpoint) SetFlag(f types.EndpointFlag, v bool) {
	if e == nil {
		return
	}

	if f < types.EndpointFlag_MutableFlags {
		return
	}

	e.setFlag(f, v)
}

func (e *Endpoint) GetListenAddress() []types.ChannelAddress {
	if e == nil {
		return nil
	}

	if len(e.listenAddress) == 0 {
		return nil
	}

	ret := make([]types.ChannelAddress, len(e.listenAddress))
	copy(ret, e.listenAddress)
	return ret
}

func (e *Endpoint) ClearListenAddress() {
	if e == nil {
		return
	}

	e.listenAddress = nil
	e.setFlag(types.EndpointFlag_HasListenPorc, false)
	e.setFlag(types.EndpointFlag_HasListenFd, false)
}

func (e *Endpoint) AddListenAddress(addr string) {
	if e == nil {
		return
	}

	if addr == "" {
		return
	}

	lowerAddr := strings.ToLower(addr)
	if strings.HasPrefix(lowerAddr, "mem:") || strings.HasPrefix(lowerAddr, "shm:") {
		e.setFlag(types.EndpointFlag_HasListenPorc, true)
	} else {
		e.setFlag(types.EndpointFlag_HasListenFd, true)
	}

	parsed, ok := channel_utility.MakeAddress(addr)
	if !ok {
		return
	}
	e.listenAddress = append(e.listenAddress, parsed)
}

func (e *Endpoint) UpdateSupportSchemes(schemes []string) {
	if e == nil {
		return
	}

	if len(schemes) == 0 {
		e.supportedSchemas = nil
		return
	}

	supported := make(map[string]struct{}, len(schemes))
	for _, scheme := range schemes {
		if scheme == "" {
			continue
		}
		supported[scheme] = struct{}{}
	}
	e.supportedSchemas = supported
}

func (e *Endpoint) IsSchemeSupported(scheme string) bool {
	if e == nil || scheme == "" || e.supportedSchemas == nil {
		return false
	}

	_, ok := e.supportedSchemas[scheme]
	return ok
}

func (e *Endpoint) AddPingTimer() {
	if e == nil || e.owner == nil {
		return
	}

	// Self node doesn't need ping
	if e.GetId() == e.owner.GetId() {
		return
	}

	// Check if ping interval is valid
	if e.owner.GetConfigure().PingInterval <= 0 {
		return
	}

	// Don't add during GC reset
	if e.owner.CheckFlag(types.NodeFlag_ResettingGc) {
		return
	}

	e.ClearPingTimer()

	if e.GetFlag(types.EndpointFlag_Resetting) {
		return
	}

	// Add to ping list - use type assertion to access internal method
	nextPingTime := e.owner.GetTimerTick().Add(e.owner.GetConfigure().PingInterval)
	e.owner.addPingTimer(e, nextPingTime)

	e.SetFlag(types.EndpointFlag_HasPingTimer, true)
}

func (e *Endpoint) ClearPingTimer() {
	if e == nil {
		return
	}

	if e.owner == nil || !e.GetFlag(types.EndpointFlag_HasPingTimer) {
		return
	}

	// Remove from ping list - use type assertion to access internal method
	e.owner.removePingTimer(e)

	e.SetFlag(types.EndpointFlag_HasPingTimer, false)
}

// ============== connection functions ==============

func (e *Endpoint) GetCtrlConnection(peer types.Endpoint) types.Connection {
	if e == nil || peer == nil {
		return nil
	}

	peerImpl, ok := peer.(*Endpoint)
	if !ok {
		return nil
	}

	if e == peerImpl {
		return nil
	}

	peerImpl.mu.Lock()
	ctrl := peerImpl.ctrlConn
	peerImpl.mu.Unlock()

	if ctrl != nil && ctrl.GetStatus() == types.ConnectionState_Connected {
		return ctrl
	}

	return nil
}

func (e *Endpoint) GetDataConnection(peer types.Endpoint, enableFallbackCtrl bool) types.Connection {
	if e == nil || peer == nil {
		return nil
	}

	peerImpl, ok := peer.(*Endpoint)
	if !ok {
		return nil
	}

	if e == peerImpl {
		return nil
	}

	sharePid := false
	shareHost := false
	if peerImpl.hostname == e.hostname {
		shareHost = true
		if peerImpl.pid == e.pid {
			sharePid = true
		}
	}

	if !peerImpl.GetFlag(types.EndpointFlag_ConnectionSorted) {
		sort.SliceStable(peerImpl.dataConn, func(i, j int) bool {
			return scoreConnection(peerImpl.dataConn[i]) < scoreConnection(peerImpl.dataConn[j])
		})
		peerImpl.setFlag(types.EndpointFlag_ConnectionSorted, true)
	}

	for _, conn := range peerImpl.dataConn {
		if conn == nil || conn.GetStatus() != types.ConnectionState_Connected {
			continue
		}

		if sharePid && conn.CheckFlag(types.ConnectionFlag_AccessShareAddr) {
			return conn
		}

		if shareHost && conn.CheckFlag(types.ConnectionFlag_AccessShareHost) {
			return conn
		}

		if !conn.CheckFlag(types.ConnectionFlag_AccessShareHost) {
			return conn
		}
	}

	if enableFallbackCtrl {
		return e.GetCtrlConnection(peer)
	}

	return nil
}

func (e *Endpoint) GetDataConnectionCount(enableFallbackCtrl bool) int {
	if e == nil {
		return 0
	}

	count := 0
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		switch conn.GetStatus() {
		case types.ConnectionState_Disconnecting, types.ConnectionState_Disconnected:
			continue
		default:
			count++
		}
	}

	if count == 0 && enableFallbackCtrl && e.ctrlConn != nil {
		if e.ctrlConn.GetStatus() != types.ConnectionState_Disconnecting &&
			e.ctrlConn.GetStatus() != types.ConnectionState_Disconnected {
			count++
		}
	}

	return count
}

// ============== statistic functions ==============

func (e *Endpoint) AddStatisticFault() uint64 {
	if e == nil {
		return 0
	}

	e.stat.FaultCount++
	return e.stat.FaultCount
}

func (e *Endpoint) ClearStatisticFault() {
	if e == nil {
		return
	}

	e.stat.FaultCount = 0
}

func (e *Endpoint) SetStatisticUnfinishedPing(p uint64) {
	if e == nil {
		return
	}

	e.stat.UnfinishedPing = p
}

func (e *Endpoint) GetStatisticUnfinishedPing() uint64 {
	if e == nil {
		return 0
	}

	return e.stat.UnfinishedPing
}

func (e *Endpoint) SetStatisticPingDelay(pd time.Duration, pongTimepoint time.Time) {
	if e == nil {
		return
	}

	e.stat.PingDelay = pd
	e.stat.LastPongTime = pongTimepoint
}

func (e *Endpoint) GetStatisticPingDelay() time.Duration {
	if e == nil {
		return 0
	}

	return e.stat.PingDelay
}

func (e *Endpoint) GetStatisticLastPong() time.Time {
	if e == nil {
		return time.Time{}
	}

	return e.stat.LastPongTime
}

func (e *Endpoint) GetStatisticCreatedTime() time.Time {
	if e == nil {
		return time.Time{}
	}

	if !e.stat.CreatedTime.IsZero() {
		return e.stat.CreatedTime
	}

	if e.owner == nil {
		return time.Time{}
	}

	e.stat.CreatedTime = e.owner.GetTimerTick()
	return e.stat.CreatedTime
}

func (e *Endpoint) GetStatisticPushStartTimes() uint64 {
	if e == nil {
		return 0
	}

	var total uint64
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		total += conn.GetStatistic().PushStartTimes
	}
	if e.ctrlConn != nil {
		total += e.ctrlConn.GetStatistic().PushStartTimes
	}
	return total
}

func (e *Endpoint) GetStatisticPushStartSize() uint64 {
	if e == nil {
		return 0
	}

	var total uint64
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		total += conn.GetStatistic().PushStartSize
	}
	if e.ctrlConn != nil {
		total += e.ctrlConn.GetStatistic().PushStartSize
	}
	return total
}

func (e *Endpoint) GetStatisticPushSuccessTimes() uint64 {
	if e == nil {
		return 0
	}

	var total uint64
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		total += conn.GetStatistic().PushSuccessTimes
	}
	if e.ctrlConn != nil {
		total += e.ctrlConn.GetStatistic().PushSuccessTimes
	}
	return total
}

func (e *Endpoint) GetStatisticPushSuccessSize() uint64 {
	if e == nil {
		return 0
	}

	var total uint64
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		total += conn.GetStatistic().PushSuccessSize
	}
	if e.ctrlConn != nil {
		total += e.ctrlConn.GetStatistic().PushSuccessSize
	}
	return total
}

func (e *Endpoint) GetStatisticPushFailedTimes() uint64 {
	if e == nil {
		return 0
	}

	var total uint64
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		total += conn.GetStatistic().PushFailedTimes
	}
	if e.ctrlConn != nil {
		total += e.ctrlConn.GetStatistic().PushFailedTimes
	}
	return total
}

func (e *Endpoint) GetStatisticPushFailedSize() uint64 {
	if e == nil {
		return 0
	}

	var total uint64
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		total += conn.GetStatistic().PushFailedSize
	}
	if e.ctrlConn != nil {
		total += e.ctrlConn.GetStatistic().PushFailedSize
	}
	return total
}

func (e *Endpoint) GetStatisticPullTimes() uint64 {
	if e == nil {
		return 0
	}

	var total uint64
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		total += conn.GetStatistic().PullStartTimes
	}
	if e.ctrlConn != nil {
		total += e.ctrlConn.GetStatistic().PullStartTimes
	}
	return total
}

func (e *Endpoint) GetStatisticPullSize() uint64 {
	if e == nil {
		return 0
	}

	var total uint64
	for _, conn := range e.dataConn {
		if conn == nil {
			continue
		}
		total += conn.GetStatistic().PullStartSize
	}
	if e.ctrlConn != nil {
		total += e.ctrlConn.GetStatistic().PullStartSize
	}
	return total
}

func (e *Endpoint) setFlag(f types.EndpointFlag, v bool) {
	if v {
		e.flags |= uint32(f)
	} else {
		e.flags &^= uint32(f)
	}
}

func scoreConnection(conn types.Connection) int {
	if conn == nil {
		return 0
	}

	score := 0
	if !conn.CheckFlag(types.ConnectionFlag_AccessShareAddr) {
		score += 0x08
	}
	if !conn.CheckFlag(types.ConnectionFlag_AccessShareHost) {
		score += 0x04
	}

	return score
}

type connectionBindingSetter interface {
	setBinding(types.Endpoint)
}

func setConnectionBinding(conn types.Connection, ep types.Endpoint) {
	if conn == nil {
		return
	}
	if setter, ok := conn.(connectionBindingSetter); ok {
		setter.setBinding(ep)
	}
}

type connectionStatusSetter interface {
	setStatus(types.ConnectionState)
}

func setConnectionStatus(conn types.Connection, status types.ConnectionState) {
	if conn == nil {
		return
	}
	if setter, ok := conn.(connectionStatusSetter); ok {
		setter.setStatus(status)
	}
}
