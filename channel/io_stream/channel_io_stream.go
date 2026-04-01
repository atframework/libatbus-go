package libatbus_channel_iostream

import (
	"context"
	"encoding/binary"
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
	// DataSmallSize matches C++ ATBUS_MACRO_DATA_SMALL_SIZE.
	// Small frames that fit within this size are parsed from a per-connection
	// static buffer without dynamic allocation.
	DataSmallSize = 7168
	// MessageMaxMergeSize, just like C++ ATBUS_MACRO_MESSAGE_MAX_MERGE_SIZE, is the maximum size of a single message that can be merged in the small-write merging logic.
	MessageMaxMergeSize = 64 * 1024
	// MergeBufferMaxSize is the maximum total size for small-write merging.
	// Matches C++ ATBUS_MACRO_TLS_MERGE_BUFFER_LEN.
	MergeBufferMaxSize = MessageMaxMergeSize - 256
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

	c.SetFlag(IoStreamChannelFlagClosing, true)

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
// Uses a two-phase approach matching C++ io_stream_on_recv_read_fn:
//   - Phase 1 (head): Small packets are parsed from a per-connection static buffer (readHead).
//   - Phase 2 (body): Large packets use a dynamically allocated buffer (readLargeFrame).
func (c *IoStreamChannel) readLoop(conn *IoStreamConnection) {
	defer func() {
		// Ensure the connection is fully torn down so writeLoop also exits
		c.disconnectInternal(conn, false)
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

		isFree := false
		incoming := readBuf[:n]

		// Feed incoming data to either the large frame buffer (body phase)
		// or the static head buffer (head phase).
		for len(incoming) > 0 {
			if conn.readLargeFrame != nil {
				// Phase 2 (body): feed data to the large frame buffer
				remaining := HashSize + conn.readLargeFrame.msgLen - conn.readLargeFrame.writePos
				copyLen := len(incoming)
				if copyLen > remaining {
					copyLen = remaining
				}
				copy(conn.readLargeFrame.buffer[conn.readLargeFrame.writePos:], incoming[:copyLen])
				conn.readLargeFrame.writePos += copyLen
				incoming = incoming[copyLen:]

				// Check if the large frame is complete
				if conn.readLargeFrame.writePos >= HashSize+conn.readLargeFrame.msgLen {
					if c.processCompleteLargeFrame(conn) {
						isFree = true
						break
					}
					conn.readLargeFrame = nil
				}
			} else {
				// Phase 1 (head): feed data to the static head buffer
				space := DataSmallSize - conn.readHead.len
				if space <= 0 {
					// Head buffer full but no frames could be parsed - data error
					isFree = true
					break
				}
				copyLen := len(incoming)
				if copyLen > space {
					copyLen = space
				}
				copy(conn.readHead.buffer[conn.readHead.len:], incoming[:copyLen])
				conn.readHead.len += copyLen
				incoming = incoming[copyLen:]

				// Parse complete frames from the head buffer
				if c.processReadHeadFrames(conn) {
					isFree = true
					break
				}
				// processReadHeadFrames may have set conn.readLargeFrame,
				// so loop back to feed remaining incoming data to the correct target.
			}
		}

		if isFree {
			return
		}
	}
}

// processReadHeadFrames parses complete frames from the per-connection static head buffer.
// Small frames (total framed size <= DataSmallSize) are dispatched directly from the head
// buffer. When a large frame header is detected (payload won't fit in the head buffer),
// a dynamic buffer is allocated and the transition to body phase is initiated.
// Returns true if the connection should be forcibly closed.
func (c *IoStreamChannel) processReadHeadFrames(conn *IoStreamConnection) bool {
	isFree := false
	buffStart := 0
	buffLeftLen := conn.readHead.len

	for buffLeftLen > HashSize {
		// Try to read varint payload length
		msgLen, vintSize := buffer.ReadVint(conn.readHead.buffer[buffStart+HashSize : buffStart+buffLeftLen])
		if vintSize == 0 {
			// Incomplete varint, need more data
			break
		}

		totalFrameSize := HashSize + vintSize + int(msgLen)

		if buffLeftLen >= totalFrameSize {
			// Complete frame in head buffer - process it directly
			payloadStart := buffStart + HashSize + vintSize
			payload := conn.readHead.buffer[payloadStart : payloadStart+int(msgLen)]

			// Verify hash
			expectedHash := binary.LittleEndian.Uint32(
				conn.readHead.buffer[buffStart : buffStart+HashSize])
			actualHash := CalculateHash(payload)

			if actualHash != expectedHash {
				atomic.AddUint64(&conn.checkHashFailedCount, 1)
				atomic.AddUint64(&c.statisticCheckHashFailedCount, 1)
				if conn.checkHashFailedCount > c.conf.MaxReadCheckHashFailedCount {
					isFree = true
				}
			} else if c.conf.ReceiveBufferLimitSize > 0 && msgLen > c.conf.ReceiveBufferLimitSize {
				atomic.AddUint64(&conn.checkBlockSizeFailedCount, 1)
				atomic.AddUint64(&c.statisticCheckBlockSizeFailedCount, 1)
				if conn.checkBlockSizeFailedCount > c.conf.MaxReadCheckBlockSizeFailedCount {
					isFree = true
				}
			} else {
				// Fire received callback.
				// The payload is a slice into the static head buffer and is valid
				// only for the duration of this synchronous callback.
				c.fireCallbackWithData(IoStreamCallbackEventTypeReceived, conn, 0, payload)
			}

			buffStart += totalFrameSize
			buffLeftLen -= totalFrameSize
		} else if totalFrameSize <= DataSmallSize {
			break
		} else {
			// Frame exceeds static head buffer capacity - use large frame path.
			// Check message size limit before allocating.
			if c.conf.ReceiveBufferLimitSize > 0 && msgLen > c.conf.ReceiveBufferLimitSize {
				atomic.AddUint64(&conn.checkBlockSizeFailedCount, 1)
				atomic.AddUint64(&c.statisticCheckBlockSizeFailedCount, 1)
				isFree = true
				break
			}

			// Allocate dynamic buffer: [hash:4][payload:msgLen]
			largeBufSize := HashSize + int(msgLen)
			largeBuf := make([]byte, largeBufSize)

			// Copy hash from head buffer
			copy(largeBuf[0:HashSize],
				conn.readHead.buffer[buffStart:buffStart+HashSize])

			// Copy remaining payload data from head buffer (data after the varint)
			remainingPayloadBytes := buffLeftLen - HashSize - vintSize
			if remainingPayloadBytes > 0 {
				copy(largeBuf[HashSize:],
					conn.readHead.buffer[buffStart+HashSize+vintSize:buffStart+buffLeftLen])
			}

			conn.readLargeFrame = &readLargeFrameState{
				buffer:   largeBuf,
				writePos: HashSize + remainingPayloadBytes,
				msgLen:   int(msgLen),
			}

			buffStart += buffLeftLen
			buffLeftLen = 0
			break
		}
	}

	// Compact: move leftover data to the front of the head buffer
	if buffStart > 0 && buffLeftLen > 0 {
		copy(conn.readHead.buffer[:], conn.readHead.buffer[buffStart:buffStart+buffLeftLen])
	}
	conn.readHead.len = buffLeftLen

	return isFree
}

// processCompleteLargeFrame processes a fully assembled large frame from the dynamic buffer.
// Returns true if the connection should be forcibly closed.
func (c *IoStreamChannel) processCompleteLargeFrame(conn *IoStreamConnection) bool {
	lf := conn.readLargeFrame
	if lf == nil {
		return false
	}

	payload := lf.buffer[HashSize : HashSize+lf.msgLen]

	// Verify hash
	expectedHash := binary.LittleEndian.Uint32(lf.buffer[0:HashSize])
	actualHash := CalculateHash(payload)

	if actualHash != expectedHash {
		atomic.AddUint64(&conn.checkHashFailedCount, 1)
		atomic.AddUint64(&c.statisticCheckHashFailedCount, 1)
		if conn.checkHashFailedCount > c.conf.MaxReadCheckHashFailedCount {
			return true
		}
		return false
	}

	if c.conf.ReceiveBufferLimitSize > 0 && uint64(lf.msgLen) > c.conf.ReceiveBufferLimitSize {
		atomic.AddUint64(&conn.checkBlockSizeFailedCount, 1)
		atomic.AddUint64(&c.statisticCheckBlockSizeFailedCount, 1)
		if conn.checkBlockSizeFailedCount > c.conf.MaxReadCheckBlockSizeFailedCount {
			return true
		}
		return false
	}

	// Fire received callback
	c.fireCallbackWithData(IoStreamCallbackEventTypeReceived, conn, 0, payload)
	return false
}

// writeLoop handles writing data to a connection.
// Implements small-write merging matching C++ io_stream_try_write:
// when multiple small frames are queued, they are merged into a single
// write syscall to reduce overhead.
func (c *IoStreamChannel) writeLoop(conn *IoStreamConnection) {
	defer func() {
		c.disconnectRun(conn)
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case data, ok := <-conn.writeQueue:
			if !ok {
				return
			}

			if conn.closed.Load() {
				return
			}

			// Try small-write merging: if the first frame is small and more are queued,
			// merge them into a single write (matches C++ io_stream_try_write merge logic)
			if len(data) <= DataSmallSize && len(conn.writeQueue) > 0 {
				frames := c.collectSmallWrites(conn, data)
				if len(frames) > 1 {
					if !c.writeMergedFramesAndNotify(conn, frames) {
						return
					}
				} else {
					if !c.writeFrameAndNotify(conn, data) {
						return
					}
				}
			} else {
				if !c.writeFrameAndNotify(conn, data) {
					return
				}
			}

			// if in disconnecting status and there is no more data to write, close it
			// (matches C++ io_stream_on_written_fn behavior)
			if conn.GetStatus() == IoStreamConnectionStatusDisconnecting {
				c.drainRemainingWrites(conn)
				return
			}
		case <-conn.disconnectCh:
			// Graceful disconnect signal: drain remaining writes then close
			c.drainRemainingWrites(conn)
			return
		}
	}
}

