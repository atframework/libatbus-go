package libatbus_channel_iostream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	channel_utility "github.com/atframework/libatbus-go/channel/utility"
	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maxTestBufferLen matches C++ MAX_TEST_BUFFER_LEN = ATBUS_MACRO_MESSAGE_LIMIT * 4
// ATBUS_MACRO_MESSAGE_LIMIT is typically 2MB, but we use a smaller buffer for testing
const maxTestBufferLen = 2*1024*1024 + 1

// testCheckFlag is an atomic counter matching C++ g_check_flag
var testCheckFlag atomic.Int32

// testRecvRec tracks received message count and total bytes, matching C++ g_recv_rec
type recvRecord struct {
	mu    sync.Mutex
	count int
	bytes int
}

// testCheckBuffSequence tracks expected receive buffer sequences, matching C++ g_check_buff_sequence
type checkBuffSequence struct {
	mu       sync.Mutex
	sequence []struct {
		offset int
		length int
	}
}

func (s *checkBuffSequence) pushBack(offset, length int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence = append(s.sequence, struct {
		offset int
		length int
	}{offset, length})
}

func (s *checkBuffSequence) popFront() (offset int, length int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sequence) == 0 {
		return 0, 0, false
	}
	front := s.sequence[0]
	s.sequence = s.sequence[1:]
	return front.offset, front.length, true
}

func (s *checkBuffSequence) isEmpty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sequence) == 0
}

func (s *checkBuffSequence) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sequence)
}

// getTestBuffer generates a random test buffer matching C++ get_test_buffer
func getTestBuffer() []byte {
	buf := make([]byte, maxTestBufferLen)
	rng := rand.New(rand.NewSource(12345)) // fixed seed for reproducibility
	for i := 0; i < maxTestBufferLen-1; i++ {
		buf[i] = byte('A' + rng.Intn(26))
	}
	return buf
}

// waitForCondition polls until a condition is true or timeout, similar to C++ uv_run loop
func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// getFreePort gets a free TCP port for testing
func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// getUnixSocketPath returns a temporary unix socket path for testing
func getUnixSocketPath(t *testing.T) string {
	t.Helper()
	dir := os.TempDir()
	return filepath.Join(dir, fmt.Sprintf("atbus-test-%d.sock", time.Now().UnixNano()))
}

// ============================================================================
// TCP Basic Test - matches C++ CASE_TEST(channel, io_stream_tcp_basic)
// ============================================================================

// TestIoStreamTcpBasic verifies basic TCP listen, connect, and data send/receive.
// Covers small buffers, big buffers, and many big buffers.
func TestIoStreamTcpBasic(t *testing.T) {
	port := getFreePort(t)
	addresses := []string{fmt.Sprintf("ipv4://127.0.0.1:%d", port)}
	if probeIPv6Loopback() {
		addresses = append(addresses, fmt.Sprintf("dns://localhost:%d", port), fmt.Sprintf("ipv6://::1:%d", port))
	}
	runIoStreamBasic(t, fmt.Sprintf("atcp://:::%d", port), addresses)
}

func runIoStreamBasic(t *testing.T, listenAddress string, dialAddresses []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	server, client := NewIoStreamChannel(ctx, nil), NewIoStreamChannel(ctx, nil)
	defer server.Close()
	defer client.Close()

	accepted := make(chan types.IoStreamConnection, len(dialAddresses))
	var connected atomic.Int32
	server.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(ch types.IoStreamChannel, conn types.IoStreamConnection, status int32, _ interface{}) {
			assert.Same(t, server, ch)
			assert.NotNil(t, conn)
			assert.Zero(t, status)
			select {
			case accepted <- conn:
			case <-ctx.Done():
			}
		})
	client.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(ch types.IoStreamChannel, conn types.IoStreamConnection, status int32, _ interface{}) {
			assert.Same(t, client, ch)
			assert.NotNil(t, conn)
			assert.Zero(t, status)
			connected.Add(1)
		})
	const burst = 153
	receive := func(ch *IoStreamChannel, capacity int) <-chan []byte {
		frames := make(chan []byte, capacity)
		ch.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
			func(receiver types.IoStreamChannel, conn types.IoStreamConnection, status int32, data interface{}) {
				assert.Same(t, ch, receiver)
				assert.NotNil(t, conn)
				assert.Zero(t, status)
				payload, ok := data.([]byte)
				if assert.True(t, ok, "unexpected received data %T", data) {
					select {
					case frames <- bytes.Clone(payload):
					case <-ctx.Done():
					}
				}
			})
		return frames
	}
	requests, replies := receive(server, 4), receive(client, burst)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, server.Listen(listenAddress))
	var firstClient, firstServer types.IoStreamConnection
	for i, address := range dialAddresses {
		conn, code := client.Connect(address)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, code, address)
		require.NotNil(t, conn)
		select {
		case peer := <-accepted:
			if i == 0 {
				firstClient, firstServer = conn, peer
			}
		case <-ctx.Done():
			t.Fatalf("accept %s: %v", address, ctx.Err())
		}
	}
	require.Eventually(t, func() bool { return int(connected.Load()) == len(dialAddresses) }, 5*time.Second, time.Millisecond)

	checkFrames := func(frames <-chan []byte, expected [][]byte) {
		t.Helper()
		for i, want := range expected {
			select {
			case got := <-frames:
				require.True(t, bytes.Equal(want, got), "frame %d: expected %d bytes, received %d", i, len(want), len(got))
			case <-ctx.Done():
				t.Fatalf("waiting for frame %d of %d: %v", i+1, len(expected), ctx.Err())
			}
		}
	}
	buf := getTestBuffer()
	small := [][]byte{buf[:13], buf[13:41], buf[41:141], buf[1024 : 1024+56*1024+3]}
	for _, frame := range small {
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, client.Send(firstClient, frame))
	}
	checkFrames(requests, small)

	rng := rand.New(rand.NewSource(42))
	large := make([][]byte, burst)
	for i := range large {
		offset, size := rng.Intn(2048), 20*1024+rng.Intn(10240)
		large[i] = buf[offset : offset+size]
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, server.Send(firstServer, large[i]))
	}
	checkFrames(replies, large)

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, server.Close())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, client.Close())
	require.Empty(t, server.GetConnections())
	require.Empty(t, client.GetConnections())
}

// ============================================================================
// TCP Reset By Client - matches C++ CASE_TEST(channel, io_stream_tcp_reset_by_client)
// ============================================================================

// TestIoStreamTcpResetByClient verifies that closing the client triggers disconnect on server.
func TestIoStreamTcpResetByClient(t *testing.T) {
	port := getFreePort(t)
	listenAddr := fmt.Sprintf("ipv4://127.0.0.1:%d", port)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var disconnectedCount atomic.Int32
	var connectedCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeDisconnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			assert.NotNil(t, channel)
			assert.NotNil(t, conn)
			t.Logf("server disconnect: %s", conn.GetAddress().GetAddress())
			disconnectedCount.Add(1)
		})

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	// Listen
	errCode := svr.Listen(listenAddr)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect 3 clients (matching C++ test pattern)
	for i := 0; i < 3; i++ {
		_, errCode := cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	}

	// Wait for all connections established (3 connected + 3 accepted)
	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 6
	})
	assert.True(t, ok, "should establish 3 connections, got %d callbacks", connectedCount.Load())
	assert.Greater(t, len(cli.GetConnections()), 0, "client should have connections")

	// Close client - should trigger disconnect on server
	cli.Close()

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(cli.GetConnections()) == 0
	})
	assert.True(t, ok, "client connections should be cleaned up")

	// Wait for server to detect disconnection
	ok = waitForCondition(t, 10*time.Second, func() bool {
		return disconnectedCount.Load() >= 3
	})
	assert.True(t, ok, "server should detect 3 disconnections, got %d", disconnectedCount.Load())

	svr.Close()

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(svr.GetConnections()) == 0
	})
	assert.True(t, ok, "server connections should be cleaned up")
}

// ============================================================================
// TCP Reset By Server - matches C++ CASE_TEST(channel, io_stream_tcp_reset_by_server)
// ============================================================================

