//go:build windows

package libatbus_channel_iostream

// pipeGoNetworkType returns the Go net package network type for pipe/unix sockets.
// On Windows 10 1803+, Go supports AF_UNIX sockets via the "unix" network type.
func pipeGoNetworkType() string {
	return "unix"
}

// pipeAcceptScheme returns the scheme name used for accepted pipe connections.
// Matches C++ behavior: "unix" on Unix/macOS, "pipe" on Windows.
func pipeAcceptScheme() string {
	return "pipe"
}
