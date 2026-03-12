package libatbus_channel_iostream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"

	buffer "github.com/atframework/libatbus-go/buffer"
	channel_utility "github.com/atframework/libatbus-go/channel/utility"
	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

const (
	// DefaultWriteQueueSize is the default size of the write queue
	DefaultWriteQueueSize = 1024
	// DefaultReadBufferSize is the default read buffer size
	DefaultReadBufferSize = 64 * 1024
)

// NewIoStreamChannel creates a new IO stream channel.
func NewIoStreamChannel(ctx context.Context, conf *IoStreamConfigure) *IoStreamChannel {
	if ctx == nil {
		ctx = context.Background()
	}

	channelCtx, cancel := context.WithCancel(ctx)

	channel := &IoStreamChannel{
		ctx:         channelCtx,
		cancel:      cancel,
		listeners:   make(map[string]net.Listener),
		connections: make(map[net.Conn]*IoStreamConnection),
	}

	if conf != nil {
		channel.conf = *conf
	} else {
		SetDefaultIoStreamConfigure(&channel.conf)
	}

	return channel
}

// Listen starts listening on the specified address.
// Supported address formats:
//   - ipv4://host:port - IPv4/IPv6 TCP
//   - ipv6://host:port - IPv4/IPv6 TCP
//   - atcp://host:port - IPv4/IPv6 TCP
//   - dns://host:port - DNS resolved TCP
//   - unix://path - Unix domain socket
//   - pipe://path - Named pipe (same as unix)
func (c *IoStreamChannel) Listen(addr string) error_code.ErrorType {
	if c.closed.Load() {
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}

	parsedAddr, ok := channel_utility.MakeAddress(addr)
	if !ok {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	network, address := c.resolveAddress(parsedAddr)
	if network == "" {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	listener, err := net.Listen(network, address)
	if err != nil {
		return error_code.EN_ATBUS_ERR_SOCK_BIND_FAILED
	}

	c.mu.Lock()
	c.listeners[addr] = listener
	c.mu.Unlock()

	// Start accepting connections in a goroutine
	go c.acceptLoop(listener, parsedAddr)

	return error_code.EN_ATBUS_ERR_SUCCESS
}

// Connect connects to the specified address.
// Returns the connection and error code.
func (c *IoStreamChannel) Connect(addr string) (types.IoStreamConnection, error_code.ErrorType) {
	if c.closed.Load() {
		return nil, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}

	parsedAddr, ok := channel_utility.MakeAddress(addr)
	if !ok {
		return nil, error_code.EN_ATBUS_ERR_PARAMS
	}

	network, address := c.resolveAddress(parsedAddr)
	if network == "" {
		return nil, error_code.EN_ATBUS_ERR_PARAMS
	}

	// Create dialer with timeout
	dialer := net.Dialer{
		Timeout:   c.conf.ConfirmTimeout,
		KeepAlive: c.conf.Keepalive,
	}

	conn, err := dialer.DialContext(c.ctx, network, address)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
		}
		return nil, error_code.EN_ATBUS_ERR_SOCK_CONNECT_FAILED
	}

	// Set TCP options
	c.setTCPOptions(conn)

	// Create connection wrapper
	ioConn := c.createConnection(conn, parsedAddr, IoStreamConnectionFlagConnect)

	// Fire connected callback
	c.fireCallback(IoStreamCallbackEventTypeConnected, ioConn, 0)

	// Start read goroutine
	go c.readLoop(ioConn)

	// Start write goroutine
	go c.writeLoop(ioConn)

	return ioConn, error_code.EN_ATBUS_ERR_SUCCESS
}