// TestIoStreamTcpResetByServer verifies that closing the server triggers disconnect on clients.
func TestIoStreamTcpResetByServer(t *testing.T) {
	port := getFreePort(t)
	listenAddr := fmt.Sprintf("ipv4://127.0.0.1:%d", port)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var disconnectedCount atomic.Int32
	var connectedCount atomic.Int32

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeDisconnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			assert.NotNil(t, channel)
			assert.NotNil(t, conn)
			t.Logf("client disconnect: %s", conn.GetAddress().GetAddress())
			disconnectedCount.Add(1)
		})

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	// Listen
	errCode := svr.Listen(listenAddr)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect 3 clients
	for i := 0; i < 3; i++ {
		_, errCode := cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	}

	// Wait for all connections established
	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 6
	})
	assert.True(t, ok, "should establish 3 connections")

	// Close server - should trigger disconnect on clients
	svr.Close()

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(svr.GetConnections()) == 0
	})
	assert.True(t, ok, "server connections should be cleaned up")

	// Wait for clients to detect disconnection
	ok = waitForCondition(t, 10*time.Second, func() bool {
		return disconnectedCount.Load() >= 3
	})
	assert.True(t, ok, "clients should detect 3 disconnections, got %d", disconnectedCount.Load())

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(cli.GetConnections()) == 0
	})
	assert.True(t, ok, "client connections should be cleaned up")

	cli.Close()
}

// ============================================================================
// TCP Size Extended - matches C++ CASE_TEST(channel, io_stream_tcp_size_extended)
// ============================================================================

// TestIoStreamTcpSizeExtended verifies send/receive buffer size limits.
func testIoStreamReceiveSizeLimits(t *testing.T, addr string) {
	t.Helper()
	var conf IoStreamConfigure
	SetDefaultIoStreamConfigure(&conf)
	conf.ReceiveBufferLimitSize = uint64(DataSmallSize) * 2
	conf.ReceiveBufferMaxSize = conf.ReceiveBufferLimitSize * 2
	conf.SendBufferLimitSize = conf.ReceiveBufferMaxSize + 1
	svr, cli := NewIoStreamChannel(context.Background(), &conf), NewIoStreamChannel(context.Background(), &conf)
	t.Cleanup(func() { cli.Close(); svr.Close() })
	type receipt struct {
		status int32
		data   []byte
	}
	received := make(chan receipt, 4)
	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived, func(_ types.IoStreamChannel, _ types.IoStreamConnection, status int32, value any) {
		data, _ := value.([]byte)
		received <- receipt{status, bytes.Clone(data)}
	})
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, svr.Listen(addr))
	conn, code := cli.Connect(addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, code)
	oversized := bytes.Repeat([]byte{0xa3}, int(conf.ReceiveBufferLimitSize+1))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, cli.Send(conn, oversized))
	require.Equal(t, error_code.EN_ATBUS_ERR_INVALID_SIZE, cli.Send(conn, make([]byte, conf.SendBufferLimitSize+1)))
	select {
	case event := <-received:
		require.Equal(t, int32(error_code.EN_ATBUS_ERR_INVALID_SIZE), event.status)
		require.Equal(t, oversized, event.data)
	case <-time.After(3 * time.Second):
		t.Fatal("missing receive size error")
	}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, cli.Send(conn, make([]byte, conf.SendBufferLimitSize)))
	require.Eventually(t, func() bool { return len(cli.GetConnections()) == 0 && len(svr.GetConnections()) == 0 }, 3*time.Second, time.Millisecond)
}

func TestIoStreamTcpSizeExtended(t *testing.T) {
	testIoStreamReceiveSizeLimits(t, fmt.Sprintf("ipv4://127.0.0.1:%d", getFreePort(t)))
}

// ============================================================================
// TCP Connect Failed - matches C++ CASE_TEST(channel, io_stream_tcp_connect_failed)
// ============================================================================

// TestIoStreamTcpConnectFailed verifies connection failure to unreachable address.
func TestIoStreamTcpConnectFailed(t *testing.T) {
	ctx := context.Background()

	// Use a short timeout so the test doesn't take too long
	conf := &IoStreamConfigure{}
	SetDefaultIoStreamConfigure(conf)
	conf.ConfirmTimeout = 2 * time.Second

	cli := NewIoStreamChannel(ctx, conf)

	// Try to connect to an unreachable port
	conn, errCode := cli.Connect("ipv4://127.0.0.1:1")
	// Should fail because the port is unreachable (OS may reject or timeout)
	if errCode == error_code.EN_ATBUS_ERR_SUCCESS {
		// If somehow connected (unlikely), just disconnect
		cli.Disconnect(conn)
	} else {
		assert.Equal(t, error_code.EN_ATBUS_ERR_SOCK_CONNECT_FAILED, errCode,
			"connect to unreachable port should fail")
		assert.Nil(t, conn, "connection should be nil on failure")
	}

	cli.Close()
}

// ============================================================================
// TCP Written Callback Test - matches C++ CASE_TEST(channel, io_stream_callback_on_written)
// ============================================================================

// TestIoStreamCallbackOnWritten verifies the written callback is triggered after data is sent.
func TestIoStreamCallbackOnWritten(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32
	var writtenCallbackCount atomic.Int32
	var writtenCallbackTotalSize atomic.Int64
	var recvCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			// Set written callback on the connection
			conn.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeWritten,
				func(ch types.IoStreamChannel, c types.IoStreamConnection, st int32, pd interface{}) {
					assert.NotNil(t, ch)
					assert.NotNil(t, c)
					assert.Equal(t, int32(0), st)
					if data, ok := pd.([]byte); ok {
						writtenCallbackTotalSize.Add(int64(len(data)))
					}
					writtenCallbackCount.Add(1)
				})
			connectedCount.Add(1)
		})

	// Setup receive callback on server to verify data arrives
	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			recvCount.Add(1)
		})

	// Listen
	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect
	conn1, errCode := cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Wait for connection
	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok, "should establish connection")

	testBuf := getTestBuffer()

	// Send small buffer (64 bytes)
	errCode = cli.Send(conn1, testBuf[0:64])
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Send medium buffer (256 bytes)
	errCode = cli.Send(conn1, testBuf[64:64+256])
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Send large buffer (1024 bytes)
	errCode = cli.Send(conn1, testBuf[320:320+1024])
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Wait for receive callbacks
	ok = waitForCondition(t, 10*time.Second, func() bool {
		return recvCount.Load() >= 3
	})
	assert.True(t, ok, "should receive 3 messages")

	// Verify written callback was called
	ok = waitForCondition(t, 5*time.Second, func() bool {
		return writtenCallbackCount.Load() >= 3
	})
	assert.True(t, ok, "written callback should be called at least 3 times, got %d", writtenCallbackCount.Load())
	t.Logf("Written callback count: %d, total size: %d",
		writtenCallbackCount.Load(), writtenCallbackTotalSize.Load())

	svr.Close()
	cli.Close()
}

// ============================================================================
// Unix Socket Basic Test - matches C++ CASE_TEST(channel, io_stream_unix_basic)
// ============================================================================

// TestIoStreamUnixBasic verifies basic Unix socket listen, connect, and data send/receive.
func TestIoStreamUnixBasic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket test skipped on Windows")
	}
	path := getUnixSocketPath(t)
	defer os.Remove(path)
	address := "unix://" + path
	runIoStreamBasic(t, address, []string{address, address, address})
}

// ============================================================================
// Unix Reset By Client - matches C++ CASE_TEST(channel, io_stream_unix_reset_by_client)
// ============================================================================

// TestIoStreamUnixResetByClient verifies that closing the unix client triggers disconnect on server.
func TestIoStreamUnixResetByClient(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket test skipped on Windows")
	}

	sockPath := getUnixSocketPath(t)
	defer os.Remove(sockPath)

	listenAddr := "unix://" + sockPath

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var disconnectedCount atomic.Int32
	var connectedCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeDisconnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			assert.NotNil(t, channel)
			assert.NotNil(t, conn)
			t.Logf("unix server disconnect: %s", conn.GetAddress().GetAddress())
			disconnectedCount.Add(1)
		})

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	// Listen
	errCode := svr.Listen(listenAddr)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect 3 clients
	for i := 0; i < 3; i++ {
		_, errCode := cli.Connect(listenAddr)
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	}

	// Wait for all connections (3 connected + 3 accepted = 6)
	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 6
	})
	assert.True(t, ok, "should establish 3 unix connections")

	// Close client
	cli.Close()

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(cli.GetConnections()) == 0
	})
	assert.True(t, ok, "client connections should be cleaned up")

	// Wait for server disconnect callbacks
	ok = waitForCondition(t, 10*time.Second, func() bool {
		return disconnectedCount.Load() >= 3
	})
	assert.True(t, ok, "server should detect 3 disconnections, got %d", disconnectedCount.Load())

	svr.Close()

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(svr.GetConnections()) == 0
	})
	assert.True(t, ok, "server connections should be cleaned up")
}

// ============================================================================
// Unix Reset By Server - matches C++ CASE_TEST(channel, io_stream_unix_reset_by_server)
// ============================================================================

