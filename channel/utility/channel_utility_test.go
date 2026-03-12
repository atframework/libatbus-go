package libatbus_channel_utility

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ===================== MakeAddress Tests =====================

// TestMakeAddressEmptyString tests parsing an empty address string
func TestMakeAddressEmptyString(t *testing.T) {
	// Arrange
	input := ""

	// Act
	addr, ok := MakeAddress(input)

	// Assert
	assert.False(t, ok, "should return false for empty string")
	assert.Nil(t, addr, "address should be nil for empty string")
}

// TestMakeAddressNoScheme tests parsing an address without scheme separator
func TestMakeAddressNoScheme(t *testing.T) {
	// Arrange
	input := "127.0.0.1:8080"

	// Act
	addr, ok := MakeAddress(input)

	// Assert
	assert.False(t, ok, "should return false when no scheme separator")
	assert.Nil(t, addr, "address should be nil when no scheme separator")
}

// TestMakeAddressValidIPv4WithPort tests parsing a valid IPv4 address with port
func TestMakeAddressValidIPv4WithPort(t *testing.T) {
	// Arrange
	input := "ipv4://127.0.0.1:8080"

	// Act
	addr, ok := MakeAddress(input)

	// Assert
	assert.True(t, ok, "should return true for valid address")
	assert.NotNil(t, addr, "address should not be nil")
	assert.Equal(t, "ipv4://127.0.0.1:8080", addr.Address)
	assert.Equal(t, "ipv4", addr.Scheme)
	assert.Equal(t, "127.0.0.1", addr.Host)
	assert.Equal(t, 8080, addr.Port)
}

// TestMakeAddressValidUnixWithoutPort tests parsing a valid Unix address without port
func TestMakeAddressValidUnixWithoutPort(t *testing.T) {
	// Arrange
	input := "unix://socket.sock"

	// Act
	addr, ok := MakeAddress(input)

	// Assert
	assert.True(t, ok, "should return true for valid address")
	assert.NotNil(t, addr, "address should not be nil")
	assert.Equal(t, "unix://socket.sock", addr.Address)
	assert.Equal(t, "unix", addr.Scheme)
	assert.Equal(t, "socket.sock", addr.Host)
	assert.Equal(t, 0, addr.Port)
}

// TestMakeAddressSchemeToLower tests that scheme is converted to lowercase
func TestMakeAddressSchemeToLower(t *testing.T) {
	// Arrange
	input := "IPV4://127.0.0.1:8080"

	// Act
	addr, ok := MakeAddress(input)

	// Assert
	assert.True(t, ok, "should return true for valid address")
	assert.Equal(t, "ipv4", addr.Scheme, "scheme should be lowercase")
}

// TestMakeAddressIPv6 tests parsing an IPv6 address
func TestMakeAddressIPv6(t *testing.T) {
	// Arrange
	input := "ipv6://::1:8080"

	// Act
	addr, ok := MakeAddress(input)

	// Assert
	assert.True(t, ok, "should return true for valid address")
	assert.Equal(t, "ipv6", addr.Scheme)
	assert.Equal(t, "::1", addr.Host)
	assert.Equal(t, 8080, addr.Port)
}

// TestMakeAddressMemory tests parsing a memory address
func TestMakeAddressMemory(t *testing.T) {
	// Arrange
	input := "mem://shared_buffer"

	// Act
	addr, ok := MakeAddress(input)

	// Assert
	assert.True(t, ok, "should return true for valid address")
	assert.Equal(t, "mem", addr.Scheme)
	assert.Equal(t, "shared_buffer", addr.Host)
	assert.Equal(t, 0, addr.Port)
}

// TestMakeAddressInvalidPort tests parsing an address with invalid port
func TestMakeAddressInvalidPort(t *testing.T) {
	// Arrange
	input := "ipv4://127.0.0.1:invalid"

	// Act
	addr, ok := MakeAddress(input)

	// Assert
	assert.True(t, ok, "should return true even with invalid port")
	assert.Equal(t, "ipv4", addr.Scheme)
	assert.Equal(t, "127.0.0.1", addr.Host)
	assert.Equal(t, 0, addr.Port, "port should be 0 for invalid port string")
}

