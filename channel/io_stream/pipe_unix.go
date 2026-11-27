//go:build !windows

package libatbus_channel_iostream

import (
	"context"
	"errors"
	"net"
	"syscall"
)

func isPeerReset(err error) bool {
	return errors.Is(err, syscall.ECONNRESET)
}

func IsNativePipePath(string) bool {
	return false
}

func listenStream(network, address string) (net.Listener, error) {
	if network == "pipe" {
		network = "unix"
	}
	return net.Listen(network, address)
}

func dialStream(ctx context.Context, dialer *net.Dialer, network, address string) (net.Conn, error) {
	if network == "pipe" {
		network = "unix"
	}
	return dialer.DialContext(ctx, network, address)
}

// pipeAcceptScheme returns the scheme name used for accepted pipe connections.
// Matches C++ behavior: "unix" on Unix/macOS, "pipe" on Windows.
func pipeAcceptScheme() string {
	return "unix"
}

// GetMaxUnixSocketLength returns the maximum path length supported for Unix domain sockets.
// Matches C++ io_stream_get_max_unix_socket_length() (conservative minimum across Linux/macOS/BSD).
func GetMaxUnixSocketLength() int {
	return 103
}
