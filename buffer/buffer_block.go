package libatbus_buffer

import (
	"errors"
)

// DataAlignSize is the alignment size for buffer data
const DataAlignSize = 8

// Error codes
var (
	ErrSuccess    = errors.New("success")
	ErrNoData     = errors.New("no data available")
	ErrBuffLimit  = errors.New("buffer limit reached")
	ErrMalloc     = errors.New("memory allocation failed")
	ErrInvalidArg = errors.New("invalid argument")
)

// BufferBlock represents a buffer block with size and used tracking
// It manages a slice of bytes with a "used" offset for pop operations
type BufferBlock struct {
	data []byte // The underlying data buffer
	used int    // How many bytes have been "popped" from the front
}

// NewBufferBlock creates a new BufferBlock with the specified size
func NewBufferBlock(size int) *BufferBlock {
	if size <= 0 {
		return nil
	}
	return &BufferBlock{
		data: make([]byte, size),
		used: 0,
	}
}

// NewBufferBlockFromSlice creates a BufferBlock wrapping an existing slice
func NewBufferBlockFromSlice(data []byte) *BufferBlock {
	if data == nil {
		return nil
	}
	return &BufferBlock{
		data: data,
		used: 0,
	}
}

// Data returns the unread portion of the buffer (after used offset)
func (b *BufferBlock) Data() []byte {
	if b == nil || b.used >= len(b.data) {
		return nil
	}
	return b.data[b.used:]
}

// RawData returns the entire buffer from the beginning
func (b *BufferBlock) RawData() []byte {
	if b == nil {
		return nil
	}
	return b.data
}

// Size returns the remaining size (total - used)
func (b *BufferBlock) Size() int {
	if b == nil {
		return 0
	}
	remaining := len(b.data) - b.used
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RawSize returns the total size of the buffer
func (b *BufferBlock) RawSize() int {
	if b == nil {
		return 0
	}
	return len(b.data)
}

// Pop advances the used pointer by s bytes, reducing the available size
// Returns the new data slice after the pop
func (b *BufferBlock) Pop(s int) []byte {
	if b == nil {
		return nil
	}

	if b.used+s > len(b.data) {
		b.used = len(b.data)
	} else {
		b.used += s
	}

	return b.Data()
}

// Reset resets the used counter to 0
func (b *BufferBlock) Reset() {
	if b != nil {
		b.used = 0
	}
}

// Used returns how many bytes have been popped
func (b *BufferBlock) Used() int {
	if b == nil {
		return 0
	}
	return b.used
}

// SetUsed sets the used counter directly
func (b *BufferBlock) SetUsed(used int) {
	if b == nil {
		return
	}
	if used < 0 {
		used = 0
	}
	if used > len(b.data) {
		used = len(b.data)
	}
	b.used = used
}

// Clone creates a deep copy of the buffer block
func (b *BufferBlock) Clone() *BufferBlock {
	if b == nil {
		return nil
	}
	newData := make([]byte, len(b.data))
	copy(newData, b.data)
	return &BufferBlock{
		data: newData,
		used: b.used,
	}
}

// PaddingSize calculates the padded size aligned to DataAlignSize
func PaddingSize(s int) int {
	pl := s % DataAlignSize
	if pl == 0 {
		return s
	}
	return s + DataAlignSize - pl
}

// HeadSize returns the overhead size for a buffer block header
// In Go, we don't need actual header storage since we use struct fields,
// but we keep this for compatibility with size calculations
func HeadSize(s int) int {
	// Estimate: 3 fields (pointer + 2 ints) ~ 24 bytes on 64-bit, padded
	return PaddingSize(24)
}

// FullSize returns the total size needed for a buffer block of size s
func FullSize(s int) int {
	return HeadSize(s) + PaddingSize(s)
}