// ===================== MakeAddressFromComponents Tests =====================

// TestMakeAddressFromComponentsWithPort tests constructing an address with port
func TestMakeAddressFromComponentsWithPort(t *testing.T) {
	// Arrange
	scheme := "ipv4"
	host := "127.0.0.1"
	port := 8080

	// Act
	addr := MakeAddressFromComponents(scheme, host, port)

	// Assert
	assert.NotNil(t, addr)
	assert.Equal(t, "ipv4://127.0.0.1:8080", addr.Address)
	assert.Equal(t, "ipv4", addr.Scheme)
	assert.Equal(t, "127.0.0.1", addr.Host)
	assert.Equal(t, 8080, addr.Port)
}

// TestMakeAddressFromComponentsWithoutPort tests constructing an address without port
func TestMakeAddressFromComponentsWithoutPort(t *testing.T) {
	// Arrange
	scheme := "unix"
	host := "socket.sock"
	port := 0

	// Act
	addr := MakeAddressFromComponents(scheme, host, port)

	// Assert
	assert.NotNil(t, addr)
	assert.Equal(t, "unix://socket.sock", addr.Address)
	assert.Equal(t, "unix", addr.Scheme)
	assert.Equal(t, "socket.sock", addr.Host)
	assert.Equal(t, 0, addr.Port)
}

// TestMakeAddressFromComponentsSchemeToLower tests that scheme is converted to lowercase
func TestMakeAddressFromComponentsSchemeToLower(t *testing.T) {
	// Arrange
	scheme := "IPV4"
	host := "localhost"
	port := 80

	// Act
	addr := MakeAddressFromComponents(scheme, host, port)

	// Assert
	assert.Equal(t, "ipv4", addr.Scheme, "scheme should be lowercase")
	assert.Equal(t, "ipv4://localhost:80", addr.Address)
}

// TestMakeAddressFromComponentsNegativePort tests constructing an address with negative port
func TestMakeAddressFromComponentsNegativePort(t *testing.T) {
	// Arrange
	scheme := "ipv4"
	host := "localhost"
	port := -1

	// Act
	addr := MakeAddressFromComponents(scheme, host, port)

	// Assert
	assert.NotNil(t, addr)
	assert.Equal(t, "ipv4://localhost", addr.Address, "negative port should not be appended")
	assert.Equal(t, -1, addr.Port)
}

// ===================== IsDuplexAddress Tests =====================

// TestIsDuplexAddressEmpty tests duplex check on empty string
func TestIsDuplexAddressEmpty(t *testing.T) {
	// Arrange
	input := ""

	// Act
	result := IsDuplexAddress(input)

	// Assert
	assert.False(t, result, "empty string should not be duplex")
}

// TestIsDuplexAddressIPv4 tests duplex check on IPv4 address
func TestIsDuplexAddressIPv4(t *testing.T) {
	// Arrange
	input := "ipv4://127.0.0.1:8080"

	// Act
	result := IsDuplexAddress(input)

	// Assert
	assert.True(t, result, "ipv4 address should be duplex")
}

// TestIsDuplexAddressMem tests duplex check on memory address
func TestIsDuplexAddressMem(t *testing.T) {
	// Arrange
	input := "mem://buffer"

	// Act
	result := IsDuplexAddress(input)

	// Assert
	assert.False(t, result, "mem address should not be duplex")
}

// TestIsDuplexAddressShm tests duplex check on shared memory address
func TestIsDuplexAddressShm(t *testing.T) {
	// Arrange
	input := "shm://shared"

	// Act
	result := IsDuplexAddress(input)

	// Assert
	assert.False(t, result, "shm address should not be duplex")
}

// ===================== IsSimplexAddress Tests =====================

