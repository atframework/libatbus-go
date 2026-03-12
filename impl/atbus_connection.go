package libatbus_impl

import (
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
	flags             uint32
	connectionContext *ConnectionContext
	statistic         types.ConnectionStatistic

	// ioStreamConnection is the underlying IO stream connection (for TCP/Unix/Pipe)
	ioStreamConnection *io_stream.IoStreamConnection
}

func CreateConnection(owner *Node, addr string) *Connection {
	if owner == nil {
		return nil
	}

	address, ok := channel_utility.MakeAddress(addr)
	if !ok || address == nil {
		return nil
	}

	c := &Connection{
		owner:             owner,
		binding:           nil,
		status:            types.ConnectionState_Disconnected,
		address:           address,
		flags:             0,
		connectionContext: NewConnectionContext(owner.GetCryptoKeyExchangeType()),
		statistic:         types.ConnectionStatistic{},
	}

	c.connectionContext.SetSupportedKeyExchange(owner.GetCryptoKeyExchangeType())
	return c
}

func (c *Connection) setFlag(f types.ConnectionFlag, v bool) {
	if c == nil {
		return
	}

	if v {
		c.flags |= uint32(f)
	} else {
		c.flags &^= uint32(f)
	}
}

func (c *Connection) Reset() {
	if c == nil {
		return
	}

	if c.CheckFlag(types.ConnectionFlag_Resetting) {
		return
	}
	c.setFlag(types.ConnectionFlag_Resetting, true)

	bindEp := c.binding

	// 后面会重置状态，影响事件判定，所以要先移除检查队列
	c.owner.removeConnectionTimer(c)

	if c.binding != nil {
		c.binding.RemoveConnection(c)
	}

	c.owner.AddConnectionGcList(c)

	c.flags = 0
	c.statistic = types.ConnectionStatistic{}

	c.owner.LogDebug(bindEp, c, nil, "connection disconnected")
	c.owner.OnDisconnect(bindEp, c)
}

func (c *Connection) Proc() types.ErrorType {
	// For iostream mode, proc is driven by goroutines, nothing to do here
	return error_code.EN_ATBUS_ERR_SUCCESS
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

	// Set status to connecting first
	c.status = types.ConnectionState_Connecting
	c.setFlag(types.ConnectionFlag_ListenFd, true)
	c.setFlag(types.ConnectionFlag_ServerMode, true)

	// Start listening
	ret := channel.Listen(c.address.GetAddress())
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		c.status = types.ConnectionState_Disconnected
		c.setFlag(types.ConnectionFlag_ListenFd, false)
		c.setFlag(types.ConnectionFlag_ServerMode, false)
		return ret
	}

	// Mark as connected (listening)
	c.status = types.ConnectionState_Connected

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *Connection) Connect() types.ErrorType {
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

	// Set status to connecting
	c.status = types.ConnectionState_Connecting
	c.setFlag(types.ConnectionFlag_ClientMode, true)

	// Add to connecting list with timeout
	c.owner.eventTimer.connectingList.Put(c.address.GetAddress(),
		types.TimerDescPair[*Connection]{
			Timeout: c.owner.eventTimer.tick.Add(c.owner.configure.FirstIdleTimeout),
			Value:   c,
		})

	// Connect asynchronously
	ioConn, ret := channel.Connect(c.address.GetAddress())
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		c.status = types.ConnectionState_Disconnected
		c.setFlag(types.ConnectionFlag_ClientMode, false)
		c.owner.eventTimer.connectingList.Delete(c.address.GetAddress())
		return ret
	}

	// Type assert to get concrete type
	if concreteConn, ok := ioConn.(*io_stream.IoStreamConnection); ok {
		c.ioStreamConnection = concreteConn
	}
	c.status = types.ConnectionState_Handshaking

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *Connection) Disconnect() types.ErrorType {
	if c == nil || c.owner == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if c.status == types.ConnectionState_Disconnected {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	c.status = types.ConnectionState_Disconnecting

	// Disconnect the underlying IO stream connection
	if c.ioStreamConnection != nil {
		channel := c.getIoStreamChannel()
		if channel != nil {
			channel.Disconnect(c.ioStreamConnection)
		}
		c.ioStreamConnection = nil
	}

	c.status = types.ConnectionState_Disconnected

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *Connection) Push(data []byte) types.ErrorType {
	if c == nil || c.owner == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if c.status != types.ConnectionState_Connected {
		return error_code.EN_ATBUS_ERR_CLOSING
	}

	if c.ioStreamConnection == nil {
		return error_code.EN_ATBUS_ERR_NO_DATA
	}

	channel := c.getIoStreamChannel()
	if channel == nil {
		return error_code.EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT
	}

	return channel.Send(c.ioStreamConnection, data)
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

func (c *Connection) AddStatFault() uint64 {
	if c == nil {
		return 0
	}

	c.statistic.FaultCount++
	return c.statistic.FaultCount
}

func (c *Connection) ClearStatFault() {
	if c == nil {
		return
	}

	c.statistic.FaultCount = 0
}

func (c *Connection) GetAddress() types.ChannelAddress {
	if c == nil {
		return nil
	}

	return c.address
}

func (c *Connection) IsConnected() bool {
	if c == nil {
		return false
	}

	return c.status == types.ConnectionState_Connected
}

func (c *Connection) IsRunning() bool {
	if c == nil {
		return false
	}

	return c.status == types.ConnectionState_Connecting ||
		c.status == types.ConnectionState_Handshaking ||
		c.status == types.ConnectionState_Connected
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

	return c.status
}

func (c *Connection) CheckFlag(flag types.ConnectionFlag) bool {
	if c == nil {
		return false
	}

	return c.flags&uint32(flag) != 0
}

func (c *Connection) SetTemporary() {
	if c == nil {
		return
	}

	c.flags |= uint32(types.ConnectionFlag_Temporary)
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
	// TODO: implement owner checker removal
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

	c.status = status
}