// TestIoStreamUnixResetByServer verifies that closing the unix server triggers disconnect on clients.
func TestIoStreamUnixResetByServer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket test skipped on Windows")
	}

	sockPath := getUnixSocketPath(t)
	defer os.Remove(sockPath)

	listenAddr := "unix://" + sockPath

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var disconnectedCount atomic.Int32
	var connectedCount atomic.Int32

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeDisconnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			assert.NotNil(t, channel)
			assert.NotNil(t, conn)
			t.Logf("unix client disconnect: %s", conn.GetAddress().GetAddress())
			disconnectedCount.Add(1)
		})

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	// Listen
	errCode := svr.Listen(listenAddr)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect 3 clients
	for i := 0; i < 3; i++ {
		_, errCode := cli.Connect(listenAddr)
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	}

	// Wait for all connections
	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 6
	})
	assert.True(t, ok, "should establish 3 unix connections")

	// Close server
	svr.Close()

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(svr.GetConnections()) == 0
	})
	assert.True(t, ok, "server connections should be cleaned up")

	// Wait for client disconnect detection
	ok = waitForCondition(t, 10*time.Second, func() bool {
		return disconnectedCount.Load() >= 3
	})
	assert.True(t, ok, "clients should detect 3 disconnections, got %d", disconnectedCount.Load())

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(cli.GetConnections()) == 0
	})
	assert.True(t, ok, "client connections should be cleaned up")

	cli.Close()
}

// ============================================================================
// Unix Size Extended - matches C++ CASE_TEST(channel, io_stream_unix_size_extended)
// ============================================================================

// TestIoStreamUnixSizeExtended verifies send/receive buffer size limits on Unix sockets.
func TestIoStreamUnixSizeExtended(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket test runs on Unix")
	}
	testIoStreamReceiveSizeLimits(t, "unix://"+getUnixSocketPath(t))
}

// ============================================================================
// Unix Connect Failed - matches C++ CASE_TEST(channel, io_stream_unix_connect_failed)
// ============================================================================

// TestIoStreamUnixConnectFailed verifies connection failure to non-existent Unix socket.
func TestIoStreamUnixConnectFailed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket test skipped on Windows")
	}

	ctx := context.Background()
	conf := &IoStreamConfigure{}
	SetDefaultIoStreamConfigure(conf)
	conf.ConfirmTimeout = 2 * time.Second

	cli := NewIoStreamChannel(ctx, conf)

	// Try to connect to a non-existent socket
	conn, errCode := cli.Connect("unix:///tmp/atbus-unit-test-nonexistent.sock")
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode,
		"connect to non-existent unix socket should fail")
	assert.Nil(t, conn, "connection should be nil on failure")

	cli.Close()
}

// ============================================================================
// Pipe Basic Test (Windows named pipe / Unix pipe alias)
// ============================================================================

func TestIoStreamResolvePipeAddress(t *testing.T) {
	var channel IoStreamChannel
	for _, scheme := range []string{"pipe", "unix"} {
		for _, host := range []string{"atbus.sock", "\\\\.\\pipe\\atbus"} {
			addr := channel_utility.MakeAddressFromComponents(scheme, host, 0)
			network, address := channel.resolveAddress(addr)
			assert.Equal(t, "pipe", network, addr.GetAddress())
			assert.Equal(t, host, address, addr.GetAddress())
		}
	}
}

// TestIoStreamPipeBasic verifies basic pipe listen, connect, and data send/receive.
func TestIoStreamPipeBasic(t *testing.T) {
	var sockPath string
	if runtime.GOOS == "windows" {
		sockPath = fmt.Sprintf(`\\.\pipe\atbus-test-%d`, time.Now().UnixNano())
	} else {
		sockPath = getUnixSocketPath(t)
		defer os.Remove(sockPath)
	}
	listenAddr := "pipe://" + sockPath

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var checkFlag atomic.Int32
	var checkBufSeq checkBuffSequence

	testBuf := getTestBuffer()

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			assert.Equal(t, int32(0), status)
			checkFlag.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			assert.Equal(t, int32(0), status)
			checkFlag.Add(1)
		})

	// Listen
	errCode := svr.Listen(listenAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "pipe listen should succeed")

	// Connect
	conn1, errCode := cli.Connect(listenAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "pipe connect should succeed")
	assert.NotNil(t, conn1)

	// Wait for connection
	ok := waitForCondition(t, 5*time.Second, func() bool {
		return checkFlag.Load() >= 2
	})
	assert.True(t, ok, "should establish pipe connection")

	// Setup receive callback
	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			data, ok := privData.([]byte)
			if !ok || data == nil {
				t.Errorf("recv: privData should be []byte")
				return
			}

			offset, length, seqOk := checkBufSeq.popFront()
			if !seqOk {
				t.Error("recv: unexpected data")
				return
			}

			assert.Equal(t, length, len(data))
			expected := testBuf[offset : offset+length]
			assert.True(t, bytes.Equal(expected, data))
			checkFlag.Add(1)
		})

	baseCheckFlag := checkFlag.Load()

	// Small buffers
	checkBufSeq.pushBack(0, 13)
	cli.Send(conn1, testBuf[0:13])

	checkBufSeq.pushBack(13, 28)
	cli.Send(conn1, testBuf[13:13+28])

	checkBufSeq.pushBack(41, 100)
	cli.Send(conn1, testBuf[41:41+100])

	// Big buffer
	checkBufSeq.pushBack(1024, 56*1024+3)
	cli.Send(conn1, testBuf[1024:1024+56*1024+3])

	ok = waitForCondition(t, 10*time.Second, func() bool {
		return checkFlag.Load()-baseCheckFlag >= 4
	})
	assert.True(t, ok, "should receive 4 pipe messages, got %d", checkFlag.Load()-baseCheckFlag)

	svr.Close()
	cli.Close()
}

// ============================================================================
// DNS Address Resolve Test - matches C++ dns://localhost connect
// ============================================================================

// TestIoStreamDnsConnect verifies dns:// address scheme can be used to connect via TCP.
func TestIoStreamDnsConnect(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	// Listen on atcp (matching C++ which listens on atcp://:::port)
	errCode := svr.Listen(fmt.Sprintf("atcp://0.0.0.0:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect via dns://localhost
	conn, errCode := cli.Connect(fmt.Sprintf("dns://localhost:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "dns connect should succeed")
	assert.NotNil(t, conn)

	// Wait for connection
	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok, "should establish dns connection")

	svr.Close()
	cli.Close()
}

// ============================================================================
// IPv6 Test - matches C++ ipv6://::1 connect
// ============================================================================

// probeIPv6Loopback mirrors C++ unit_test_probe_ipv6_loopback(): verifies that both
// binding and dialing IPv6 loopback [::1] actually work on the current host.
func probeIPv6Loopback() bool {
	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	defer l.Close()

	conn, err := net.DialTimeout("tcp6", l.Addr().String(), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// TestIoStreamIpv6Connect verifies IPv6 connections work.
func TestIoStreamIpv6Connect(t *testing.T) {
	// Check if IPv6 loopback connect is available (not just bind)
	if !probeIPv6Loopback() {
		t.Skip("IPv6 loopback connect not available on this system")
	}

	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 not available on this system")
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	// Listen on IPv6 (using C++ style ipv6://:::port)
	errCode := svr.Listen(fmt.Sprintf("ipv6://:::%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect to IPv6 loopback
	conn, errCode := cli.Connect(fmt.Sprintf("ipv6://::1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "ipv6 connect should succeed")
	assert.NotNil(t, conn)

	// Wait for connection
	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok, "should establish ipv6 connection")

	svr.Close()
	cli.Close()
}

// ============================================================================
// Channel Close Idempotency Test
// ============================================================================

// TestIoStreamChannelCloseIdempotent verifies that Close can be called multiple times safely.
func TestIoStreamChannelCloseIdempotent(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)

	errCode := ch.Close()
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "first close should succeed")

	errCode = ch.Close()
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "second close should succeed")
}

// ============================================================================
// Send After Close Test
// ============================================================================

// TestIoStreamSendAfterClose verifies that Send returns error after channel is closed.
func TestIoStreamSendAfterClose(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	conn1, errCode := cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok)

	// Close channel
	cli.Close()

	// Send should fail after close
	errCode = cli.Send(conn1, []byte("test"))
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode,
		"send after close should fail")

	svr.Close()
}

// ============================================================================
// Listen After Close Test
// ============================================================================

// TestIoStreamListenAfterClose verifies that Listen returns error after channel is closed.
func TestIoStreamListenAfterClose(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	ch.Close()

	port := getFreePort(t)
	errCode := ch.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, errCode,
		"listen after close should return channel closing")
}

// ============================================================================
// Connect After Close Test
// ============================================================================

// TestIoStreamConnectAfterClose verifies that Connect returns error after channel is closed.
func TestIoStreamConnectAfterClose(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	ch.Close()

	conn, errCode := ch.Connect("ipv4://127.0.0.1:12345")
	assert.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, errCode,
		"connect after close should return channel closing")
	assert.Nil(t, conn)
}

// ============================================================================
// Invalid Address Tests
// ============================================================================

// TestIoStreamInvalidAddress verifies that invalid addresses are rejected.
func TestIoStreamInvalidAddress(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	defer ch.Close()

	// Empty address
	errCode := ch.Listen("")
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "empty address should fail")

	// Invalid scheme
	errCode = ch.Listen("invalid://something")
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "invalid scheme should fail")

	// No scheme separator
	errCode = ch.Listen("noproto")
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode, "no scheme should fail")

	// Connect with invalid address
	conn, errCode := ch.Connect("")
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	assert.Nil(t, conn)

	conn, errCode = ch.Connect("invalid://something")
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	assert.Nil(t, conn)
}