// TestIsSimplexAddressEmpty tests simplex check on empty string
func TestIsSimplexAddressEmpty(t *testing.T) {
	// Arrange
	input := ""

	// Act
	result := IsSimplexAddress(input)

	// Assert
	assert.False(t, result, "empty string should not be simplex")
}

// TestIsSimplexAddressMem tests simplex check on memory address
func TestIsSimplexAddressMem(t *testing.T) {
	// Arrange
	input := "mem://buffer"

	// Act
	result := IsSimplexAddress(input)

	// Assert
	assert.True(t, result, "mem address should be simplex")
}

// TestIsSimplexAddressMemUppercase tests simplex check on uppercase memory address
func TestIsSimplexAddressMemUppercase(t *testing.T) {
	// Arrange
	input := "MEM://buffer"

	// Act
	result := IsSimplexAddress(input)

	// Assert
	assert.True(t, result, "MEM address should be simplex (case-insensitive)")
}

// TestIsSimplexAddressShm tests simplex check on shared memory address
func TestIsSimplexAddressShm(t *testing.T) {
	// Arrange
	input := "shm://shared"

	// Act
	result := IsSimplexAddress(input)

	// Assert
	assert.True(t, result, "shm address should be simplex")
}

// TestIsSimplexAddressShmUppercase tests simplex check on uppercase shared memory address
func TestIsSimplexAddressShmUppercase(t *testing.T) {
	// Arrange
	input := "SHM://shared"

	// Act
	result := IsSimplexAddress(input)

	// Assert
	assert.True(t, result, "SHM address should be simplex (case-insensitive)")
}

// TestIsSimplexAddressIPv4 tests simplex check on IPv4 address
func TestIsSimplexAddressIPv4(t *testing.T) {
	// Arrange
	input := "ipv4://127.0.0.1:8080"

	// Act
	result := IsSimplexAddress(input)

	// Assert
	assert.False(t, result, "ipv4 address should not be simplex")
}

// TestIsSimplexAddressUnix tests simplex check on Unix socket address
func TestIsSimplexAddressUnix(t *testing.T) {
	// Arrange
	input := "unix://socket.sock"

	// Act
	result := IsSimplexAddress(input)

	// Assert
	assert.False(t, result, "unix address should not be simplex")
}

// ===================== IsLocalHostAddress Tests =====================

// TestIsLocalHostAddressEmpty tests local host check on empty string
func TestIsLocalHostAddressEmpty(t *testing.T) {
	// Arrange
	input := ""

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.False(t, result, "empty string should not be local host")
}

// TestIsLocalHostAddressMem tests local host check on memory address
func TestIsLocalHostAddressMem(t *testing.T) {
	// Arrange
	input := "mem://buffer"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "mem address should be local host")
}

// TestIsLocalHostAddressShm tests local host check on shared memory address
func TestIsLocalHostAddressShm(t *testing.T) {
	// Arrange
	input := "shm://shared"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "shm address should be local host")
}

// TestIsLocalHostAddressUnix tests local host check on Unix socket address
func TestIsLocalHostAddressUnix(t *testing.T) {
	// Arrange
	input := "unix://socket.sock"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "unix address should be local host")
}

// TestIsLocalHostAddressUnixUppercase tests local host check on uppercase Unix socket address
func TestIsLocalHostAddressUnixUppercase(t *testing.T) {
	// Arrange
	input := "UNIX://socket.sock"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "UNIX address should be local host (case-insensitive)")
}

// TestIsLocalHostAddressPipe tests local host check on pipe address
func TestIsLocalHostAddressPipe(t *testing.T) {
	// Arrange
	input := "pipe://named_pipe"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "pipe address should be local host")
}

// TestIsLocalHostAddressPipeUppercase tests local host check on uppercase pipe address
func TestIsLocalHostAddressPipeUppercase(t *testing.T) {
	// Arrange
	input := "PIPE://named_pipe"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "PIPE address should be local host (case-insensitive)")
}

// TestIsLocalHostAddressIPv4Loopback tests local host check on IPv4 loopback
func TestIsLocalHostAddressIPv4Loopback(t *testing.T) {
	// Arrange
	input := "ipv4://127.0.0.1:8080"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "ipv4://127.0.0.1 should be local host")
}

