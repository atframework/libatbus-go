// Package libatbus_channel_utility provides channel address utilities for libatbus.
// It provides functions to parse, construct and validate channel addresses.
package libatbus_channel_utility

import (
	"strconv"
	"strings"

	types "github.com/atframework/libatbus-go/types"
)

// ChannelAddress represents a parsed channel address with scheme, host, port and full address.
type ChannelAddress struct {
	Address string // Full address string (e.g., "tcp://127.0.0.1:8080")
	Scheme  string // Protocol scheme (e.g., "tcp", "unix", "shm")
	Host    string // Host part of the address
	Port    int    // Port number (0 if not specified)
}

func (c *ChannelAddress) GetAddress() string {
	return c.Address
}

func (c *ChannelAddress) GetScheme() string {
	return c.Scheme
}

func (c *ChannelAddress) GetHost() string {
	return c.Host
}

func (c *ChannelAddress) GetPort() int {
	return c.Port
}

var _ types.ChannelAddress = (*ChannelAddress)(nil)

// MakeAddress parses an address string and populates the ChannelAddress struct.
// Returns true if parsing succeeded, false otherwise.
// Address format: scheme://host[:port]
func MakeAddress(in string) (*ChannelAddress, bool) {
	if in == "" {
		return nil, false
	}

	addr := &ChannelAddress{
		Address: in,
	}

	// Find scheme separator "://"
	schemeEnd := strings.Index(in, "://")
	if schemeEnd == -1 {
		return nil, false
	}

	// Extract and lowercase the scheme
	addr.Scheme = strings.ToLower(in[:schemeEnd])

	// Find the last colon for port (must be after "://")
	rest := in[schemeEnd+3:]
	portStart := strings.LastIndex(rest, ":")

	addr.Port = 0
	if portStart != -1 {
		// Parse port number
		portStr := rest[portStart+1:]
		if port, err := strconv.Atoi(portStr); err == nil {
			addr.Port = port
		}
		addr.Host = rest[:portStart]
	} else {
		addr.Host = rest
	}

	return addr, true
}

// MakeAddressFromComponents constructs a ChannelAddress from individual components.
func MakeAddressFromComponents(scheme, host string, port int) *ChannelAddress {
	addr := &ChannelAddress{
		Scheme: strings.ToLower(scheme),
		Host:   host,
		Port:   port,
	}

	// Build the full address string
	var builder strings.Builder
	builder.Grow(len(addr.Scheme) + len(addr.Host) + 4 + 8)
	builder.WriteString(addr.Scheme)
	builder.WriteString("://")
	builder.WriteString(addr.Host)

	if port > 0 {
		builder.WriteString(":")
		builder.WriteString(strconv.Itoa(port))
	}

	addr.Address = builder.String()
	return addr
}

// IsDuplexAddress checks if the address is a duplex (bidirectional) address.
// Duplex addresses are those that are not simplex addresses.
func IsDuplexAddress(in string) bool {
	if in == "" {
		return false
	}
	return !IsSimplexAddress(in)
}

// IsSimplexAddress checks if the address is a simplex (unidirectional) address.
// Simplex addresses include: mem:, shm:
func IsSimplexAddress(in string) bool {
	if in == "" {
		return false
	}

	lowerIn := strings.ToLower(in)

	// Check for mem: prefix
	if len(lowerIn) >= 4 && lowerIn[:4] == "mem:" {
		return true
	}

	// Check for shm: prefix
	if len(lowerIn) >= 4 && lowerIn[:4] == "shm:" {
		return true
	}

	return false
}

// IsLocalHostAddress checks if the address refers to the local host.
// Local host addresses include: mem:, shm:, unix:, pipe:, atcp://127.0.0.1, atcp://::1, ipv4://127.0.0.1, ipv6://::1
func IsLocalHostAddress(in string) bool {
	if in == "" {
		return false
	}

	// Check if it's a local process address first
	if IsLocalProcessAddress(in) {
		return true
	}

	lowerIn := strings.ToLower(in)

	// Check for shm: prefix
	if len(lowerIn) >= 4 && lowerIn[:4] == "shm:" {
		return true
	}

	// Check for unix: or pipe: prefix
	if len(lowerIn) >= 5 {
		prefix := lowerIn[:5]
		if prefix == "unix:" || prefix == "pipe:" {
			return true
		}
	}

	// Check for atcp/ipv4/ipv6://127.0.0.1 or atcp/ipv4/ipv6://::1
	if len(lowerIn) >= 10 && (lowerIn[:5] == "atcp:" || lowerIn[:5] == "ipv4:" || lowerIn[:5] == "ipv6:") {
		rest := lowerIn[7:] // skip "xxxx://"
		for _, loopback := range []string{"127.0.0.1", "::1"} {
			if strings.HasPrefix(rest, loopback) {
				remaining := rest[len(loopback):]
				if remaining == "" || remaining[0] == ':' {
					return true
				}
			}
		}
	}

	return false
}

// IsLocalProcessAddress checks if the address is a local process address.
// Local process addresses include: mem:
func IsLocalProcessAddress(in string) bool {
	if in == "" {
		return false
	}

	lowerIn := strings.ToLower(in)

	// Check for mem: prefix
	minLen := 4
	if len(lowerIn) < minLen {
		minLen = len(lowerIn)
	}

	if minLen >= 4 && lowerIn[:4] == "mem:" {
		return true
	}

	return false
}
