// Package libatbus_channel_iostream provides IO stream channel implementation for libatbus.
// It handles TCP/Unix socket/Named pipe connections with proper frame encoding/decoding.
package libatbus_channel_iostream

import (
	"encoding/binary"
	"errors"

	buffer "github.com/atframework/libatbus-go/buffer"
	error_code "github.com/atframework/libatbus-go/error_code"
	"github.com/spaolacci/murmur3"
)

// Frame format:
// +----------------+----------------+----------------+
// | Hash (4 bytes) | Length (varint)| Payload        |
// +----------------+----------------+----------------+
//
// - Hash: 32-bit murmur3 hash of payload (little-endian), seed = 0
// - Length: Variable-length encoded integer (libatbus custom vint, NOT protobuf varint)
// - Payload: The actual message data

const (
	// HashSize is the size of the hash field in bytes (murmur3_x86_32)
	HashSize = 4
	// MaxVintSize is the maximum size of a varint encoded value (for uint64)
	MaxVintSize = 10
	// MaxFrameHeaderSize is the maximum possible frame header size
	MaxFrameHeaderSize = HashSize + MaxVintSize
)

var (
	// ErrBufferTooSmall indicates the buffer is too small to hold the frame
	ErrBufferTooSmall = errors.New("buffer too small for frame")
	// ErrInvalidFrameHash indicates the frame hash verification failed
	ErrInvalidFrameHash = errors.New("invalid frame hash")
	// ErrInvalidFrameLength indicates the frame length is invalid
	ErrInvalidFrameLength = errors.New("invalid frame length")
	// ErrIncompleteFrame indicates the frame is incomplete (need more data)
	ErrIncompleteFrame = errors.New("incomplete frame data")
)

// CalculateHash calculates the murmur3_x86_32 hash of the payload with seed 0.
// This matches the C++ implementation: atfw::util::hash::murmur_hash3_x86_32(buf, len, 0)
func CalculateHash(payload []byte) uint32 {
	return murmur3.Sum32(payload)
}

// PackFrameSize returns the total size needed to pack a frame with the given payload size.
func PackFrameSize(payloadSize uint64) int {
	return HashSize + buffer.VintEncodedSize(payloadSize) + int(payloadSize)
}

// PackFrame packs payload into a frame with hash and length prefix.
// Returns the number of bytes written, or 0 if buffer is too small.
//
// The frame format is:
//   - 4 bytes: murmur3_x86_32 hash of payload (little-endian)
//   - varint: payload length (libatbus custom vint format)
//   - payload: the actual data
func PackFrame(payload []byte, frame []byte) int {
	payloadLen := uint64(len(payload))
	vintSize := buffer.VintEncodedSize(payloadLen)
	totalSize := HashSize + vintSize + len(payload)

	if len(frame) < totalSize {
		return 0
	}

	// Write hash (little-endian, matching C++ memcpy behavior)
	hash := CalculateHash(payload)
	binary.LittleEndian.PutUint32(frame[0:HashSize], hash)

	// Write varint length
	vintWritten := buffer.WriteVint(payloadLen, frame[HashSize:HashSize+vintSize])
	if vintWritten != vintSize {
		return 0
	}

	// Write payload
	copy(frame[HashSize+vintSize:], payload)

	return totalSize
}

// UnpackFrameResult contains the result of unpacking a frame.
type UnpackFrameResult struct {
	// Payload is the extracted payload data
	Payload []byte
	// Consumed is the total number of bytes consumed from the input buffer
	Consumed int
	// Error indicates any error that occurred during unpacking
	Error error
	// ErrorCode is the libatbus error code
	ErrorCode error_code.ErrorType
}