// writeFrameAndNotify writes a single frame to the connection and fires the written callback.
// Returns false on write error.
func (c *IoStreamChannel) writeFrameAndNotify(conn *IoStreamConnection, data []byte) bool {
	conn.SetFlag(IoStreamConnectionFlagWriting, true)
	payload := extractFramePayload(data)

	// Write the frame
	written := 0
	for written < len(data) {
		n, err := conn.conn.Write(data[written:])
		if err != nil {
			conn.SetFlag(IoStreamConnectionFlagWriting, false)
			c.fireCallbackWithData(IoStreamCallbackEventTypeWritten, conn, int32(error_code.EN_ATBUS_ERR_WRITE_FAILED), payload)
			return false
		}
		written += n
	}

	conn.SetFlag(IoStreamConnectionFlagWriting, false)

	// Fire written callback - extract payload from frame for callback
	// Frame format: [hash:4][varint:N][payload]
	c.fireCallbackWithData(IoStreamCallbackEventTypeWritten, conn, 0, payload)

	return true
}

// collectSmallWrites non-blocking drains additional frames from the write queue
// for merging into a single write. Returns at least the initial frame.
// Stops when the total size reaches MergeBufferMaxSize or the queue is empty.
func (c *IoStreamChannel) collectSmallWrites(conn *IoStreamConnection, first []byte) [][]byte {
	frames := [][]byte{first}
	totalSize := len(first)

	for totalSize < MergeBufferMaxSize {
		select {
		case data, ok := <-conn.writeQueue:
			if !ok {
				return frames
			}
			frames = append(frames, data)
			totalSize += len(data)
			if totalSize >= MergeBufferMaxSize {
				return frames
			}
		default:
			// No more data available
			return frames
		}
	}
	return frames
}