// Send sends data to the specified connection.
// The data will be automatically framed with hash and length prefix.
func (c *IoStreamChannel) Send(conn types.IoStreamConnection, data []byte) error_code.ErrorType {
	if c.closed.Load() {
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	}

	if conn == nil {
		return error_code.EN_ATBUS_ERR_CLOSING
	}

	ioConn, ok := conn.(*IoStreamConnection)
	if !ok || ioConn == nil || ioConn.closed.Load() {
		return error_code.EN_ATBUS_ERR_CLOSING
	}

	if ioConn.GetStatus() != IoStreamConnectionStatusConnected {
		return error_code.EN_ATBUS_ERR_CLOSING
	}

	// Check message size limit
	if c.conf.SendBufferLimitSize > 0 && uint64(len(data)) > c.conf.SendBufferLimitSize {
		return error_code.EN_ATBUS_ERR_INVALID_SIZE
	}

	// Pack the frame
	frameSize := PackFrameSize(uint64(len(data)))
	frame := make([]byte, frameSize)
	written := PackFrame(data, frame)
	if written != frameSize {
		return error_code.EN_ATBUS_ERR_MALLOC
	}

	// Send to write queue
	select {
	case ioConn.writeQueue <- frame:
		return error_code.EN_ATBUS_ERR_SUCCESS
	case <-c.ctx.Done():
		return error_code.EN_ATBUS_ERR_CHANNEL_CLOSING
	default:
		// Queue is full
		return error_code.EN_ATBUS_ERR_BUFF_LIMIT
	}
}

// Disconnect disconnects the specified connection.
func (c *IoStreamChannel) Disconnect(conn types.IoStreamConnection) error_code.ErrorType {
	if conn == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	ioConn, ok := conn.(*IoStreamConnection)
	if !ok || ioConn == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	return c.disconnectInternal(ioConn, false)
}

// Close closes the channel and all connections.
func (c *IoStreamChannel) Close() error_code.ErrorType {
	if c.closed.Swap(true) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	c.mu.Lock()
	c.flags |= uint32(IoStreamChannelFlagClosing)
	c.mu.Unlock()

	// Cancel the context to stop all goroutines
	c.cancel()

	// Close all listeners
	c.mu.Lock()
	for addr, listener := range c.listeners {
		listener.Close()
		delete(c.listeners, addr)
	}
	c.mu.Unlock()

	// Close all connections
	c.mu.Lock()
	connections := make([]*IoStreamConnection, 0, len(c.connections))
	for _, conn := range c.connections {
		connections = append(connections, conn)
	}
	c.mu.Unlock()

	for _, conn := range connections {
		c.disconnectInternal(conn, true)
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

// GetConnection returns all current connections.
func (c *IoStreamChannel) GetConnections() []*IoStreamConnection {
	c.mu.RLock()
	defer c.mu.RUnlock()

	connections := make([]*IoStreamConnection, 0, len(c.connections))
	for _, conn := range c.connections {
		connections = append(connections, conn)
	}
	return connections
}

// acceptLoop handles accepting new connections.
func (c *IoStreamChannel) acceptLoop(listener net.Listener, listenAddr *channel_utility.ChannelAddress) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-c.ctx.Done():
				return
			default:
				// Log error but continue accepting
				continue
			}
		}

		// Check if channel is closing
		if c.closed.Load() {
			conn.Close()
			return
		}

		// Set TCP options
		c.setTCPOptions(conn)

		// Create connection wrapper with the actual remote address
		remoteAddr := conn.RemoteAddr().String()
		var parsedAddr *channel_utility.ChannelAddress

		// Determine the scheme based on the listener address
		switch listenAddr.Scheme {
		case "unix", "pipe":
			parsedAddr = channel_utility.MakeAddressFromComponents(listenAddr.Scheme, remoteAddr, 0)
		default:
			// For TCP connections, parse the remote address
			host, port := parseHostPort(remoteAddr)
			parsedAddr = channel_utility.MakeAddressFromComponents(listenAddr.Scheme, host, port)
		}

		ioConn := c.createConnection(conn, parsedAddr, IoStreamConnectionFlagAccept)

		// Fire accepted callback
		c.fireCallback(IoStreamCallbackEventTypeAccepted, ioConn, 0)

		// Start read goroutine
		go c.readLoop(ioConn)

		// Start write goroutine
		go c.writeLoop(ioConn)
	}
}