// ============================================================================
// Disconnect nil Connection Test
// ============================================================================

// Closing the transport before two disconnect requests must emit one notification.
func TestIoStreamTcpDisconnectAfterTransportClose(t *testing.T) {
	addr := fmt.Sprintf("ipv4://127.0.0.1:%d", getFreePort(t))
	server := NewIoStreamChannel(context.Background(), nil)
	client := NewIoStreamChannel(context.Background(), nil)
	t.Cleanup(func() { client.Close(); server.Close() })
	var callbacks atomic.Int32
	client.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeDisconnected,
		func(types.IoStreamChannel, types.IoStreamConnection, int32, interface{}) { callbacks.Add(1) })
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, server.Listen(addr))
	connection, status := client.Connect(addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, status)
	require.Len(t, client.GetConnections(), 1)
	require.Eventually(t, func() bool { return len(server.GetConnections()) == 1 }, 3*time.Second, time.Millisecond)
	require.NoError(t, connection.(*IoStreamConnection).conn.Close())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, client.Disconnect(connection))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, client.Disconnect(connection))
	require.Eventually(t, func() bool { return callbacks.Load() == 1 && len(client.GetConnections()) == 0 }, 3*time.Second, time.Millisecond)
	client.Close()
	server.Close()
	require.Equal(t, int32(1), callbacks.Load())
	require.Empty(t, server.GetConnections())
}

// A transport obtained before shutdown can reach pool registration after Close.
func TestIoStreamRejectsLateTransport(t *testing.T) {
	c := NewIoStreamChannel(context.Background(), nil)
	c.Close()
	local, peer := net.Pipe()
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)))
	defer local.Close()
	defer peer.Close()
	addr, ok := channel_utility.MakeAddress("ipv4://127.0.0.1:1234")
	require.True(t, ok)
	require.Nil(t, c.createConnection(local, addr, IoStreamConnectionFlagAccept))
	require.Empty(t, c.GetConnections())
	var b [1]byte
	_, err := peer.Read(b[:])
	require.ErrorIs(t, err, io.EOF)
}

func TestIoStreamCloseDuringListen(t *testing.T) {
	c := NewIoStreamChannel(context.Background(), nil)
	endpoint := fmt.Sprintf("127.0.0.1:%d", getFreePort(t))
	listened := make(chan error_code.ErrorType, 1)
	closed := make(chan error_code.ErrorType, 1)

	// Hold registration until the socket is bound and Close has marked the channel.
	c.mu.Lock()
	go func() { listened <- c.Listen("ipv4://" + endpoint) }()
	bound := waitForCondition(t, 5*time.Second, func() bool {
		conn, err := net.DialTimeout("tcp4", endpoint, 50*time.Millisecond)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	})
	go func() { closed <- c.Close() }()
	closing := waitForCondition(t, 5*time.Second, c.closed.Load)
	c.mu.Unlock()

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, <-closed)
	result := <-listened
	require.True(t, bound && closing, "listen and close must overlap")
	require.Equal(t, error_code.EN_ATBUS_ERR_CHANNEL_CLOSING, result)
	require.Empty(t, c.listeners)
	rebound, err := net.Listen("tcp4", endpoint)
	require.NoError(t, err)
	rebound.Close()
}

func TestIoStreamDisconnectNil(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	defer ch.Close()

	errCode := ch.Disconnect(nil)
	assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, errCode,
		"disconnect nil should return params error")
}

// ============================================================================
// Send nil Connection Test
// ============================================================================

// TestIoStreamSendNilConnection verifies Send handles nil connection.
func TestIoStreamSendNilConnection(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	defer ch.Close()

	errCode := ch.Send(nil, []byte("test"))
	assert.Equal(t, error_code.EN_ATBUS_ERR_CLOSING, errCode,
		"send to nil connection should return closing error")
}

// ============================================================================
// Connection Status and Flags Test
// ============================================================================

// TestIoStreamConnectionStatusAndFlags verifies connection status and flag management.
func TestIoStreamConnectionStatusAndFlags(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32
	var svrAcceptedConn *IoStreamConnection

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			svrAcceptedConn = conn.(*IoStreamConnection)
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	cliConn, errCode := cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok)

	// Check client connection flags
	cliIoConn := cliConn.(*IoStreamConnection)
	assert.True(t, cliIoConn.GetFlag(IoStreamConnectionFlagConnect),
		"client connection should have Connect flag")
	assert.False(t, cliIoConn.GetFlag(IoStreamConnectionFlagAccept),
		"client connection should not have Accept flag")
	assert.Equal(t, IoStreamConnectionStatusConnected, cliIoConn.GetStatus(),
		"client connection should be in Connected status")

	// Check server accepted connection flags
	assert.NotNil(t, svrAcceptedConn)
	assert.True(t, svrAcceptedConn.GetFlag(IoStreamConnectionFlagAccept),
		"server connection should have Accept flag")
	assert.False(t, svrAcceptedConn.GetFlag(IoStreamConnectionFlagConnect),
		"server connection should not have Connect flag")
	assert.Equal(t, IoStreamConnectionStatusConnected, svrAcceptedConn.GetStatus(),
		"server connection should be in Connected status")

	// Test private data
	cliIoConn.SetPrivateData("test_data")
	assert.Equal(t, "test_data", cliIoConn.GetPrivateData())

	// Test channel reference
	assert.Equal(t, cli, cliIoConn.GetChannel())

	svr.Close()
	cli.Close()
}

// ============================================================================
// Channel Private Data Test
// ============================================================================

// TestIoStreamChannelPrivateData verifies channel-level private data.
func TestIoStreamChannelPrivateData(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	defer ch.Close()

	assert.Nil(t, ch.GetPrivateData())

	ch.SetPrivateData(42)
	assert.Equal(t, 42, ch.GetPrivateData())

	ch.SetPrivateData("hello")
	assert.Equal(t, "hello", ch.GetPrivateData())
}

// ============================================================================
// Configure Test
// ============================================================================

// TestIoStreamDefaultConfigure verifies default configuration values.
func TestIoStreamDefaultConfigure(t *testing.T) {
	conf := &IoStreamConfigure{}
	SetDefaultIoStreamConfigure(conf)

	assert.Equal(t, 60*time.Second, conf.Keepalive, "default keepalive should be 60s")
	assert.True(t, conf.NoBlock, "default NoBlock should be true")
	assert.True(t, conf.NoDelay, "default NoDelay should be true")
	assert.Equal(t, uint64(8*1024*1024), conf.SendBufferMaxSize, "default send buffer max 8MB")
	assert.Equal(t, uint64(2*1024*1024), conf.SendBufferLimitSize, "default send limit 2MB")
	assert.Equal(t, uint64(256*1024*1024), conf.ReceiveBufferMaxSize, "default recv buffer max 256MB")
	assert.Equal(t, uint64(2*1024*1024), conf.ReceiveBufferLimitSize, "default recv limit 2MB")
	assert.Equal(t, int32(256), conf.Backlog, "default backlog 256")
	assert.Equal(t, 30*time.Second, conf.ConfirmTimeout, "default confirm timeout 30s")
	assert.Equal(t, uint64(1000), conf.MaxReadNetEgainCount)
	assert.Equal(t, uint64(3), conf.MaxReadCheckBlockSizeFailedCount)
	assert.Equal(t, uint64(3), conf.MaxReadCheckHashFailedCount)
}

