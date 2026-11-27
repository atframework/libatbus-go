package libatbus_impl

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	io_stream "github.com/atframework/libatbus-go/channel/io_stream"
	channel_utility "github.com/atframework/libatbus-go/channel/utility"
	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

var _ types.Connection = (*Connection)(nil)

type Connection struct {
	owner             *Node
	binding           types.Endpoint
	status            types.ConnectionState
	address           types.ChannelAddress
	originAddress     string
	autoAddressMode   bool
	flags             uint32
	connectionContext *ConnectionContext
	statistic         types.ConnectionStatistic

	// ioStreamConnection is the underlying IO stream connection (for TCP/Unix/Pipe)
	ioStreamConnection *io_stream.IoStreamConnection
}

func normalizeAddressForListenChannel(address *channel_utility.ChannelAddress) {
	if address == nil {
		return
	}

	scheme := strings.ToLower(address.Scheme)
	if (scheme == "unix" || scheme == "pipe") && !io_stream.IsNativePipePath(address.Host) {
		if !filepath.IsAbs(address.Host) {
			absPath, err := filepath.Abs(address.Host)
			maxAddrSize := io_stream.GetMaxUnixSocketLength()
			if err == nil && maxAddrSize > 0 && len(absPath) <= maxAddrSize {
				*address = *channel_utility.MakeAddressFromComponents(address.Scheme, absPath, 0)
			}
		}
	}
}

func normalizeAddressForConnectChannel(address *channel_utility.ChannelAddress) *channel_utility.ChannelAddress {
	if address == nil {
		return nil
	}

	scheme := strings.ToLower(address.Scheme)
	if scheme == "atcp" || scheme == "ipv6" || scheme == "ipv4" {
		if address.Host == "0.0.0.0" {
			return channel_utility.MakeAddressFromComponents("atcp", "127.0.0.1", address.Port)
		}
		if address.Host == "::" {
			return channel_utility.MakeAddressFromComponents("atcp", "::1", address.Port)
		}
	}

	return address
}

func makeConnectionAddress(addr string, listenMode ...bool) (*channel_utility.ChannelAddress, bool) {
	address, ok := channel_utility.MakeAddress(addr)
	if !ok || address == nil {
		return nil, false
	}

	if len(listenMode) == 0 {
		return address, true
	}
	if listenMode[0] {
		normalizeAddressForListenChannel(address)
	} else {
		address = normalizeAddressForConnectChannel(address)
	}

	return address, address != nil
}

func CreateConnection(owner *Node, addr string, listenMode ...bool) *Connection {
	if owner == nil || addr == "" {
		return nil
	}

	address, ok := makeConnectionAddress(addr, listenMode...)
	if !ok || address == nil {
		return nil
	}

	c := &Connection{
		owner:             owner,
		binding:           nil,
		status:            types.ConnectionState_Disconnected,
		address:           address,
		originAddress:     addr,
		autoAddressMode:   len(listenMode) == 0,
		flags:             0,
		connectionContext: NewConnectionContext(owner.GetCryptoKeyExchangeType()),
		statistic:         types.ConnectionStatistic{},
	}

	if !owner.addConnectionTimer(c) {
		return nil
	}

	return c
}

func (c *Connection) setFlag(f types.ConnectionFlag, v bool) {
	if c == nil {
		return
	}

	mask := uint32(f)
	for {
		old := atomic.LoadUint32(&c.flags)
		var next uint32
		if v {
			next = old | mask
		} else {
			next = old &^ mask
		}
		if atomic.CompareAndSwapUint32(&c.flags, old, next) {
			return
		}
	}
}

func (c *Connection) trySetFlag(f types.ConnectionFlag) bool {
	if c == nil {
		return false
	}

	mask := uint32(f)
	for {
		old := atomic.LoadUint32(&c.flags)
		if old&mask != 0 {
			return false
		}
		if atomic.CompareAndSwapUint32(&c.flags, old, old|mask) {
			return true
		}
	}
}

func (c *Connection) Reset() {
	if c == nil {
		return
	}

	if !c.trySetFlag(types.ConnectionFlag_Resetting) {
		return
	}

	bindEp := c.binding

	// 后面会重置状态，影响事件判定，所以要先移除检查队列
	c.owner.removeConnectionTimer(c, false)

	c.Disconnect()

	if c.binding != nil {
		c.binding.RemoveConnection(c)
	}

	c.owner.AddConnectionGcList(c)

	atomic.StoreUint32(&c.flags, 0)
	c.statistic = types.ConnectionStatistic{}

	c.owner.LogDebug(bindEp, c, nil, "connection disconnected")
	c.owner.OnDisconnect(bindEp, c)
}

