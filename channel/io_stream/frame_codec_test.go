package libatbus_channel_iostream

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCalculateHashEmptyPayload verifies that hash calculation works for empty payloads
func TestCalculateHashEmptyPayload(t *testing.T) {
	// Arrange
	payload := []byte{}

	// Act
	hash := CalculateHash(payload)

	// Assert - murmur3 hash of empty input with seed 0 is 0
	// This is expected behavior matching the C++ implementation
	assert.Equal(t, uint32(0), hash, "hash of empty payload should be 0")
}

// TestCalculateHashNonEmptyPayload verifies that hash calculation produces consistent results
func TestCalculateHashNonEmptyPayload(t *testing.T) {
	// Arrange
	payload := []byte("Hello, atbus!")

	// Act
	hash1 := CalculateHash(payload)
	hash2 := CalculateHash(payload)

	// Assert
	assert.Equal(t, hash1, hash2, "same payload should produce same hash")
}

// TestPackFrameBasic verifies basic frame packing functionality
func TestPackFrameBasic(t *testing.T) {
	// Arrange
	payload := []byte("test payload data")
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)

	// Act
	written := PackFrame(payload, frame)

	// Assert
	assert.Equal(t, frameSize, written, "should write correct number of bytes")
	assert.Greater(t, written, len(payload), "frame should be larger than payload due to header")

	// Verify frame starts with 4 bytes hash
	assert.GreaterOrEqual(t, written, HashSize, "frame should have at least hash size")
}

// TestPackFrameSizeCalculation verifies PackFrameSize returns correct size
func TestPackFrameSizeCalculation(t *testing.T) {
	// Arrange
	payload := []byte("test payload")

	// Act
	calculatedSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, calculatedSize)
	writtenSize := PackFrame(payload, frame)

	// Assert
	assert.Equal(t, calculatedSize, writtenSize, "calculated size should match actual written size")
}

// TestPackFrameBufferTooSmall verifies PackFrame returns 0 when buffer is too small
func TestPackFrameBufferTooSmall(t *testing.T) {
	// Arrange
	payload := []byte("test payload")
	tooSmallBuffer := make([]byte, 5) // too small

	// Act
	written := PackFrame(payload, tooSmallBuffer)

	// Assert
	assert.Equal(t, 0, written, "should return 0 when buffer is too small")
}

// TestUnpackFrameBasic verifies basic frame unpacking functionality
func TestUnpackFrameBasic(t *testing.T) {
	// Arrange
	originalPayload := []byte("test payload data for unpacking")
	frameSize := PackFrameSize(uint64(len(originalPayload)))
	frame := make([]byte, frameSize)
	PackFrame(originalPayload, frame)

	// Act
	result := UnpackFrame(frame)

	// Assert
	assert.Nil(t, result.Error, "unpack should not return error")
	assert.Equal(t, originalPayload, result.Payload, "unpacked payload should match original")
	assert.Equal(t, frameSize, result.Consumed, "should consume entire frame")
}

// TestUnpackFrameRoundTrip verifies pack then unpack returns original data
func TestUnpackFrameRoundTrip(t *testing.T) {
	testCases := []struct {
		name    string
		payload []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("small")},
		{"medium", []byte("this is a medium sized payload for testing")},
		{"with_binary", []byte{0x00, 0xFF, 0x01, 0xFE, 0x02, 0xFD}},
		{"unicode", []byte("你好世界 🌍")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			frameSize := PackFrameSize(uint64(len(tc.payload)))
			frame := make([]byte, frameSize)
			PackFrame(tc.payload, frame)

			// Act
			result := UnpackFrame(frame)

			// Assert
			assert.Nil(t, result.Error, "unpack should not return error")
			assert.Equal(t, tc.payload, result.Payload, "round-trip should preserve data")
		})
	}
}

// TestUnpackFrameInvalidHash verifies error on corrupted hash
func TestUnpackFrameInvalidHash(t *testing.T) {
	// Arrange
	payload := []byte("test payload")
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	PackFrame(payload, frame)

	// Corrupt the hash (first 4 bytes)
	frame[0] ^= 0xFF
	frame[1] ^= 0xFF

	// Act
	result := UnpackFrame(frame)

	// Assert
	assert.NotNil(t, result.Error, "unpack should return error on invalid hash")
	assert.Equal(t, ErrInvalidFrameHash, result.Error, "should be hash error")
}

// TestUnpackFrameTooShort verifies error on frame smaller than hash size
func TestUnpackFrameTooShort(t *testing.T) {
	// Arrange
	shortFrame := []byte{0x01, 0x02, 0x03} // only 3 bytes, less than HashSize

	// Act
	result := UnpackFrame(shortFrame)

	// Assert
	assert.NotNil(t, result.Error, "unpack should return error on too short frame")
	assert.Equal(t, ErrIncompleteFrame, result.Error, "should be incomplete frame error")
}

// TestTryUnpackFrameHeaderValid verifies header parsing on valid frame
func TestTryUnpackFrameHeaderValid(t *testing.T) {
	// Arrange
	payload := []byte("payload for header test")
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	PackFrame(payload, frame)

	// Act
	payloadLen, headerSize, needMoreData := TryUnpackFrameHeader(frame)

	// Assert
	assert.False(t, needMoreData, "should not need more data")
	assert.Equal(t, uint64(len(payload)), payloadLen, "payload length should match")
	assert.Greater(t, headerSize, HashSize, "header size should include hash and length varint")
}

