//go:build windows

package libatbus_channel_iostream

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func isPeerReset(err error) bool {
	return errors.Is(err, windows.WSAECONNRESET) || errors.Is(err, windows.ERROR_BROKEN_PIPE)
}

func IsNativePipePath(path string) bool {
	path = strings.ToLower(path)
	return strings.HasPrefix(path, `\\.\pipe\`) || strings.HasPrefix(path, `\\?\pipe\`)
}

const (
	pipeIPCHeaderSize = 16
	pipeIPCHasData    = 1
)

type ipcPipeListener struct {
	net.Listener
}

func (l ipcPipeListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &ipcPipeConn{Conn: conn}, nil
}

// C++ libatbus opens connected libuv pipes with IPC enabled. On Windows,
// libuv wraps data writes in a 16-byte IPC header before the atbus framing.
type ipcPipeConn struct {
	net.Conn
	remaining uint32
}

func (c *ipcPipeConn) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	for c.remaining == 0 {
		var header [pipeIPCHeaderSize]byte
		if _, err := io.ReadFull(c.Conn, header[:]); err != nil {
			return 0, err
		}
		flags := binary.LittleEndian.Uint32(header[:4])
		length := binary.LittleEndian.Uint32(header[8:12])
		reserved := binary.LittleEndian.Uint32(header[12:])
		if flags&^pipeIPCHasData != 0 || reserved != 0 || (flags == 0 && length != 0) {
			return 0, errors.New("invalid libuv pipe data frame")
		}
		c.remaining = length
	}
	if uint64(len(data)) > uint64(c.remaining) {
		data = data[:c.remaining]
	}
	n, err := c.Conn.Read(data)
	c.remaining -= uint32(n)
	return n, err
}

func (c *ipcPipeConn) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if uint64(len(data)) > math.MaxUint32 {
		return 0, errors.New("libuv pipe data frame is too large")
	}
	var header [pipeIPCHeaderSize]byte
	binary.LittleEndian.PutUint32(header[:4], pipeIPCHasData)
	binary.LittleEndian.PutUint32(header[8:12], uint32(len(data)))
	buffers := net.Buffers{header[:], data}
	n, err := buffers.WriteTo(c.Conn)
	return int(max(0, n-pipeIPCHeaderSize)), err
}

func listenStream(network, address string) (net.Listener, error) {
	if network != "pipe" {
		return net.Listen(network, address)
	}
	if !IsNativePipePath(address) {
		return net.Listen("unix", address)
	}
	listener, err := winio.ListenPipe(address, nil)
	if err != nil {
		return nil, err
	}
	return ipcPipeListener{Listener: listener}, nil
}

func dialStream(ctx context.Context, dialer *net.Dialer, network, address string) (net.Conn, error) {
	if network != "pipe" {
		return dialer.DialContext(ctx, network, address)
	}
	if !IsNativePipePath(address) {
		return dialer.DialContext(ctx, "unix", address)
	}
	if dialer.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dialer.Timeout)
		defer cancel()
	}
	conn, err := winio.DialPipeContext(ctx, address)
	if err != nil {
		return nil, err
	}
	return &ipcPipeConn{Conn: conn}, nil
}

// pipeAcceptScheme returns the scheme name used for accepted pipe connections.
// Matches C++ behavior: "unix" on Unix/macOS, "pipe" on Windows.
func pipeAcceptScheme() string {
	return "pipe"
}

// GetMaxUnixSocketLength returns the maximum path length supported for AF_UNIX sockets on Windows.
// Matches C++ io_stream_get_max_unix_socket_length() (sizeof(sockaddr_un::sun_path) - 1).
func GetMaxUnixSocketLength() int {
	return 107
}