// writeMergedFramesAndNotify merges multiple frames into a single write syscall
// and fires written callbacks for each original frame afterward.
// This matches the C++ io_stream_try_write small-buffer merge optimization.
func (c *IoStreamChannel) writeMergedFramesAndNotify(conn *IoStreamConnection, frames [][]byte) bool {
	conn.SetFlag(IoStreamConnectionFlagWriting, true)

	// Calculate total size and merge into one buffer
	totalSize := 0
	for _, f := range frames {
		totalSize += len(f)
	}

	merged := make([]byte, totalSize)
	pos := 0
	for _, f := range frames {
		copy(merged[pos:], f)
		pos += len(f)
	}

	// Write merged buffer in one syscall
	written := 0
	for written < len(merged) {
		n, err := conn.conn.Write(merged[written:])
		if err != nil {
			conn.SetFlag(IoStreamConnectionFlagWriting, false)
			for _, frame := range frames {
				c.fireCallbackWithData(IoStreamCallbackEventTypeWritten, conn, int32(error_code.EN_ATBUS_ERR_WRITE_FAILED), extractFramePayload(frame))
			}
			return false
		}
		written += n
	}

	conn.SetFlag(IoStreamConnectionFlagWriting, false)

	// Fire written callbacks for each original frame
	for _, frame := range frames {
		c.fireCallbackWithData(IoStreamCallbackEventTypeWritten, conn, 0, extractFramePayload(frame))
	}

	return true
}