func (c *Connection) Proc() types.ErrorType {
	// For iostream mode, proc is driven by goroutines, nothing to do here
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *Connection) resolveAddressMode(listen bool) {
	if !c.autoAddressMode {
		return
	}
	// The factory already parsed originAddress and registered its timeout.
	address, _ := makeConnectionAddress(c.originAddress, listen)
	old := c.address.GetAddress()
	c.address = address
	if old == address.Address {
		return
	}
	timer := &c.owner.eventTimer
	timer.connectingListMu.Lock()
	defer timer.connectingListMu.Unlock()
	if timer.connectingListByChannel[old] == c {
		delete(timer.connectingListByChannel, old)
	}
	if _, pending := timer.connectingListByPtr.Get(c, false); pending {
		timer.connectingListByChannel[address.Address] = c
	}
}

func (c *Connection) Listen() types.ErrorType {
	if c == nil || c.owner == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if c.address == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// Get the channel from owner
	channel := c.getIoStreamChannel()
	if channel == nil {
		return error_code.EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT
	}

	c.resolveAddressMode(true)
	scheme := strings.ToLower(c.address.GetScheme())
	if scheme == "unix" || scheme == "pipe" {
		c.setFlag(types.ConnectionFlag_AccessShareHost, true)
	}

	// Set status to connecting first
	c.setStatus(types.ConnectionState_Connecting)
	c.setFlag(types.ConnectionFlag_ListenFd, true)
	c.setFlag(types.ConnectionFlag_ServerMode, true)

	// Start listening
	ret := channel.Listen(c.address.GetAddress())
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		c.owner.LogError(c.GetBinding(), c, 0, ret,
			fmt.Sprintf("listen %s failed", c.address.GetAddress()))
		c.setStatus(types.ConnectionState_Disconnected)
		c.setFlag(types.ConnectionFlag_ListenFd, false)
		c.setFlag(types.ConnectionFlag_ServerMode, false)
		c.owner.removeConnectionTimer(c, false)
		return ret
	}

	// Mark as connected (listening) — also removes the connection timer
	c.setStatus(types.ConnectionState_Connected)

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *Connection) Connect() types.ErrorType {
	return c.connect(nil)
}

func (c *Connection) connect(endpoint types.Endpoint) types.ErrorType {
	if c == nil || c.owner == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if c.address == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	// Get the channel from owner
	channel := c.getIoStreamChannel()
	if channel == nil {
		return error_code.EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT
	}

	c.resolveAddressMode(false)
	scheme := strings.ToLower(c.address.GetScheme())
	if scheme == "unix" || scheme == "pipe" {
		c.setFlag(types.ConnectionFlag_AccessShareHost, true)
	}

	// Set status to connecting
	c.setStatus(types.ConnectionState_Connecting)
	c.setFlag(types.ConnectionFlag_RegFd, true)
	c.setFlag(types.ConnectionFlag_ClientMode, true)

	// Connect
	ioConn, ret := channel.Connect(c.address.GetAddress())
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		c.owner.LogError(c.GetBinding(), c, 0, ret,
			fmt.Sprintf("channel connect to %s failed", c.address.GetAddress()))
		c.Reset()
		return ret
	}

	// Type assert to get concrete type
	if concreteConn, ok := ioConn.(*io_stream.IoStreamConnection); ok {
		c.ioStreamConnection = concreteConn
	}
	ioConn.SetPrivateData(c)
	// Attach an explicitly selected endpoint while the connection is still
	// connecting. AddConnection promotes a handshaking connection to connected.
	if endpoint != nil && !endpoint.AddConnection(c, false) {
		c.Reset()
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}
	c.setStatus(types.ConnectionState_Handshaking)

	ret = c.owner.OnNewConnection(c)
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		c.Reset()
		return ret
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *Connection) Disconnect() types.ErrorType {
	if c == nil || c.owner == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if c.GetStatus() == types.ConnectionState_Disconnected {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	c.setStatus(types.ConnectionState_Disconnecting)

	// Disconnect the underlying IO stream connection
	if c.ioStreamConnection != nil {
		ioConn := c.ioStreamConnection
		c.ioStreamConnection = nil
		ioConn.SetPrivateData(nil)
		channel := c.getIoStreamChannel()
		if channel != nil {
			channel.Disconnect(ioConn)
		}
	}

	c.setStatus(types.ConnectionState_Disconnected)

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *Connection) Push(data []byte) types.ErrorType {
	if c == nil || c.owner == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	payloadSize := uint64(len(data))
	c.statistic.PushStartTimes++
	c.statistic.PushStartSize += payloadSize

	if c.status != types.ConnectionState_Connected && c.status != types.ConnectionState_Handshaking {
		c.statistic.PushFailedTimes++
		c.statistic.PushFailedSize += payloadSize
		return error_code.EN_ATBUS_ERR_CLOSING
	}

	if c.ioStreamConnection == nil {
		c.statistic.PushFailedTimes++
		c.statistic.PushFailedSize += payloadSize
		return error_code.EN_ATBUS_ERR_NO_DATA
	}

	channel := c.getIoStreamChannel()
	if channel == nil {
		c.statistic.PushFailedTimes++
		c.statistic.PushFailedSize += payloadSize
		return error_code.EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT
	}

	ret := channel.Send(c.ioStreamConnection, data)
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		c.statistic.PushFailedTimes++
		c.statistic.PushFailedSize += payloadSize
	}

	return ret
}