// UnpackFrame attempts to unpack a frame from the input buffer.
// Returns the unpacked result including payload, consumed bytes, and any error.
//
// If the buffer doesn't contain a complete frame, Error will be ErrIncompleteFrame.
// If the hash verification fails, Error will be ErrInvalidFrameHash.
func UnpackFrame(data []byte) UnpackFrameResult {
	result := UnpackFrameResult{
		ErrorCode: error_code.EN_ATBUS_ERR_SUCCESS,
	}

	// Need at least hash + 1 byte for varint
	if len(data) < HashSize+1 {
		result.Error = ErrIncompleteFrame
		result.ErrorCode = error_code.EN_ATBUS_ERR_NO_DATA
		return result
	}

	// Read hash (little-endian)
	expectedHash := binary.LittleEndian.Uint32(data[0:HashSize])

	// Read varint length
	payloadLen, vintSize := buffer.ReadVint(data[HashSize:])
	if vintSize == 0 {
		// Varint is incomplete, need more data
		result.Error = ErrIncompleteFrame
		result.ErrorCode = error_code.EN_ATBUS_ERR_NO_DATA
		return result
	}

	// Check if we have the complete payload
	totalFrameSize := HashSize + vintSize + int(payloadLen)
	if len(data) < totalFrameSize {
		result.Error = ErrIncompleteFrame
		result.ErrorCode = error_code.EN_ATBUS_ERR_NO_DATA
		return result
	}

	// Extract payload
	payloadStart := HashSize + vintSize
	result.Payload = data[payloadStart : payloadStart+int(payloadLen)]
	result.Consumed = totalFrameSize

	// Verify hash
	actualHash := CalculateHash(result.Payload)
	if actualHash != expectedHash {
		result.Error = ErrInvalidFrameHash
		result.ErrorCode = error_code.EN_ATBUS_ERR_BAD_DATA
		return result
	}

	return result
}

// TryUnpackFrameHeader tries to parse just the frame header (hash + length) without validating hash.
// This is useful for pre-allocating buffer space for large messages.
// Returns:
//   - payloadLen: the payload length if header is complete
//   - headerSize: the size of the header (hash + varint) if complete, 0 otherwise
//   - needMoreData: true if more data is needed to parse the header
func TryUnpackFrameHeader(data []byte) (payloadLen uint64, headerSize int, needMoreData bool) {
	// Need at least hash + 1 byte for varint
	if len(data) < HashSize+1 {
		return 0, 0, true
	}

	// Read varint length
	payloadLen, vintSize := buffer.ReadVint(data[HashSize:])
	if vintSize == 0 {
		// Varint is incomplete, need more data
		return 0, 0, true
	}

	headerSize = HashSize + vintSize
	return payloadLen, headerSize, false
}

// FrameReader provides a streaming frame reader that accumulates data and extracts complete frames.
type FrameReader struct {
	// Buffer for accumulating incoming data
	buf []byte
	// Current read position in buffer
	readPos int
	// Current write position in buffer
	writePos int
}

// NewFrameReader creates a new FrameReader with the specified initial buffer capacity.
func NewFrameReader(initialCapacity int) *FrameReader {
	return &FrameReader{
		buf: make([]byte, initialCapacity),
	}
}

// Write appends data to the reader's buffer, growing it if necessary.
// Returns the number of bytes written.
func (r *FrameReader) Write(data []byte) int {
	needed := r.writePos + len(data)
	if needed > len(r.buf) {
		// Grow buffer
		newSize := len(r.buf) * 2
		if newSize < needed {
			newSize = needed
		}
		newBuf := make([]byte, newSize)
		copy(newBuf, r.buf[r.readPos:r.writePos])
		r.buf = newBuf
		r.writePos -= r.readPos
		r.readPos = 0
	}
	n := copy(r.buf[r.writePos:], data)
	r.writePos += n
	return n
}

// ReadFrame attempts to read a complete frame from the buffer.
// Returns the unpacked result. If no complete frame is available,
// Error will be ErrIncompleteFrame.
func (r *FrameReader) ReadFrame() UnpackFrameResult {
	available := r.buf[r.readPos:r.writePos]
	result := UnpackFrame(available)

	if result.Error == nil && result.Consumed > 0 {
		r.readPos += result.Consumed

		// Compact buffer if too much space is wasted
		if r.readPos > len(r.buf)/2 {
			remaining := r.writePos - r.readPos
			copy(r.buf, r.buf[r.readPos:r.writePos])
			r.readPos = 0
			r.writePos = remaining
		}
	}

	return result
}

// Available returns the number of bytes available for reading.
func (r *FrameReader) Available() int {
	return r.writePos - r.readPos
}

// Reset clears the reader's buffer.
func (r *FrameReader) Reset() {
	r.readPos = 0
	r.writePos = 0
}
