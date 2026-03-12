package libatbus_channel_iostream

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	buffer "github.com/atframework/libatbus-go/buffer"
	types "github.com/atframework/libatbus-go/types"
)

// IoStreamConfigure is an alias for types.IoStreamConfigure.
type IoStreamConfigure = types.IoStreamConfigure

// SetDefaultIoStreamConfigure is an alias for types.SetDefaultIoStreamConfigure.
var SetDefaultIoStreamConfigure = types.SetDefaultIoStreamConfigure

// IoStreamCallbackEventType defines the type of IO stream callback event.
type IoStreamCallbackEventType = types.IoStreamCallbackEventType

const (
	// IoStreamCallbackEventTypeAccepted is triggered when a new connection is accepted.
	IoStreamCallbackEventTypeAccepted = types.IoStreamCallbackEventType_Accepted
	// IoStreamCallbackEventTypeConnected is triggered when an outbound connection is established.
	IoStreamCallbackEventTypeConnected = types.IoStreamCallbackEventType_Connected
	// IoStreamCallbackEventTypeDisconnected is triggered when a connection is closed.
	IoStreamCallbackEventTypeDisconnected = types.IoStreamCallbackEventType_Disconnected
	// IoStreamCallbackEventTypeReceived is triggered when data is received.
	IoStreamCallbackEventTypeReceived = types.IoStreamCallbackEventType_Received
	// IoStreamCallbackEventTypeWritten is triggered when data has been written.
	IoStreamCallbackEventTypeWritten = types.IoStreamCallbackEventType_Written
	// IoStreamCallbackEventTypeMax is the maximum event type value.
	IoStreamCallbackEventTypeMax = types.IoStreamCallbackEventType_Max
)

// IoStreamCallbackFunc is the callback function type for IO stream events.
type IoStreamCallbackFunc = types.IoStreamCallbackFunc

// IoStreamCallbackEventHandleSet holds callback functions for IO stream events.
type IoStreamCallbackEventHandleSet = types.IoStreamCallbackEventHandleSet

// IoStreamConnectionFlag defines connection flags.
type IoStreamConnectionFlag = types.IoStreamConnectionFlag

const (
	// IoStreamConnectionFlagListen indicates this is a listening connection.
	IoStreamConnectionFlagListen = types.IoStreamConnectionFlag_Listen
	// IoStreamConnectionFlagConnect indicates this is an outbound connection.
	IoStreamConnectionFlagConnect = types.IoStreamConnectionFlag_Connect
	// IoStreamConnectionFlagAccept indicates this connection was accepted from a listener.
	IoStreamConnectionFlagAccept = types.IoStreamConnectionFlag_Accept
	// IoStreamConnectionFlagWriting indicates the connection is currently writing.
	IoStreamConnectionFlagWriting = types.IoStreamConnectionFlag_Writing
	// IoStreamConnectionFlagClosing indicates the connection is being closed.
	IoStreamConnectionFlagClosing = types.IoStreamConnectionFlag_Closing
)

// IoStreamConnectionStatus defines connection status.
type IoStreamConnectionStatus = types.IoStreamConnectionStatus

const (
	// IoStreamConnectionStatusCreated indicates the connection has been created but not connected.
	IoStreamConnectionStatusCreated = types.IoStreamConnectionStatus_Created
	// IoStreamConnectionStatusConnected indicates the connection is established.
	IoStreamConnectionStatusConnected = types.IoStreamConnectionStatus_Connected
	// IoStreamConnectionStatusDisconnecting indicates the connection is being disconnected.
	IoStreamConnectionStatusDisconnecting = types.IoStreamConnectionStatus_Disconnecting
	// IoStreamConnectionStatusDisconnected indicates the connection has been closed.
	IoStreamConnectionStatusDisconnected = types.IoStreamConnectionStatus_Disconnected
)

// IoStreamChannelFlag defines channel flags.
type IoStreamChannelFlag = types.IoStreamChannelFlag

const (
	// IoStreamChannelFlagIsLoopOwner indicates the channel owns the event loop.
	IoStreamChannelFlagIsLoopOwner = types.IoStreamChannelFlag_IsLoopOwner
	// IoStreamChannelFlagClosing indicates the channel is being closed.
	IoStreamChannelFlagClosing = types.IoStreamChannelFlag_Closing
	// IoStreamChannelFlagInCallback indicates we're currently in a callback.
	IoStreamChannelFlagInCallback = types.IoStreamChannelFlag_InCallback
)

// IoStreamConnection represents a single IO stream connection.
type IoStreamConnection struct {
	// channel is the parent channel
	channel *IoStreamChannel

	// conn is the underlying network connection
	conn net.Conn

	// address is the parsed connection address
	address types.ChannelAddress

	// status is the current connection status
	status IoStreamConnectionStatus

	// flags holds connection flags
	flags uint32

	// eventHandleSet holds event callbacks
	eventHandleSet IoStreamCallbackEventHandleSet

	// proactivelyDisconnectCallback is called when proactively disconnecting
	proactivelyDisconnectCallback IoStreamCallbackFunc

	// readBufferManager manages the receive buffer
	readBufferManager *buffer.BufferManager

	// frameReader handles frame parsing
	frameReader *FrameReader

	// writeQueue is the channel for outgoing messages
	writeQueue chan []byte

	// privateData holds user-defined data
	privateData interface{}

	// mu protects concurrent access
	mu sync.RWMutex

	// closed indicates if the connection has been closed
	closed atomic.Bool

	// readEgainCount tracks EAGAIN errors
	readEgainCount uint64
	// checkBlockSizeFailedCount tracks block size check failures
	checkBlockSizeFailedCount uint64
	// checkHashFailedCount tracks hash check failures
	checkHashFailedCount uint64
}