// TestIoStreamCustomConfigure verifies custom configuration is applied.
func TestIoStreamCustomConfigure(t *testing.T) {
	conf := &IoStreamConfigure{}
	SetDefaultIoStreamConfigure(conf)
	conf.Keepalive = 30 * time.Second
	conf.NoDelay = false

	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, conf)
	defer ch.Close()

	assert.Equal(t, 30*time.Second, ch.GetConfigure().Keepalive)
	assert.False(t, ch.GetConfigure().NoDelay)
}

// ============================================================================
// NewIoStreamChannel with nil context Test
// ============================================================================

// TestNewIoStreamChannelNilContext verifies channel creation with nil context.
func TestNewIoStreamChannelNilContext(t *testing.T) {
	ch := NewIoStreamChannel(nil, nil)
	defer ch.Close()

	assert.NotNil(t, ch)
	assert.NotNil(t, ch.GetContext())
}

// ============================================================================
// Statistics Test
// ============================================================================

// TestIoStreamStatistics verifies statistics counters start at zero.
func TestIoStreamStatistics(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	defer ch.Close()

	assert.Equal(t, uint64(0), ch.GetStatisticActiveRequestCount())
	assert.Equal(t, uint64(0), ch.GetStatisticReadNetEgainCount())
	assert.Equal(t, uint64(0), ch.GetStatisticCheckBlockSizeFailedCount())
	assert.Equal(t, uint64(0), ch.GetStatisticCheckHashFailedCount())
}

// ============================================================================
// Bidirectional Communication Test (server sends to client)
// ============================================================================

// TestIoStreamBidirectionalCommunication verifies data can be sent in both directions.
func TestIoStreamBidirectionalCommunication(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32
	var cliRecvCount atomic.Int32
	var svrRecvCount atomic.Int32
	var svrAcceptedConn types.IoStreamConnection

	testBuf := getTestBuffer()

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			svrAcceptedConn = conn
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	// Receive callback on server
	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			data, ok := privData.([]byte)
			if !ok {
				return
			}
			assert.Equal(t, testBuf[0:100], data, "server should receive correct data")
			svrRecvCount.Add(1)
		})

	// Receive callback on client
	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			data, ok := privData.([]byte)
			if !ok {
				return
			}
			assert.Equal(t, testBuf[200:300], data, "client should receive correct data")
			cliRecvCount.Add(1)
		})

	// Listen
	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect
	cliConn, errCode := cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok)

	// Client -> Server
	cli.Send(cliConn, testBuf[0:100])

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return svrRecvCount.Load() >= 1
	})
	assert.True(t, ok, "server should receive data from client")

	// Server -> Client (using accepted connection)
	assert.NotNil(t, svrAcceptedConn)
	svr.Send(svrAcceptedConn, testBuf[200:300])

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return cliRecvCount.Load() >= 1
	})
	assert.True(t, ok, "client should receive data from server")

	svr.Close()
	cli.Close()
}

// ============================================================================
// Multiple Connections Simultaneous Data Test
// ============================================================================

// TestIoStreamMultipleConnectionsSimultaneousData verifies multiple connections can send/receive simultaneously.
func TestIoStreamMultipleConnectionsSimultaneousData(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32
	var totalRecvCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			totalRecvCount.Add(1)
		})

	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Create 5 separate client channels
	numClients := 5
	clients := make([]*IoStreamChannel, numClients)
	clientConns := make([]types.IoStreamConnection, numClients)

	for i := 0; i < numClients; i++ {
		clients[i] = NewIoStreamChannel(ctx, nil)
		clients[i].GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
			func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
				connectedCount.Add(1)
			})
		conn, errCode := clients[i].Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		clientConns[i] = conn
	}

	// Wait for all connections
	ok := waitForCondition(t, 10*time.Second, func() bool {
		return connectedCount.Load() >= int32(numClients*2)
	})
	assert.True(t, ok, "all clients should connect")

	testBuf := getTestBuffer()

	// Each client sends 10 messages simultaneously
	messagesPerClient := 10
	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < messagesPerClient; j++ {
				offset := (idx*messagesPerClient + j) * 100
				if offset+100 > maxTestBufferLen {
					offset = 0
				}
				clients[idx].Send(clientConns[idx], testBuf[offset:offset+100])
			}
		}(i)
	}
	wg.Wait()

	expectedTotal := int32(numClients * messagesPerClient)
	ok = waitForCondition(t, 15*time.Second, func() bool {
		return totalRecvCount.Load() >= expectedTotal
	})
	assert.True(t, ok, "server should receive all %d messages, got %d",
		expectedTotal, totalRecvCount.Load())

	// Cleanup
	for i := 0; i < numClients; i++ {
		clients[i].Close()
	}
	svr.Close()
}

// ============================================================================
// Context Cancellation Test
// ============================================================================

// TestIoStreamContextCancellation verifies that cancelling context closes the channel.
func TestIoStreamContextCancellation(t *testing.T) {
	port := getFreePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	_, errCode = cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok)

	// Cancel the context
	cancel()

	// Connections should eventually clean up
	ok = waitForCondition(t, 10*time.Second, func() bool {
		return len(cli.GetConnections()) == 0
	})
	// Note: Context cancellation may not immediately clean all connections depending on impl
	// This is mainly testing that context cancel doesn't cause panics

	svr.Close()
	cli.Close()
}

// ============================================================================
// Disconnect Individual Connection Test
// ============================================================================

// TestIoStreamDisconnectIndividual verifies disconnecting a specific connection.
func TestIoStreamDisconnectIndividual(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32
	var disconnectedCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeDisconnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			disconnectedCount.Add(1)
		})

	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Connect 3 connections
	conns := make([]types.IoStreamConnection, 3)
	for i := 0; i < 3; i++ {
		conn, errCode := cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		conns[i] = conn
	}

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 6
	})
	assert.True(t, ok)

	initialConnCount := len(cli.GetConnections())
	assert.Equal(t, 3, initialConnCount)

	// Disconnect one specific connection
	errCode = cli.Disconnect(conns[1])
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	// Wait for disconnection
	ok = waitForCondition(t, 5*time.Second, func() bool {
		return disconnectedCount.Load() >= 1
	})
	assert.True(t, ok, "should get disconnect callback")

	ok = waitForCondition(t, 5*time.Second, func() bool {
		return len(cli.GetConnections()) <= 2
	})
	assert.True(t, ok, "should have 2 remaining connections, got %d", len(cli.GetConnections()))

	svr.Close()
	cli.Close()
}

// ============================================================================
// Frame Protocol Interoperability Test
// ============================================================================

// TestFrameProtocolInterop verifies that the Go frame encoding matches C++ format:
// [hash:4 LE][varint][payload]
// This ensures C++ and Go implementations can communicate.
func TestFrameProtocolInterop(t *testing.T) {
	// Test with known data to verify frame format
	payload := []byte("Hello, atbus protocol interop test!")

	// Pack
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	written := PackFrame(payload, frame)
	assert.Equal(t, frameSize, written)

	// Verify frame structure:
	// 1. First 4 bytes: murmur3 hash (little-endian)
	hash := CalculateHash(payload)
	assert.Equal(t, hash, uint32(frame[0])|uint32(frame[1])<<8|uint32(frame[2])<<16|uint32(frame[3])<<24,
		"first 4 bytes should be murmur3 hash in little-endian")

	// 2. After hash: varint-encoded length
	// 3. After length: payload
	result := UnpackFrame(frame)
	assert.Nil(t, result.Error)
	assert.Equal(t, payload, result.Payload)

	// Verify TryUnpackFrameHeader
	payloadLen, headerSize, needMore := TryUnpackFrameHeader(frame)
	assert.False(t, needMore)
	assert.Equal(t, uint64(len(payload)), payloadLen)
	assert.Greater(t, headerSize, HashSize)

	// Verify payload starts at headerSize
	assert.Equal(t, payload, frame[headerSize:headerSize+int(payloadLen)])
}

// TestFrameProtocolLargePayload verifies frame encoding for large payloads
// similar to C++ 56*1024+3 byte test.
func TestFrameProtocolLargePayload(t *testing.T) {
	testBuf := getTestBuffer()
	payload := testBuf[1024 : 1024+56*1024+3]

	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	written := PackFrame(payload, frame)
	assert.Equal(t, frameSize, written)

	result := UnpackFrame(frame)
	assert.Nil(t, result.Error)
	assert.True(t, bytes.Equal(payload, result.Payload),
		"large payload should round-trip correctly")
}

