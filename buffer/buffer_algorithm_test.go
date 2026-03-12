package libatbus_buffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// ReadVint Tests
// =============================================================================

// TestReadVintEmptyBuffer tests reading from an empty buffer
func TestReadVintEmptyBuffer(t *testing.T) {
	// Arrange
	data := []byte{}

	// Act
	value, consumed := ReadVint(data)

	// Assert
	assert.Equal(t, uint64(0), value, "value should be 0 for empty buffer")
	assert.Equal(t, 0, consumed, "consumed should be 0 for empty buffer")
}

// TestReadVintSingleByte tests reading a single-byte value (0-127)
func TestReadVintSingleByte(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected uint64
	}{
		{"zero", []byte{0x00}, 0},
		{"one", []byte{0x01}, 1},
		{"max_single_byte", []byte{0x7F}, 127},
		{"mid_value", []byte{0x40}, 64},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			value, consumed := ReadVint(tc.data)

			// Assert
			assert.Equal(t, tc.expected, value, "value mismatch")
			assert.Equal(t, 1, consumed, "should consume exactly 1 byte")
		})
	}
}

// TestReadVintTwoByte tests reading a two-byte value (128-16383)
func TestReadVintTwoByte(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected uint64
	}{
		// 128 = 0x80 in varint: [0x81, 0x00] => (1 << 7) | 0 = 128
		{"value_128", []byte{0x81, 0x00}, 128},
		// 255 = (1 << 7) | 127 = 0x81, 0x7F
		{"value_255", []byte{0x81, 0x7F}, 255},
		// 300 = (2 << 7) | 44 = 0x82, 0x2C
		{"value_300", []byte{0x82, 0x2C}, 300},
		// 16383 = (127 << 7) | 127 = 0xFF, 0x7F
		{"max_two_byte", []byte{0xFF, 0x7F}, 16383},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			value, consumed := ReadVint(tc.data)

			// Assert
			assert.Equal(t, tc.expected, value, "value mismatch")
			assert.Equal(t, 2, consumed, "should consume exactly 2 bytes")
		})
	}
}

// TestReadVintMultiByte tests reading multi-byte values
func TestReadVintMultiByte(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected uint64
	}{
		// 3 bytes: (1 << 14) | (0 << 7) | 0 = 16384
		{"value_16384", []byte{0x81, 0x80, 0x00}, 16384},
		// Large value test
		{"large_value", []byte{0x81, 0x80, 0x80, 0x00}, 2097152}, // 1 << 21
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			value, consumed := ReadVint(tc.data)

			// Assert
			assert.Equal(t, tc.expected, value, "value mismatch")
			assert.Equal(t, len(tc.data), consumed, "should consume all bytes")
		})
	}
}

// TestReadVintTruncated tests reading from a truncated buffer (continuation bit set but no more data)
func TestReadVintTruncated(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"single_continuation", []byte{0x80}},
		{"two_continuations", []byte{0x80, 0x80}},
		{"valid_then_truncated", []byte{0x81, 0x80}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			value, consumed := ReadVint(tc.data)

			// Assert
			assert.Equal(t, uint64(0), value, "value should be 0 for truncated buffer")
			assert.Equal(t, 0, consumed, "consumed should be 0 for truncated buffer")
		})
	}
}

// TestReadVintWithExtraData tests that reading stops at the terminating byte
func TestReadVintWithExtraData(t *testing.T) {
	// Arrange: value 127 followed by extra data
	data := []byte{0x7F, 0xFF, 0xFF}

	// Act
	value, consumed := ReadVint(data)

	// Assert
	assert.Equal(t, uint64(127), value, "value should be 127")
	assert.Equal(t, 1, consumed, "should consume only 1 byte")
}

// =============================================================================
// WriteVint Tests
// =============================================================================

// TestWriteVintEmptyBuffer tests writing to an empty buffer
func TestWriteVintEmptyBuffer(t *testing.T) {
	// Arrange
	data := []byte{}

	// Act
	written := WriteVint(0, data)

	// Assert
	assert.Equal(t, 0, written, "should return 0 for empty buffer")
}

// TestWriteVintSingleByte tests writing single-byte values (0-127)
func TestWriteVintSingleByte(t *testing.T) {
	testCases := []struct {
		name     string
		value    uint64
		expected []byte
	}{
		{"zero", 0, []byte{0x00}},
		{"one", 1, []byte{0x01}},
		{"max_single_byte", 127, []byte{0x7F}},
		{"mid_value", 64, []byte{0x40}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			data := make([]byte, 10)

			// Act
			written := WriteVint(tc.value, data)

			// Assert
			assert.Equal(t, len(tc.expected), written, "written bytes mismatch")
			assert.Equal(t, tc.expected, data[:written], "data mismatch")
		})
	}
}

// TestWriteVintTwoByte tests writing two-byte values (128-16383)
func TestWriteVintTwoByte(t *testing.T) {
	testCases := []struct {
		name     string
		value    uint64
		expected []byte
	}{
		{"value_128", 128, []byte{0x81, 0x00}},
		{"value_255", 255, []byte{0x81, 0x7F}},
		{"value_300", 300, []byte{0x82, 0x2C}},
		{"max_two_byte", 16383, []byte{0xFF, 0x7F}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			data := make([]byte, 10)

			// Act
			written := WriteVint(tc.value, data)

			// Assert
			assert.Equal(t, len(tc.expected), written, "written bytes mismatch")
			assert.Equal(t, tc.expected, data[:written], "data mismatch")
		})
	}
}

