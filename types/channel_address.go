package libatbus_types

// ChannelAddress represents a parsed channel address with scheme, host, port and full address.
type ChannelAddress interface {
	GetAddress() string // Full address string (e.g., "tcp://127.0.0.1:8080")
	GetScheme() string  // Protocol scheme (e.g., "tcp", "unix", "shm")
	GetHost() string    // Host part of the address
	GetPort() int       // Port number (0 if not specified)
}