// getIoStreamChannel returns the IoStreamChannel from the owner node.
func (c *Connection) getIoStreamChannel() *io_stream.IoStreamChannel {
	if c == nil || c.owner == nil {
		return nil
	}

	channel := c.owner.ioStreamChannel
	if channel == nil {
		return nil
	}

	// Type assert to get the concrete type
	if ioChannel, ok := channel.(*io_stream.IoStreamChannel); ok {
		return ioChannel
	}
	return nil
}

// GetIoStreamConnection returns the underlying IoStreamConnection.
func (c *Connection) GetIoStreamConnection() *io_stream.IoStreamConnection {
	if c == nil {
		return nil
	}
	return c.ioStreamConnection
}

// SetIoStreamConnection sets the underlying IoStreamConnection.
func (c *Connection) SetIoStreamConnection(conn *io_stream.IoStreamConnection) {
	if c == nil {
		return
	}
	c.ioStreamConnection = conn
}

func (c *Connection) AddStatisticFault() uint64 {
	if c == nil {
		return 0
	}

	return atomic.AddUint64(&c.statistic.FaultCount, 1)
}

func (c *Connection) ClearStatisticFault() {
	if c == nil {
		return
	}

	atomic.StoreUint64(&c.statistic.FaultCount, 0)
}

func (c *Connection) GetAddress() types.ChannelAddress {
	if c == nil {
		return nil
	}

	return c.address
}

func (c *Connection) GetOriginAddress() string {
	if c == nil {
		return ""
	}

	return c.originAddress
}

func (c *Connection) IsConnected() bool {
	if c == nil {
		return false
	}

	return c.GetStatus() == types.ConnectionState_Connected
}

func (c *Connection) IsRunning() bool {
	if c == nil {
		return false
	}

	status := c.GetStatus()
	return status == types.ConnectionState_Connecting ||
		status == types.ConnectionState_Handshaking ||
		status == types.ConnectionState_Connected
}

func (c *Connection) GetBinding() types.Endpoint {
	if c == nil {
		return nil
	}

	return c.binding
}

func (c *Connection) GetStatus() types.ConnectionState {
	if c == nil {
		return types.ConnectionState_Disconnected
	}

	return types.ConnectionState(atomic.LoadInt32((*int32)(&c.status)))
}

func (c *Connection) CheckFlag(flag types.ConnectionFlag) bool {
	if c == nil {
		return false
	}

	return atomic.LoadUint32(&c.flags)&uint32(flag) != 0
}

func (c *Connection) SetTemporary() {
	if c == nil {
		return
	}

	c.setFlag(types.ConnectionFlag_Temporary, true)
}

func (c *Connection) GetStatistic() types.ConnectionStatistic {
	if c == nil {
		return types.ConnectionStatistic{}
	}

	return c.statistic
}

func (c *Connection) GetConnectionContext() types.ConnectionContext {
	if c == nil {
		return nil
	}

	return c.connectionContext
}

func (c *Connection) RemoveOwnerChecker() {
	if c == nil {
		return
	}

	if c.owner == nil {
		return
	}

	c.owner.removeConnectionTimer(c, false)
}

func (c *Connection) setBinding(ep types.Endpoint) {
	if c == nil {
		return
	}

	c.binding = ep
}

func (c *Connection) setStatus(status types.ConnectionState) {
	if c == nil {
		return
	}

	old := types.ConnectionState(atomic.SwapInt32((*int32)(&c.status), int32(status)))
	if old == status {
		return
	}

	if c.owner != nil && status == types.ConnectionState_Connected {
		c.owner.removeConnectionTimer(c, false)
	}
}