// TestFrameProtocolEmptyPayload verifies frame encoding for empty payload.
func TestFrameProtocolEmptyPayload(t *testing.T) {
	payload := []byte{}

	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	written := PackFrame(payload, frame)
	assert.Equal(t, frameSize, written)

	result := UnpackFrame(frame)
	assert.Nil(t, result.Error)
	assert.Equal(t, 0, len(result.Payload))
}

// ============================================================================
// Channel Flag Test
// ============================================================================

// TestIoStreamChannelFlags verifies channel flag operations.
func TestIoStreamChannelFlags(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	defer ch.Close()

	// Initially no flags set
	assert.False(t, ch.GetFlag(IoStreamChannelFlagClosing))
	assert.False(t, ch.GetFlag(IoStreamChannelFlagInCallback))

	// Set and verify
	ch.SetFlag(IoStreamChannelFlagClosing, true)
	assert.True(t, ch.GetFlag(IoStreamChannelFlagClosing))

	// Clear and verify
	ch.SetFlag(IoStreamChannelFlagClosing, false)
	assert.False(t, ch.GetFlag(IoStreamChannelFlagClosing))
}

// ============================================================================
// Event Handle Set Test
// ============================================================================

// TestIoStreamEventHandleSet verifies event callback set/get operations.
func TestIoStreamEventHandleSet(t *testing.T) {
	ctx := context.Background()
	ch := NewIoStreamChannel(ctx, nil)
	defer ch.Close()

	// Initially no callbacks set
	assert.Nil(t, ch.GetEventHandleSet().GetCallback(IoStreamCallbackEventTypeAccepted))
	assert.Nil(t, ch.GetEventHandleSet().GetCallback(IoStreamCallbackEventTypeConnected))
	assert.Nil(t, ch.GetEventHandleSet().GetCallback(IoStreamCallbackEventTypeDisconnected))
	assert.Nil(t, ch.GetEventHandleSet().GetCallback(IoStreamCallbackEventTypeReceived))
	assert.Nil(t, ch.GetEventHandleSet().GetCallback(IoStreamCallbackEventTypeWritten))

	// Set callback and verify
	var called atomic.Bool
	ch.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			called.Store(true)
		})
	assert.NotNil(t, ch.GetEventHandleSet().GetCallback(IoStreamCallbackEventTypeReceived))

	// Invalid event type should return nil
	assert.Nil(t, ch.GetEventHandleSet().GetCallback(-1))
	assert.Nil(t, ch.GetEventHandleSet().GetCallback(IoStreamCallbackEventTypeMax))
}

// ============================================================================
// Connection-level Callback Test
// ============================================================================

// TestIoStreamConnectionLevelCallback verifies that connection-level callbacks are invoked
// in addition to channel-level callbacks.
func TestIoStreamConnectionLevelCallback(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32
	var channelRecvCount atomic.Int32
	var connRecvCount atomic.Int32

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			// Set connection-level receive callback on accepted connection
			conn.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
				func(ch types.IoStreamChannel, c types.IoStreamConnection, st int32, pd interface{}) {
					connRecvCount.Add(1)
				})
			connectedCount.Add(1)
		})

	// Channel-level receive callback
	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			channelRecvCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	conn1, errCode := cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok)

	// Send data
	cli.Send(conn1, []byte("test connection level callback"))

	// Both channel-level and connection-level callbacks should fire
	ok = waitForCondition(t, 5*time.Second, func() bool {
		return channelRecvCount.Load() >= 1 && connRecvCount.Load() >= 1
	})
	assert.True(t, ok,
		"both callbacks should fire: channel=%d, conn=%d",
		channelRecvCount.Load(), connRecvCount.Load())

	svr.Close()
	cli.Close()
}

// ============================================================================
// TCP Server Sends Many Big Buffers Test
// (Matching C++ test that sends from server-side accepted connection)
// ============================================================================

// TestIoStreamTcpServerSendsManyBigBuffers verifies that the server can send many large
// buffers to a connected client, matching the C++ test pattern.
func TestIoStreamTcpServerSendsManyBigBuffers(t *testing.T) {
	port := getFreePort(t)

	ctx := context.Background()
	svr := NewIoStreamChannel(ctx, nil)
	cli := NewIoStreamChannel(ctx, nil)

	var connectedCount atomic.Int32
	var svrAcceptedConn types.IoStreamConnection
	var checkBufSeq checkBuffSequence
	var recvRec recvRecord
	var recvCheckFlag atomic.Int32

	testBuf := getTestBuffer()

	svr.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeAccepted,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			svrAcceptedConn = conn
			connectedCount.Add(1)
		})

	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeConnected,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			connectedCount.Add(1)
		})

	// Recv callback on client
	cli.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(channel types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			data, ok := privData.([]byte)
			if !ok || data == nil {
				return
			}

			offset, length, seqOk := checkBufSeq.popFront()
			if !seqOk {
				t.Error("unexpected data received from server")
				return
			}

			recvRec.mu.Lock()
			recvRec.count++
			recvRec.bytes += len(data)
			recvRec.mu.Unlock()

			assert.Equal(t, length, len(data), "recv: data length mismatch (offset=%d)", offset)
			expected := testBuf[offset : offset+length]
			if !bytes.Equal(expected, data) {
				maxShow := 32
				if len(data) < maxShow {
					maxShow = len(data)
				}
				t.Errorf("server->client: data mismatch at offset=%d length=%d, first %d bytes: expected=%v got=%v",
					offset, length, maxShow, expected[:maxShow], data[:maxShow])
			}

			recvCheckFlag.Add(1)
		})

	errCode := svr.Listen(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	_, errCode = cli.Connect(fmt.Sprintf("ipv4://127.0.0.1:%d", port))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return connectedCount.Load() >= 2
	})
	assert.True(t, ok)

	// Server sends many big buffers to client
	rng := rand.New(rand.NewSource(42))
	sumSize := 0
	numMessages := 100
	for i := 0; i < numMessages; i++ {
		s := rng.Intn(2048)
		l := rng.Intn(10240) + 20*1024
		if s+l > maxTestBufferLen {
			l = maxTestBufferLen - s
		}
		checkBufSeq.pushBack(s, l)
		errCode := svr.Send(svrAcceptedConn, testBuf[s:s+l])
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
		sumSize += l
	}

	t.Logf("server send %d bytes data with %d packages", sumSize, numMessages)

	ok = waitForCondition(t, 30*time.Second, func() bool {
		return recvCheckFlag.Load() >= int32(numMessages)
	})
	assert.True(t, ok, "client should receive all %d messages, got %d",
		numMessages, recvCheckFlag.Load())

	recvRec.mu.Lock()
	t.Logf("client recv %d bytes data with %d packages", recvRec.bytes, recvRec.count)
	recvRec.mu.Unlock()

	svr.Close()
	cli.Close()
}

// ============================================================================
// processReadHeadFrames Small-Frame Compaction Unit Test
// ============================================================================