func extractFramePayload(frame []byte) []byte {
	if len(frame) <= HashSize {
		return nil
	}

	payloadLen, headerSize, _ := TryUnpackFrameHeader(frame)
	if headerSize <= 0 || len(frame) < headerSize+int(payloadLen) {
		return nil
	}

	return frame[headerSize : headerSize+int(payloadLen)]
}

// drainRemainingWrites drains all pending write data from the queue, writing each frame.
// This is called during graceful disconnect to flush buffered data before closing.
func (c *IoStreamChannel) drainRemainingWrites(conn *IoStreamConnection) {
	for {
		select {
		case data, ok := <-conn.writeQueue:
			if !ok {
				return
			}
			if conn.closed.Load() {
				return
			}
			if !c.writeFrameAndNotify(conn, data) {
				return
			}
		default:
			return
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
		writeQueue:        make(chan []byte, DefaultWriteQueueSize),
		disconnectCh:      make(chan struct{}),
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

// disconnectInternal initiates disconnection of a connection.
//
// When force=false (user's Disconnect(), readLoop exit), if a write is in progress
// (kWriting flag set), the actual shutdown is deferred — writeLoop will complete pending
// writes and then call disconnectRun. This matches C++ io_stream_disconnect_internal behavior.
//
// When force=true (channel Close()), shutdown proceeds immediately regardless of
// pending writes.
func (c *IoStreamChannel) disconnectInternal(conn *IoStreamConnection, force bool) error_code.ErrorType {
	if conn == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	conn.mu.Lock()
	if conn.status != IoStreamConnectionStatusConnected {
		conn.mu.Unlock()
		if force {
			// Force close even if already disconnecting
			return c.disconnectRun(conn)
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	conn.status = IoStreamConnectionStatusDisconnecting
	conn.mu.Unlock()

	// if there is any writing data, closing this connection later
	// (matches C++ io_stream_disconnect_internal: defer if kWriting && !force)
	if !force && conn.GetFlag(IoStreamConnectionFlagWriting) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	if !force {
		// Signal writeLoop to drain remaining writes and close gracefully
		conn.disconnectOnce.Do(func() {
			close(conn.disconnectCh)
		})
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	return c.disconnectRun(conn)
}

// disconnectRun performs the actual connection teardown.
// It is idempotent — protected by conn.closed atomic swap.
// This matches C++ io_stream_disconnect_run + io_stream_shutdown_connection + on_close callback.
func (c *IoStreamChannel) disconnectRun(conn *IoStreamConnection) error_code.ErrorType {
	if conn.closed.Swap(true) {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	conn.mu.Lock()
	conn.flags |= uint32(IoStreamConnectionFlagClosing)
	conn.mu.Unlock()

	// Close the write queue
	close(conn.writeQueue)

	// Close the underlying connection
	if conn.conn != nil {
		conn.conn.Close()
	}

	// Remove from connections map
	c.mu.Lock()
	delete(c.connections, conn.conn)
	c.mu.Unlock()

	conn.SetStatus(IoStreamConnectionStatusDisconnected)

	// Fire disconnected callback
	c.fireCallback(IoStreamCallbackEventTypeDisconnected, conn, 0)

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
	c.SetFlag(IoStreamChannelFlagInCallback, true)

	defer func() {
		c.SetFlag(IoStreamChannelFlagInCallback, false)
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