// TestIsLocalHostAddressIPv4LoopbackNoPort tests local host check on IPv4 loopback without port
func TestIsLocalHostAddressIPv4LoopbackNoPort(t *testing.T) {
	// Arrange
	input := "ipv4://127.0.0.1"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "ipv4://127.0.0.1 without port should be local host")
}

// TestIsLocalHostAddressIPv4LoopbackUppercase tests local host check on uppercase IPv4 loopback
func TestIsLocalHostAddressIPv4LoopbackUppercase(t *testing.T) {
	// Arrange
	input := "IPV4://127.0.0.1:8080"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "IPV4://127.0.0.1 should be local host (case-insensitive)")
}

// TestIsLocalHostAddressIPv6Loopback tests local host check on IPv6 loopback
func TestIsLocalHostAddressIPv6Loopback(t *testing.T) {
	// Arrange
	input := "ipv6://::1:8080"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "ipv6://::1 should be local host")
}

// TestIsLocalHostAddressIPv6LoopbackNoPort tests local host check on IPv6 loopback without port
func TestIsLocalHostAddressIPv6LoopbackNoPort(t *testing.T) {
	// Arrange
	input := "ipv6://::1"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "ipv6://::1 without port should be local host")
}

// TestIsLocalHostAddressIPv6LoopbackUppercase tests local host check on uppercase IPv6 loopback
func TestIsLocalHostAddressIPv6LoopbackUppercase(t *testing.T) {
	// Arrange
	input := "IPV6://::1"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.True(t, result, "IPV6://::1 should be local host (case-insensitive)")
}

// TestIsLocalHostAddressIPv4Remote tests local host check on remote IPv4 address
func TestIsLocalHostAddressIPv4Remote(t *testing.T) {
	// Arrange
	input := "ipv4://192.168.1.1:8080"

	// Act
	result := IsLocalHostAddress(input)

	// Assert
	assert.False(t, result, "remote ipv4 address should not be local host")
}

// ===================== IsLocalProcessAddress Tests =====================

// TestIsLocalProcessAddressEmpty tests local process check on empty string
func TestIsLocalProcessAddressEmpty(t *testing.T) {
	// Arrange
	input := ""

	// Act
	result := IsLocalProcessAddress(input)

	// Assert
	assert.False(t, result, "empty string should not be local process")
}

// TestIsLocalProcessAddressMem tests local process check on memory address
func TestIsLocalProcessAddressMem(t *testing.T) {
	// Arrange
	input := "mem://buffer"

	// Act
	result := IsLocalProcessAddress(input)

	// Assert
	assert.True(t, result, "mem address should be local process")
}

// TestIsLocalProcessAddressMemUppercase tests local process check on uppercase memory address
func TestIsLocalProcessAddressMemUppercase(t *testing.T) {
	// Arrange
	input := "MEM://buffer"

	// Act
	result := IsLocalProcessAddress(input)

	// Assert
	assert.True(t, result, "MEM address should be local process (case-insensitive)")
}

// TestIsLocalProcessAddressMemShort tests local process check on short memory address
func TestIsLocalProcessAddressMemShort(t *testing.T) {
	// Arrange
	input := "mem:"

	// Act
	result := IsLocalProcessAddress(input)

	// Assert
	assert.True(t, result, "mem: should be local process")
}

// TestIsLocalProcessAddressShm tests local process check on shared memory address
func TestIsLocalProcessAddressShm(t *testing.T) {
	// Arrange
	input := "shm://shared"

	// Act
	result := IsLocalProcessAddress(input)

	// Assert
	assert.False(t, result, "shm address should not be local process")
}

// TestIsLocalProcessAddressUnix tests local process check on Unix socket address
func TestIsLocalProcessAddressUnix(t *testing.T) {
	// Arrange
	input := "unix://socket.sock"

	// Act
	result := IsLocalProcessAddress(input)

	// Assert
	assert.False(t, result, "unix address should not be local process")
}