// readLoop handles reading data from a connection.
func (c *IoStreamChannel) readLoop(conn *IoStreamConnection) {
	defer func() {
		// Mark connection as disconnected
		conn.SetStatus(IoStreamConnectionStatusDisconnected)
		conn.closed.Store(true)

		// Remove from connections map
		c.mu.Lock()
		delete(c.connections, conn.conn)
		c.mu.Unlock()

		// Fire disconnected callback
		c.fireCallback(IoStreamCallbackEventTypeDisconnected, conn, 0)
	}()

	readBuf := make([]byte, DefaultReadBufferSize)

	for {
		// Check if we should stop
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if conn.closed.Load() || conn.GetStatus() != IoStreamConnectionStatusConnected {
			return
		}

		// Set read deadline based on context
		if deadline, ok := c.ctx.Deadline(); ok {
			conn.conn.SetReadDeadline(deadline)
		}

		n, err := conn.conn.Read(readBuf)
		if err != nil {
			if err == io.EOF {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Timeout, check context and continue
				select {
				case <-c.ctx.Done():
					return
				default:
					atomic.AddUint64(&conn.readEgainCount, 1)
					atomic.AddUint64(&c.statisticReadNetEgainCount, 1)
					if conn.readEgainCount > c.conf.MaxReadNetEgainCount {
						return
					}
					continue
				}
			}
			// Other error, disconnect
			return
		}

		if n == 0 {
			continue
		}

		// Write to frame reader
		conn.frameReader.Write(readBuf[:n])

		// Try to read complete frames
		for {
			result := conn.frameReader.ReadFrame()
			if result.Error != nil {
				if errors.Is(result.Error, ErrIncompleteFrame) {
					// Need more data
					break
				}

				if errors.Is(result.Error, ErrInvalidFrameHash) {
					atomic.AddUint64(&conn.checkHashFailedCount, 1)
					atomic.AddUint64(&c.statisticCheckHashFailedCount, 1)
					if conn.checkHashFailedCount > c.conf.MaxReadCheckHashFailedCount {
						return
					}
					// Skip this frame and continue
					continue
				}

				// Other error
				return
			}

			// Check payload size
			if c.conf.ReceiveBufferLimitSize > 0 && uint64(len(result.Payload)) > c.conf.ReceiveBufferLimitSize {
				atomic.AddUint64(&conn.checkBlockSizeFailedCount, 1)
				atomic.AddUint64(&c.statisticCheckBlockSizeFailedCount, 1)
				if conn.checkBlockSizeFailedCount > c.conf.MaxReadCheckBlockSizeFailedCount {
					return
				}
				continue
			}

			// Fire received callback
			c.fireCallbackWithData(IoStreamCallbackEventTypeReceived, conn, 0, result.Payload)
		}
	}
}

// writeLoop handles writing data to a connection.
func (c *IoStreamChannel) writeLoop(conn *IoStreamConnection) {
	for {
		select {
		case <-c.ctx.Done():
			return
		case data, ok := <-conn.writeQueue:
			if !ok {
				return
			}

			if conn.closed.Load() || conn.GetStatus() != IoStreamConnectionStatusConnected {
				return
			}

			conn.SetFlag(IoStreamConnectionFlagWriting, true)

			// Write the frame
			written := 0
			for written < len(data) {
				n, err := conn.conn.Write(data[written:])
				if err != nil {
					conn.SetFlag(IoStreamConnectionFlagWriting, false)
					// Write error, disconnect
					c.disconnectInternal(conn, false)
					return
				}
				written += n
			}

			conn.SetFlag(IoStreamConnectionFlagWriting, false)

			// Fire written callback - extract payload from frame for callback
			// Frame format: [hash:4][varint:N][payload]
			if len(data) > HashSize {
				payloadLen, headerSize, _ := TryUnpackFrameHeader(data)
				if headerSize > 0 && len(data) >= headerSize+int(payloadLen) {
					payload := data[headerSize : headerSize+int(payloadLen)]
					c.fireCallbackWithData(IoStreamCallbackEventTypeWritten, conn, 0, payload)
				}
			}
		}
	}
}

// createConnection creates a new IoStreamConnection wrapper.
func (c *IoStreamChannel) createConnection(conn net.Conn, addr *channel_utility.ChannelAddress, flag IoStreamConnectionFlag) *IoStreamConnection {
	ioConn := &IoStreamConnection{
		channel:           c,
		conn:              conn,
		address:           addr,
		status:            IoStreamConnectionStatusConnected,
		flags:             uint32(flag),
		readBufferManager: buffer.NewBufferManager(),
		frameReader:       NewFrameReader(DefaultReadBufferSize),
		writeQueue:        make(chan []byte, DefaultWriteQueueSize),
	}

	// Configure buffer manager
	if c.conf.ReceiveBufferMaxSize > 0 {
		ioConn.readBufferManager.SetLimit(int(c.conf.ReceiveBufferMaxSize), 0)
	}

	c.mu.Lock()
	c.connections[conn] = ioConn
	c.mu.Unlock()

	return ioConn
}