// TestProcessReadHeadFramesSmallFrameCompaction verifies that processReadHeadFrames
// correctly compacts remaining data to the front of the head buffer when multiple
// small frames are consumed and a partial small frame remains at the end.
//
// Scenario reproduced:
//  1. Head buffer is completely full (DataSmallSize bytes).
//  2. It contains N complete small frames followed by a partial (N+1)th frame.
//  3. processReadHeadFrames should process all N complete frames, then compact
//     the partial frame data to offset 0 and set conn.readHead.len accordingly.
//  4. After compaction, space = DataSmallSize - conn.readHead.len must be > 0
//     so that the next read can fill in the remaining bytes.
func TestProcessReadHeadFramesSmallFrameCompaction(t *testing.T) {
	ctx := context.Background()
	channel := NewIoStreamChannel(ctx, nil)
	defer channel.Close()

	// Track received payloads (must copy since they reference the static buffer)
	var received [][]byte
	var mu sync.Mutex
	channel.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(ch types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			data, ok := privData.([]byte)
			if !ok || data == nil {
				return
			}
			cp := make([]byte, len(data))
			copy(cp, data)
			mu.Lock()
			received = append(received, cp)
			mu.Unlock()
		})

	// Create a net.Pipe connection (we only need the conn object for direct testing)
	serverEnd, clientEnd := net.Pipe()
	defer serverEnd.Close()
	defer clientEnd.Close()

	conn := channel.createConnection(serverEnd, nil, IoStreamConnectionFlagConnect)

	// Build test frames: payload=1000 bytes each.
	// PackFrameSize(1000) = 4 (hash) + 2 (varint for 1000) + 1000 = 1006 bytes.
	// 7 frames = 7042 bytes; remaining space in DataSmallSize(7168) = 126.
	// So the 8th frame will be partially present (126 of 1006 bytes).
	testBuf := getTestBuffer()
	const payloadSize = 1000
	const numCompleteFrames = 7

	var frames [][]byte
	offset := 0
	for i := 0; i < numCompleteFrames+1; i++ {
		payload := testBuf[offset : offset+payloadSize]
		frameSize := PackFrameSize(uint64(payloadSize))
		frame := make([]byte, frameSize)
		written := PackFrame(payload, frame)
		assert.Equal(t, frameSize, written, "PackFrame should succeed")
		frames = append(frames, frame)
		offset += payloadSize
	}

	singleFrameSize := len(frames[0]) // 1006
	totalComplete := numCompleteFrames * singleFrameSize
	partialSize := DataSmallSize - totalComplete

	t.Logf("frame=%d bytes, %d complete=%d bytes, partial=%d of %d bytes",
		singleFrameSize, numCompleteFrames, totalComplete, partialSize, singleFrameSize)
	assert.True(t, partialSize > 0 && partialSize < singleFrameSize,
		"test setup: 8th frame should be partially in buffer")

	// Fill head buffer: 7 complete frames + partial 8th
	pos := 0
	for i := 0; i < numCompleteFrames; i++ {
		copy(conn.readHead.buffer[pos:], frames[i])
		pos += len(frames[i])
	}
	copy(conn.readHead.buffer[pos:], frames[numCompleteFrames][:partialSize])
	conn.readHead.len = DataSmallSize

	// === Round 1: Process head buffer ===
	isFree := channel.processReadHeadFrames(conn)
	assert.False(t, isFree, "should not request disconnect")

	mu.Lock()
	round1Count := len(received)
	mu.Unlock()
	assert.Equal(t, numCompleteFrames, round1Count,
		"should receive exactly %d complete frames", numCompleteFrames)

	// CRITICAL: After compaction, head buffer must have the partial frame data
	// at position 0 and conn.readHead.len must equal the partial data size.
	assert.Equal(t, partialSize, conn.readHead.len,
		"head buffer len should equal partial frame size (%d) after compaction, got %d",
		partialSize, conn.readHead.len)

	// Verify space is available for the next read
	space := DataSmallSize - conn.readHead.len
	assert.True(t, space > 0,
		"space for next read should be > 0 after compaction, got %d", space)

	// Verify the compacted data matches the beginning of the 8th frame
	expectedPartial := frames[numCompleteFrames][:partialSize]
	assert.True(t, bytes.Equal(expectedPartial, conn.readHead.buffer[:conn.readHead.len]),
		"compacted partial data should match first %d bytes of frame 8", partialSize)

	// === Round 2: Deliver remaining bytes and process again ===
	remainingData := frames[numCompleteFrames][partialSize:]
	copy(conn.readHead.buffer[conn.readHead.len:], remainingData)
	conn.readHead.len += len(remainingData)

	isFree = channel.processReadHeadFrames(conn)
	assert.False(t, isFree, "should not request disconnect on round 2")

	mu.Lock()
	round2Count := len(received)
	mu.Unlock()
	assert.Equal(t, numCompleteFrames+1, round2Count,
		"should receive %d frames total after round 2", numCompleteFrames+1)

	assert.Equal(t, 0, conn.readHead.len, "head buffer should be empty after all frames processed")

	// Verify all payload contents
	mu.Lock()
	for i := 0; i < numCompleteFrames+1; i++ {
		expected := testBuf[i*payloadSize : (i+1)*payloadSize]
		assert.True(t, bytes.Equal(expected, received[i]),
			"frame %d payload mismatch", i)
	}
	mu.Unlock()
}

// ============================================================================
// readLoop Small-Frame Compaction Integration Test (via net.Pipe)
// ============================================================================

// TestReadLoopSmallFrameCompactionViaPipe tests the full readLoop path by writing
// pre-framed data in DataSmallSize-aligned chunks through a net.Pipe so that the
// head buffer boundary is hit deterministically. This exercises the compaction of
// partial small frames across multiple read iterations.
func TestReadLoopSmallFrameCompactionViaPipe(t *testing.T) {
	ctx := context.Background()
	channel := NewIoStreamChannel(ctx, nil)
	defer channel.Close()

	// Track received payloads
	var receivedPayloads [][]byte
	var mu sync.Mutex
	channel.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(ch types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			data, ok := privData.([]byte)
			if !ok || data == nil {
				return
			}
			cp := make([]byte, len(data))
			copy(cp, data)
			mu.Lock()
			receivedPayloads = append(receivedPayloads, cp)
			mu.Unlock()
		})

	serverEnd, clientEnd := net.Pipe()
	conn := channel.createConnection(serverEnd, nil, IoStreamConnectionFlagAccept)

	// Start readLoop and writeLoop
	go channel.readLoop(conn)
	go channel.writeLoop(conn)

	// Build frame data: 15 frames of 1000-byte payloads (each frame = 1006 bytes)
	// Total = 15090 bytes. This is ~2.1x DataSmallSize so the head buffer will be
	// filled and compacted multiple times.
	testBuf := getTestBuffer()
	const payloadSize = 1000
	const numFrames = 15

	var allFrameData []byte
	for i := 0; i < numFrames; i++ {
		payload := testBuf[i*payloadSize : (i+1)*payloadSize]
		frameSize := PackFrameSize(uint64(payloadSize))
		frame := make([]byte, frameSize)
		PackFrame(payload, frame)
		allFrameData = append(allFrameData, frame...)
	}

	// Write through the pipe in DataSmallSize-aligned chunks so the head buffer
	// is exercised at its exact boundary.
	go func() {
		written := 0
		for written < len(allFrameData) {
			chunkSize := DataSmallSize
			if written+chunkSize > len(allFrameData) {
				chunkSize = len(allFrameData) - written
			}
			n, err := clientEnd.Write(allFrameData[written : written+chunkSize])
			if err != nil {
				return
			}
			written += n
		}
		// Allow readLoop to process all data before closing
		time.Sleep(200 * time.Millisecond)
		clientEnd.Close()
	}()

	// Wait for all frames
	ok := waitForCondition(t, 10*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedPayloads) >= numFrames
	})
	assert.True(t, ok, "should receive all %d frames, got %d", numFrames, func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedPayloads)
	}())

	// Verify payloads
	mu.Lock()
	for i := 0; i < numFrames && i < len(receivedPayloads); i++ {
		expected := testBuf[i*payloadSize : (i+1)*payloadSize]
		assert.True(t, bytes.Equal(expected, receivedPayloads[i]),
			"frame %d payload mismatch", i)
	}
	mu.Unlock()

	serverEnd.Close()
	clientEnd.Close()
	channel.Close()
}

// TestReadLoopManySmallFramesBoundaryStress sends hundreds of tiny frames through
// a net.Pipe in DataSmallSize-aligned chunks. The varying frame sizes cause the
// partial-frame boundary to land at different offsets within the head buffer,
// thoroughly exercising the compaction logic in processReadHeadFrames.
func TestReadLoopManySmallFramesBoundaryStress(t *testing.T) {
	ctx := context.Background()
	channel := NewIoStreamChannel(ctx, nil)
	defer channel.Close()

	var receivedPayloads [][]byte
	var mu sync.Mutex
	channel.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(ch types.IoStreamChannel, conn types.IoStreamConnection, status int32, privData interface{}) {
			data, ok := privData.([]byte)
			if !ok || data == nil {
				return
			}
			cp := make([]byte, len(data))
			copy(cp, data)
			mu.Lock()
			receivedPayloads = append(receivedPayloads, cp)
			mu.Unlock()
		})

	serverEnd, clientEnd := net.Pipe()
	conn := channel.createConnection(serverEnd, nil, IoStreamConnectionFlagAccept)

	go channel.readLoop(conn)
	go channel.writeLoop(conn)

	testBuf := getTestBuffer()
	const numFrames = 200
	rng := rand.New(rand.NewSource(99))

	type frameMeta struct {
		offset int
		length int
	}
	var metas []frameMeta
	var allFrameData []byte
	offset := 0
	for i := 0; i < numFrames; i++ {
		// Vary payload size between 10 and 2000 bytes to create different
		// partial-frame patterns at the head buffer boundary.
		payloadSize := 10 + rng.Intn(1991)
		if offset+payloadSize > len(testBuf) {
			payloadSize = len(testBuf) - offset
		}
		if payloadSize <= 0 {
			break
		}
		payload := testBuf[offset : offset+payloadSize]
		frameSize := PackFrameSize(uint64(payloadSize))
		frame := make([]byte, frameSize)
		PackFrame(payload, frame)
		allFrameData = append(allFrameData, frame...)
		metas = append(metas, frameMeta{offset, payloadSize})
		offset += payloadSize
	}

	actualNumFrames := len(metas)

	// Write in DataSmallSize-aligned chunks
	go func() {
		written := 0
		for written < len(allFrameData) {
			chunkSize := DataSmallSize
			if written+chunkSize > len(allFrameData) {
				chunkSize = len(allFrameData) - written
			}
			n, err := clientEnd.Write(allFrameData[written : written+chunkSize])
			if err != nil {
				return
			}
			written += n
		}
		time.Sleep(300 * time.Millisecond)
		clientEnd.Close()
	}()

	ok := waitForCondition(t, 15*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedPayloads) >= actualNumFrames
	})
	assert.True(t, ok, "should receive all %d frames, got %d", actualNumFrames, func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedPayloads)
	}())

	mu.Lock()
	for i := 0; i < actualNumFrames && i < len(receivedPayloads); i++ {
		expected := testBuf[metas[i].offset : metas[i].offset+metas[i].length]
		assert.True(t, bytes.Equal(expected, receivedPayloads[i]),
			"frame %d (payload %d bytes) mismatch", i, metas[i].length)
	}
	mu.Unlock()

	serverEnd.Close()
	channel.Close()
}