// TestIsLocalProcessAddressIPv4 tests local process check on IPv4 address
func TestIsLocalProcessAddressIPv4(t *testing.T) {
	// Arrange
	input := "ipv4://127.0.0.1:8080"

	// Act
	result := IsLocalProcessAddress(input)

	// Assert
	assert.False(t, result, "ipv4 address should not be local process")
}

// TestIsLocalProcessAddressTooShort tests local process check on too short string
func TestIsLocalProcessAddressTooShort(t *testing.T) {
	// Arrange
	input := "me"

	// Act
	result := IsLocalProcessAddress(input)

	// Assert
	assert.False(t, result, "string too short should not be local process")
}

// ===================== Round-trip Tests =====================

// TestMakeAddressRoundTrip tests that parsing and constructing produces consistent results
func TestMakeAddressRoundTrip(t *testing.T) {
	// Arrange
	original := "ipv4://localhost:8080"

	// Act
	parsed, ok := MakeAddress(original)
	assert.True(t, ok)

	reconstructed := MakeAddressFromComponents(parsed.Scheme, parsed.Host, parsed.Port)

	// Assert
	assert.Equal(t, original, reconstructed.Address, "round-trip should produce same address")
}

// TestMakeAddressRoundTripNoPort tests round-trip for address without port
func TestMakeAddressRoundTripNoPort(t *testing.T) {
	// Arrange
	original := "unix://socket.sock"

	// Act
	parsed, ok := MakeAddress(original)
	assert.True(t, ok)

	reconstructed := MakeAddressFromComponents(parsed.Scheme, parsed.Host, parsed.Port)

	// Assert
	assert.Equal(t, original, reconstructed.Address, "round-trip should produce same address")
}

// ===================== Table-driven Tests =====================

// TestIsLocalHostAddressTableDriven tests IsLocalHostAddress with multiple inputs
func TestIsLocalHostAddressTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty", "", false},
		{"mem", "mem://buffer", true},
		{"shm", "shm://shared", true},
		{"unix", "unix://socket", true},
		{"pipe", "pipe://named", true},
		{"ipv4 loopback", "ipv4://127.0.0.1", true},
		{"ipv4 loopback with port", "ipv4://127.0.0.1:8080", true},
		{"ipv6 loopback", "ipv6://::1", true},
		{"ipv6 loopback with port", "ipv6://::1:8080", true},
		{"atcp ipv4 loopback", "atcp://127.0.0.1", true},
		{"atcp ipv4 loopback with port", "atcp://127.0.0.1:8080", true},
		{"atcp ipv6 loopback", "atcp://::1", true},
		{"atcp ipv6 loopback with port", "atcp://::1:8080", true},
		{"ipv4 remote 2", "ipv4://192.168.1.1:8080", false},
		{"ipv4 remote", "ipv4://10.0.0.1:80", false},
		{"atcp ipv4 remote 2", "atcp://192.168.1.1:8080", false},
		{"atcp ipv4 remote", "atcp://10.0.0.1:80", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := IsLocalHostAddress(tt.input)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsSimplexAddressTableDriven tests IsSimplexAddress with multiple inputs
func TestIsSimplexAddressTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"empty", "", false},
		{"mem", "mem://buffer", true},
		{"MEM uppercase", "MEM://buffer", true},
		{"shm", "shm://shared", true},
		{"SHM uppercase", "SHM://shared", true},
		{"unix", "unix://socket", false},
		{"ipv4", "ipv4://localhost:8080", false},
		{"pipe", "pipe://named", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := IsSimplexAddress(tt.input)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

/*
Uncovered scenarios and reasons:
1. IPv6 addresses with brackets (e.g., [::1]) - The C++ implementation does not handle this case explicitly
2. URL-encoded characters in host - Not handled in C++ implementation
3. Multiple ports in address - The C++ uses LastIndex which handles this, Go implementation follows same pattern
4. Thread safety - These are pure functions with no shared state, inherently thread-safe
*/