// disconnectInternal handles disconnecting a connection.
func (c *IoStreamChannel) disconnectInternal(conn *IoStreamConnection, isClosing bool) error_code.ErrorType {
	if conn.closed.Swap(true) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	conn.mu.Lock()
	conn.status = IoStreamConnectionStatusDisconnecting
	conn.flags |= uint32(IoStreamConnectionFlagClosing)
	conn.mu.Unlock()

	// Close the write queue
	close(conn.writeQueue)

	// Close the underlying connection
	if conn.conn != nil {
		conn.conn.Close()
	}

	// Remove from connections map (if not already done by readLoop)
	c.mu.Lock()
	delete(c.connections, conn.conn)
	c.mu.Unlock()

	return error_code.EN_ATBUS_ERR_SUCCESS
}

// resolveAddress converts a parsed address to network and address strings for net.Dial/Listen.
func (c *IoStreamChannel) resolveAddress(addr *channel_utility.ChannelAddress) (network string, address string) {
	scheme := strings.ToLower(addr.Scheme)

	switch scheme {
	case "atcp", "ipv4", "ipv6":
		if strings.Contains(addr.Host, ":") {
			return "tcp6", fmt.Sprintf("[%s]:%d", addr.Host, addr.Port)
		} else {
			return "tcp4", fmt.Sprintf("%s:%d", addr.Host, addr.Port)
		}
	case "dns":
		// DNS uses regular TCP, Go's net package handles DNS resolution
		return "tcp", fmt.Sprintf("%s:%d", addr.Host, addr.Port)
	case "unix", "pipe":
		return "unix", addr.Host
	default:
		return "", ""
	}
}

// setTCPOptions sets TCP socket options.
func (c *IoStreamChannel) setTCPOptions(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}

	if c.conf.NoDelay {
		tcpConn.SetNoDelay(true)
	}

	if c.conf.Keepalive > 0 {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(c.conf.Keepalive)
	}
}

// fireCallback fires a callback event.
func (c *IoStreamChannel) fireCallback(eventType IoStreamCallbackEventType, conn *IoStreamConnection, status int32) {
	c.fireCallbackWithData(eventType, conn, status, nil)
}

// fireCallbackWithData fires a callback event with data.
func (c *IoStreamChannel) fireCallbackWithData(eventType IoStreamCallbackEventType, conn *IoStreamConnection, status int32, data []byte) {
	if eventType < 0 || eventType >= IoStreamCallbackEventTypeMax {
		return
	}

	// Set in-callback flag
	c.mu.Lock()
	c.flags |= uint32(IoStreamChannelFlagInCallback)
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.flags &^= uint32(IoStreamChannelFlagInCallback)
		c.mu.Unlock()
	}()

	// Channel-level callback
	channelCallback := c.eventHandleSet.GetCallback(eventType)
	if channelCallback != nil {
		channelCallback(c, conn, status, data)
	}

	// Connection-level callback
	if conn != nil {
		connCallback := conn.eventHandleSet.GetCallback(eventType)
		if connCallback != nil {
			connCallback(c, conn, status, data)
		}
	}
}

// parseHostPort parses a host:port string.
func parseHostPort(addr string) (host string, port int) {
	// Handle IPv6 addresses like [::1]:8080
	if strings.HasPrefix(addr, "[") {
		closeBracket := strings.LastIndex(addr, "]")
		if closeBracket > 0 {
			host = addr[1:closeBracket]
			if len(addr) > closeBracket+2 && addr[closeBracket+1] == ':' {
				fmt.Sscanf(addr[closeBracket+2:], "%d", &port)
			}
			return
		}
	}

	// Regular host:port
	lastColon := strings.LastIndex(addr, ":")
	if lastColon > 0 {
		host = addr[:lastColon]
		fmt.Sscanf(addr[lastColon+1:], "%d", &port)
	} else {
		host = addr
	}
	return
}

// Ensure IoStreamChannel implements types.IoStreamChannel
var _ types.IoStreamChannel = (*IoStreamChannel)(nil)