// TestWriteVintMultiByte tests writing multi-byte values
func TestWriteVintMultiByte(t *testing.T) {
	testCases := []struct {
		name     string
		value    uint64
		expected []byte
	}{
		{"value_16384", 16384, []byte{0x81, 0x80, 0x00}},
		{"value_2097152", 2097152, []byte{0x81, 0x80, 0x80, 0x00}}, // 1 << 21
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			data := make([]byte, 10)

			// Act
			written := WriteVint(tc.value, data)

			// Assert
			assert.Equal(t, len(tc.expected), written, "written bytes mismatch")
			assert.Equal(t, tc.expected, data[:written], "data mismatch")
		})
	}
}

// TestWriteVintBufferTooSmall tests writing when buffer is too small
func TestWriteVintBufferTooSmall(t *testing.T) {
	testCases := []struct {
		name       string
		value      uint64
		bufferSize int
	}{
		{"value_128_size_1", 128, 1},       // needs 2 bytes
		{"value_16384_size_2", 16384, 2},   // needs 3 bytes
		{"large_value_size_5", 1 << 42, 5}, // needs more bytes
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			data := make([]byte, tc.bufferSize)

			// Act
			written := WriteVint(tc.value, data)

			// Assert
			assert.Equal(t, 0, written, "should return 0 when buffer is too small")
		})
	}
}

// TestWriteVintMaxUint64 tests writing the maximum uint64 value
func TestWriteVintMaxUint64(t *testing.T) {
	// Arrange
	value := uint64(0xFFFFFFFFFFFFFFFF)
	data := make([]byte, 10) // max uint64 needs 10 bytes in varint encoding

	// Act
	written := WriteVint(value, data)

	// Assert
	assert.True(t, written > 0, "should successfully write max uint64")
	assert.LessOrEqual(t, written, 10, "should not exceed 10 bytes")
}

// =============================================================================
// Round-Trip Tests (Write then Read)
// =============================================================================

// TestVintRoundTrip tests that writing and reading produces the original value
func TestVintRoundTrip(t *testing.T) {
	testCases := []uint64{
		0,
		1,
		127,
		128,
		255,
		256,
		16383,
		16384,
		2097152,
		268435455,
		1 << 32,
		1 << 56,
		0xFFFFFFFFFFFFFFFF,
	}

	for _, value := range testCases {
		t.Run("", func(t *testing.T) {
			// Arrange
			data := make([]byte, 10)

			// Act - Write
			written := WriteVint(value, data)
			assert.True(t, written > 0, "write should succeed for value %d", value)

			// Act - Read
			readValue, consumed := ReadVint(data[:written])

			// Assert
			assert.Equal(t, written, consumed, "consumed bytes should match written bytes")
			assert.Equal(t, value, readValue, "read value should match original for %d", value)
		})
	}
}

// =============================================================================
// VintEncodedSize Tests
// =============================================================================

// TestVintEncodedSize tests the size calculation function
func TestVintEncodedSize(t *testing.T) {
	testCases := []struct {
		name     string
		value    uint64
		expected int
	}{
		{"zero", 0, 1},
		{"one", 1, 1},
		{"max_1_byte", 127, 1},
		{"min_2_bytes", 128, 2},
		{"max_2_bytes", 16383, 2},
		{"min_3_bytes", 16384, 3},
		{"large_value", 1 << 42, 7},
		{"max_uint64", 0xFFFFFFFFFFFFFFFF, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			size := VintEncodedSize(tc.value)

			// Assert
			assert.Equal(t, tc.expected, size, "size mismatch")
		})
	}
}

// TestVintEncodedSizeMatchesWritten tests that VintEncodedSize matches actual written bytes
func TestVintEncodedSizeMatchesWritten(t *testing.T) {
	testValues := []uint64{
		0, 1, 127, 128, 255, 256, 16383, 16384, 2097152,
		1 << 32, 1 << 56, 0xFFFFFFFFFFFFFFFF,
	}

	for _, value := range testValues {
		// Arrange
		data := make([]byte, 10)

		// Act
		predictedSize := VintEncodedSize(value)
		actualWritten := WriteVint(value, data)

		// Assert
		assert.Equal(t, predictedSize, actualWritten,
			"VintEncodedSize(%d) = %d but WriteVint wrote %d bytes",
			value, predictedSize, actualWritten)
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkWriteVintSmall benchmarks writing small values (0-127)
func BenchmarkWriteVintSmall(b *testing.B) {
	data := make([]byte, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WriteVint(64, data)
	}
}

// BenchmarkWriteVintMedium benchmarks writing medium values (128-16383)
func BenchmarkWriteVintMedium(b *testing.B) {
	data := make([]byte, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WriteVint(8192, data)
	}
}

// BenchmarkWriteVintLarge benchmarks writing large values
func BenchmarkWriteVintLarge(b *testing.B) {
	data := make([]byte, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WriteVint(0xFFFFFFFFFFFFFFFF, data)
	}
}

// BenchmarkReadVintSmall benchmarks reading small values
func BenchmarkReadVintSmall(b *testing.B) {
	data := []byte{0x40} // 64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReadVint(data)
	}
}

// BenchmarkReadVintMedium benchmarks reading medium values
func BenchmarkReadVintMedium(b *testing.B) {
	data := []byte{0xC0, 0x00} // 8192
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReadVint(data)
	}
}

// BenchmarkReadVintLarge benchmarks reading large values
func BenchmarkReadVintLarge(b *testing.B) {
	// Pre-encode max uint64
	encoded := make([]byte, 10)
	size := WriteVint(0xFFFFFFFFFFFFFFFF, encoded)
	data := encoded[:size]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ReadVint(data)
	}
}
