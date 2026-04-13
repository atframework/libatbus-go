//go:build !windows

package libatbus_channel_iostream

// pipeGoNetworkType returns the Go net package network type for pipe/unix sockets.
// On Unix/macOS, this is "unix" (Unix domain socket).
func pipeGoNetworkType() string {
	return "unix"
}

// pipeAcceptScheme returns the scheme name used for accepted pipe connections.
// Matches C++ behavior: "unix" on Unix/macOS, "pipe" on Windows.
func pipeAcceptScheme() string {
	return "unix"
}