// GetAddress returns the connection address.
func (c *IoStreamConnection) GetAddress() types.ChannelAddress {
	return c.address
}

// GetStatus returns the current connection status.
func (c *IoStreamConnection) GetStatus() IoStreamConnectionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// SetStatus sets the connection status.
func (c *IoStreamConnection) SetStatus(status IoStreamConnectionStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}

// SetFlag sets or clears a connection flag.
func (c *IoStreamConnection) SetFlag(f IoStreamConnectionFlag, v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v {
		c.flags |= uint32(f)
	} else {
		c.flags &^= uint32(f)
	}
}

// GetFlag returns whether a flag is set.
func (c *IoStreamConnection) GetFlag(f IoStreamConnectionFlag) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return (c.flags & uint32(f)) != 0
}

// GetChannel returns the parent channel.
func (c *IoStreamConnection) GetChannel() types.IoStreamChannel {
	return c.channel
}

// GetEventHandleSet returns the event handle set.
func (c *IoStreamConnection) GetEventHandleSet() *IoStreamCallbackEventHandleSet {
	return &c.eventHandleSet
}

// GetProactivelyDisconnectCallback returns the proactive disconnect callback.
func (c *IoStreamConnection) GetProactivelyDisconnectCallback() IoStreamCallbackFunc {
	return c.proactivelyDisconnectCallback
}

// GetReadBufferManager returns the read buffer manager.
func (c *IoStreamConnection) GetReadBufferManager() *buffer.BufferManager {
	return c.readBufferManager
}

// SetPrivateData sets user-defined private data.
func (c *IoStreamConnection) SetPrivateData(data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.privateData = data
}

// GetPrivateData returns user-defined private data.
func (c *IoStreamConnection) GetPrivateData() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.privateData
}

// GetNetConn returns the underlying net.Conn.
func (c *IoStreamConnection) GetNetConn() net.Conn {
	return c.conn
}

// Ensure IoStreamConnection implements types.IoStreamConnection
var _ types.IoStreamConnection = (*IoStreamConnection)(nil)

// IoStreamChannel manages IO stream connections.
type IoStreamChannel struct {
	// conf holds the channel configuration
	conf IoStreamConfigure

	// ctx is the channel context
	ctx context.Context
	// cancel is the context cancel function
	cancel context.CancelFunc

	// listeners maps address string to listener
	listeners map[string]net.Listener

	// connections maps connection to IoStreamConnection
	connections map[net.Conn]*IoStreamConnection

	// eventHandleSet holds event callbacks
	eventHandleSet IoStreamCallbackEventHandleSet

	// flags holds channel flags
	flags uint32

	// Statistics
	statisticActiveRequestCount        uint64
	statisticReadNetEgainCount         uint64
	statisticCheckBlockSizeFailedCount uint64
	statisticCheckHashFailedCount      uint64

	// privateData holds user-defined data
	privateData interface{}

	// mu protects concurrent access
	mu sync.RWMutex

	// closed indicates if the channel has been closed
	closed atomic.Bool
}

// GetContext returns the channel context.
func (c *IoStreamChannel) GetContext() context.Context {
	return c.ctx
}

// SetFlag sets or clears a channel flag.
func (c *IoStreamChannel) SetFlag(f IoStreamConnectionFlag, v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v {
		c.flags |= uint32(f)
	} else {
		c.flags &^= uint32(f)
	}
}

// GetFlag returns whether a flag is set.
func (c *IoStreamChannel) GetFlag(f IoStreamConnectionFlag) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return (c.flags & uint32(f)) != 0
}

// GetEventHandleSet returns the event handle set.
func (c *IoStreamChannel) GetEventHandleSet() *IoStreamCallbackEventHandleSet {
	return &c.eventHandleSet
}

// GetStatisticActiveRequestCount returns the active request count.
func (c *IoStreamChannel) GetStatisticActiveRequestCount() uint64 {
	return atomic.LoadUint64(&c.statisticActiveRequestCount)
}

// GetStatisticReadNetEgainCount returns the EAGAIN count.
func (c *IoStreamChannel) GetStatisticReadNetEgainCount() uint64 {
	return atomic.LoadUint64(&c.statisticReadNetEgainCount)
}

// GetStatisticCheckBlockSizeFailedCount returns the block size check failure count.
func (c *IoStreamChannel) GetStatisticCheckBlockSizeFailedCount() uint64 {
	return atomic.LoadUint64(&c.statisticCheckBlockSizeFailedCount)
}

// GetStatisticCheckHashFailedCount returns the hash check failure count.
func (c *IoStreamChannel) GetStatisticCheckHashFailedCount() uint64 {
	return atomic.LoadUint64(&c.statisticCheckHashFailedCount)
}

// SetPrivateData sets user-defined private data.
func (c *IoStreamChannel) SetPrivateData(data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.privateData = data
}

// GetPrivateData returns user-defined private data.
func (c *IoStreamChannel) GetPrivateData() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.privateData
}

// GetConfigure returns the channel configuration.
func (c *IoStreamChannel) GetConfigure() *IoStreamConfigure {
	return &c.conf
}