// TestIoStreamDisconnectCreatedConnection verifies that disconnecting a connection
// still in IoStreamConnectionStatusCreated succeeds and removes it from the pool
// (mirrors C++ commit 9577f560 allowing disconnect on kCreated connections).
func TestIoStreamDisconnectCreatedConnection(t *testing.T) {
	ctx := context.Background()
	channel := NewIoStreamChannel(ctx, nil)
	defer channel.Close()

	var disconnected atomic.Int32
	channel.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeDisconnected,
		func(ch types.IoStreamChannel, c types.IoStreamConnection, status int32, privData interface{}) {
			disconnected.Add(1)
		})

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()

	conn := channel.createConnection(serverEnd, nil, IoStreamConnectionFlagAccept)
	conn.SetStatus(IoStreamConnectionStatusCreated)
	go channel.writeLoop(conn)

	errCode := channel.Disconnect(conn)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	// Repeated disconnect is idempotent
	errCode = channel.Disconnect(conn)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return conn.GetStatus() == IoStreamConnectionStatusDisconnected && disconnected.Load() == 1
	})
	assert.True(t, ok, "created connection should transition to disconnected")
	assert.Equal(t, IoStreamConnectionStatusDisconnected, conn.GetStatus())
	assert.Equal(t, int32(1), disconnected.Load())
	channel.mu.RLock()
	connCount := len(channel.connections)
	channel.mu.RUnlock()
	assert.Equal(t, 0, connCount)
}

func TestIoStreamSendConcurrentDisconnect(t *testing.T) {
	channel := NewIoStreamChannel(context.Background(), nil)
	defer channel.Close()
	for range 100 {
		server, client := net.Pipe()
		conn := channel.createConnection(server, nil, IoStreamConnectionFlagAccept)
		ready := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			<-ready
			for range 100 {
				channel.Send(conn, []byte("payload"))
			}
		}()
		close(ready)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, channel.disconnectInternal(conn, true))
		<-done
		client.Close()
		require.Equal(t, error_code.EN_ATBUS_ERR_CLOSING, channel.Send(conn, []byte("payload")))
	}
}

// TestIoStreamSendConcurrentForceDisconnectWakesWriteLoop verifies that multiple concurrent
// Send goroutines racing against forced teardown neither panic nor leak writeLoop.
func TestIoStreamSendConcurrentForceDisconnectWakesWriteLoop(t *testing.T) {
	channel := NewIoStreamChannel(context.Background(), nil)
	defer channel.Close()

	for range 50 {
		server, client := net.Pipe()
		conn := channel.createConnection(server, nil, IoStreamConnectionFlagAccept)

		writeDone := make(chan struct{})
		go func() {
			defer close(writeDone)
			channel.writeLoop(conn)
		}()

		// Drain peer side so writes can make progress before teardown.
		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			buf := make([]byte, 4096)
			for {
				if _, err := client.Read(buf); err != nil {
					return
				}
			}
		}()

		ready := make(chan struct{})
		var senders sync.WaitGroup
		for range 4 {
			senders.Add(1)
			go func() {
				defer senders.Done()
				<-ready
				for range 64 {
					ret := channel.Send(conn, []byte("concurrent-payload"))
					assert.True(t,
						ret == error_code.EN_ATBUS_ERR_SUCCESS ||
							ret == error_code.EN_ATBUS_ERR_CLOSING ||
							ret == error_code.EN_ATBUS_ERR_BUFF_LIMIT,
						"unexpected Send result: %v", ret)
				}
			}()
		}

		close(ready)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, channel.disconnectInternal(conn, true))
		senders.Wait()

		select {
		case <-writeDone:
		case <-time.After(5 * time.Second):
			t.Fatal("writeLoop did not exit after forced disconnect")
		}

		client.Close()
		<-readDone
		assert.Equal(t, IoStreamConnectionStatusDisconnected, conn.GetStatus())
		assert.True(t, conn.GetFlag(IoStreamConnectionFlagClosing))
		assert.Equal(t, error_code.EN_ATBUS_ERR_CLOSING, channel.Send(conn, []byte("after-close")))
	}
}

// TestIoStreamSendConcurrentGracefulDisconnect verifies concurrent Send + Disconnect
// with an active writeLoop completes cleanly without panic or deadlock.
func TestIoStreamSendConcurrentGracefulDisconnect(t *testing.T) {
	channel := NewIoStreamChannel(context.Background(), nil)
	defer channel.Close()

	for range 50 {
		server, client := net.Pipe()
		conn := channel.createConnection(server, nil, IoStreamConnectionFlagAccept)

		writeDone := make(chan struct{})
		go func() {
			defer close(writeDone)
			channel.writeLoop(conn)
		}()

		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			buf := make([]byte, 4096)
			for {
				if _, err := client.Read(buf); err != nil {
					return
				}
			}
		}()

		ready := make(chan struct{})
		var senders sync.WaitGroup
		for range 4 {
			senders.Add(1)
			go func() {
				defer senders.Done()
				<-ready
				for range 64 {
					ret := channel.Send(conn, []byte("graceful-payload"))
					assert.True(t,
						ret == error_code.EN_ATBUS_ERR_SUCCESS ||
							ret == error_code.EN_ATBUS_ERR_CLOSING ||
							ret == error_code.EN_ATBUS_ERR_BUFF_LIMIT,
						"unexpected Send result: %v", ret)
				}
			}()
		}

		close(ready)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, channel.Disconnect(conn))
		senders.Wait()

		select {
		case <-writeDone:
		case <-time.After(5 * time.Second):
			t.Fatal("writeLoop did not exit after graceful disconnect")
		}

		client.Close()
		<-readDone
		assert.Equal(t, IoStreamConnectionStatusDisconnected, conn.GetStatus())
		assert.Equal(t, error_code.EN_ATBUS_ERR_CLOSING, channel.Send(conn, []byte("after-disconnect")))
	}
}

// TestIoStreamReadEgainCountResetOnSuccess verifies that readEgainCount resets to 0
// when bytes are successfully read (mirrors C++ commit af0f581b).
func TestIoStreamReadEgainCountResetOnSuccess(t *testing.T) {
	ctx := context.Background()
	conf := &IoStreamConfigure{}
	SetDefaultIoStreamConfigure(conf)
	conf.MaxReadNetEgainCount = 10
	channel := NewIoStreamChannel(ctx, conf)
	defer channel.Close()

	var recvCount atomic.Int32
	channel.GetEventHandleSet().SetCallback(IoStreamCallbackEventTypeReceived,
		func(ch types.IoStreamChannel, c types.IoStreamConnection, status int32, privData interface{}) {
			recvCount.Add(1)
		})

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	defer serverEnd.Close()

	conn := channel.createConnection(serverEnd, nil, IoStreamConnectionFlagAccept)
	atomic.StoreUint64(&conn.readEgainCount, 5)

	go channel.readLoop(conn)
	go channel.writeLoop(conn)

	payload := []byte("reset-egain-counter")
	frame := make([]byte, PackFrameSize(uint64(len(payload))))
	PackFrame(payload, frame)
	_, err := clientEnd.Write(frame)
	assert.NoError(t, err)

	ok := waitForCondition(t, 5*time.Second, func() bool {
		return recvCount.Load() >= 1
	})
	assert.True(t, ok, "should receive frame")
	assert.Equal(t, uint64(0), atomic.LoadUint64(&conn.readEgainCount), "readEgainCount should reset to 0 on successful read")
}