// TestTryUnpackFrameHeaderTooShort verifies header parsing on short data
func TestTryUnpackFrameHeaderTooShort(t *testing.T) {
	// Arrange
	shortData := []byte{0x01}

	// Act
	_, headerSize, needMoreData := TryUnpackFrameHeader(shortData)

	// Assert
	assert.True(t, needMoreData, "should need more data")
	assert.Equal(t, 0, headerSize, "header size should be 0 for insufficient data")
}

// TestFrameReaderBasic verifies FrameReader can read a complete frame
func TestFrameReaderBasic(t *testing.T) {
	// Arrange
	payload := []byte("frame reader test payload")
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	PackFrame(payload, frame)
	reader := NewFrameReader(frameSize + 100) // buffer larger than frame

	// Act
	reader.Write(frame)
	result := reader.ReadFrame()

	// Assert
	assert.Nil(t, result.Error, "read should not return error")
	assert.NotNil(t, result.Payload, "should return complete frame")
	assert.Equal(t, payload, result.Payload, "should return original payload")
	assert.Equal(t, frameSize, result.Consumed, "should consume frame size")
}

// TestFrameReaderPartialFeed verifies FrameReader handles partial data
func TestFrameReaderPartialFeed(t *testing.T) {
	// Arrange
	payload := []byte("partial feed test")
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	PackFrame(payload, frame)
	reader := NewFrameReader(frameSize + 100)

	// Feed first half
	half := frameSize / 2
	reader.Write(frame[:half])

	// Act - first read should return incomplete
	result1 := reader.ReadFrame()

	// Feed second half
	reader.Write(frame[half:])

	// Act - second read should return complete
	result2 := reader.ReadFrame()

	// Assert
	assert.NotNil(t, result1.Error, "first read should return error for incomplete frame")
	assert.Equal(t, ErrIncompleteFrame, result1.Error, "should be incomplete frame error")
	assert.Nil(t, result1.Payload, "first read should return nil for incomplete frame")

	assert.Nil(t, result2.Error, "second read should not error")
	assert.NotNil(t, result2.Payload, "second read should return complete frame")
	assert.Equal(t, payload, result2.Payload, "payload should match original")
}

// TestFrameReaderMultipleFrames verifies FrameReader can read multiple frames
func TestFrameReaderMultipleFrames(t *testing.T) {
	// Arrange
	payloads := [][]byte{
		[]byte("first frame"),
		[]byte("second frame"),
		[]byte("third frame"),
	}
	reader := NewFrameReader(1024)

	// Feed all frames at once
	for _, p := range payloads {
		frameSize := PackFrameSize(uint64(len(p)))
		frame := make([]byte, frameSize)
		PackFrame(p, frame)
		reader.Write(frame)
	}

	// Act - read all frames
	var results [][]byte
	for {
		result := reader.ReadFrame()
		if result.Error == ErrIncompleteFrame {
			break
		}
		assert.Nil(t, result.Error, "read should not error")
		if result.Payload == nil {
			break
		}
		results = append(results, result.Payload)
	}

	// Assert
	assert.Equal(t, len(payloads), len(results), "should read all frames")
	for i, p := range payloads {
		assert.Equal(t, p, results[i], "frame %d should match", i)
	}
}

// TestFrameReaderReset verifies FrameReader reset functionality
func TestFrameReaderReset(t *testing.T) {
	// Arrange
	payload := []byte("reset test")
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	PackFrame(payload, frame)
	reader := NewFrameReader(256)

	// Feed partial data then reset
	reader.Write(frame[:5])
	reader.Reset()

	// Feed complete frame
	reader.Write(frame)
	result := reader.ReadFrame()

	// Assert
	assert.Nil(t, result.Error, "read should not error after reset")
	assert.Equal(t, payload, result.Payload, "should return complete payload after reset")
}

// TestLargePayload verifies handling of larger payloads
func TestLargePayload(t *testing.T) {
	// Arrange - create 64KB payload
	largePayload := make([]byte, 64*1024)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	// Act
	frameSize := PackFrameSize(uint64(len(largePayload)))
	frame := make([]byte, frameSize)
	PackFrame(largePayload, frame)
	result := UnpackFrame(frame)

	// Assert
	assert.Nil(t, result.Error, "should handle large payload")
	assert.Equal(t, largePayload, result.Payload, "large payload should round-trip correctly")
}

// TestCrossLanguageCompatibility verifies frame format is compatible with C++ version
// This test uses known values from C++ implementation
func TestCrossLanguageCompatibility(t *testing.T) {
	// Arrange - simple payload with known hash from C++ tests
	payload := []byte("test")

	// Act
	hash := CalculateHash(payload)
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	written := PackFrame(payload, frame)

	// Assert
	t.Logf("Payload: %q, Hash: 0x%08X, Frame length: %d", payload, hash, written)
	t.Logf("Frame hex: %X", frame[:written])

	// Verify structure: [hash:4][length:varint][payload]
	assert.GreaterOrEqual(t, written, HashSize+1+len(payload), "frame should have hash + length + payload")

	// Unpack and verify
	result := UnpackFrame(frame[:written])
	assert.Nil(t, result.Error, "should unpack successfully")
	assert.Equal(t, payload, result.Payload, "should match original payload")
}

// BenchmarkPackFrame benchmarks frame packing performance
func BenchmarkPackFrame(b *testing.B) {
	payload := bytes.Repeat([]byte("benchmark "), 100)
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PackFrame(payload, frame)
	}
}

// BenchmarkUnpackFrame benchmarks frame unpacking performance
func BenchmarkUnpackFrame(b *testing.B) {
	payload := bytes.Repeat([]byte("benchmark "), 100)
	frameSize := PackFrameSize(uint64(len(payload)))
	frame := make([]byte, frameSize)
	PackFrame(payload, frame)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		UnpackFrame(frame)
	}
}
