// Package libatbus_buffer provides buffer management utilities for libatbus.
// This file implements StaticBufferBlock, a Go port of C++ static_buffer_block.
package libatbus_buffer

// StaticBufferBlock is a buffer block designed for temporary buffer allocation.
// It tracks the total allocated size and the used portion.
//
// This is the Go equivalent of C++ static_buffer_block from libatbus.
// Unlike the C++ version which uses unique_ptr<unsigned char[]>, we use a []byte slice.
type StaticBufferBlock struct {
	// The underlying buffer data
	data []byte

	// Total allocated size of the buffer
	size int

	// Number of bytes actually used
	used int
}

// NewStaticBufferBlock creates a new StaticBufferBlock with the given size.
// The buffer is allocated but used is set to 0.
func NewStaticBufferBlock(size int) *StaticBufferBlock {
	if size <= 0 {
		return nil
	}
	return &StaticBufferBlock{
		data: make([]byte, size),
		size: size,
		used: 0,
	}
}

// NewStaticBufferBlockWithUsed creates a StaticBufferBlock with specified size and used.
func NewStaticBufferBlockWithUsed(size, used int) *StaticBufferBlock {
	if size <= 0 {
		return nil
	}
	if used < 0 {
		used = 0
	}
	if used > size {
		used = size
	}
	return &StaticBufferBlock{
		data: make([]byte, size),
		size: size,
		used: used,
	}
}

// NewStaticBufferBlockFromData creates a StaticBufferBlock from existing data.
// The data is copied (not referenced) to avoid aliasing issues.
func NewStaticBufferBlockFromData(data []byte) *StaticBufferBlock {
	if data == nil {
		return nil
	}
	size := len(data)
	if size == 0 {
		return nil
	}
	copied := make([]byte, size)
	copy(copied, data)
	return &StaticBufferBlock{
		data: copied,
		size: size,
		used: size,
	}
}

// Data returns the underlying buffer data pointer (up to size).
func (s *StaticBufferBlock) Data() []byte {
	if s == nil {
		return nil
	}
	return s.data
}

// Size returns the total allocated size of the buffer.
func (s *StaticBufferBlock) Size() int {
	if s == nil {
		return 0
	}
	return s.size
}

// Used returns the number of bytes actually used.
func (s *StaticBufferBlock) Used() int {
	if s == nil {
		return 0
	}
	return s.used
}

// SetUsed sets the used size, clamped to [0, size].
func (s *StaticBufferBlock) SetUsed(used int) {
	if s == nil {
		return
	}
	if used < 0 {
		used = 0
	}
	if used > s.size {
		used = s.size
	}
	s.used = used
}

// MaxSpan returns a slice of the entire allocated buffer.
func (s *StaticBufferBlock) MaxSpan() []byte {
	if s == nil {
		return nil
	}
	return s.data[:s.size]
}

// UsedSpan returns a slice of only the used portion of the buffer.
func (s *StaticBufferBlock) UsedSpan() []byte {
	if s == nil || s.used == 0 {
		return nil
	}
	return s.data[:s.used]
}

// IsEmpty returns true if no bytes are used.
func (s *StaticBufferBlock) IsEmpty() bool {
	return s == nil || s.used == 0
}

// Reset clears the used counter to 0 (does not zero the buffer).
func (s *StaticBufferBlock) Reset() {
	if s != nil {
		s.used = 0
	}
}

// Write appends data to the buffer, updating used.
// Returns the number of bytes written.
func (s *StaticBufferBlock) Write(data []byte) int {
	if s == nil || len(data) == 0 {
		return 0
	}
	available := s.size - s.used
	if available <= 0 {
		return 0
	}
	n := len(data)
	if n > available {
		n = available
	}
	copy(s.data[s.used:], data[:n])
	s.used += n
	return n
}

// WriteAt writes data at a specific offset, not updating used.
// Returns the number of bytes written.
func (s *StaticBufferBlock) WriteAt(offset int, data []byte) int {
	if s == nil || offset < 0 || offset >= s.size || len(data) == 0 {
		return 0
	}
	available := s.size - offset
	n := len(data)
	if n > available {
		n = available
	}
	copy(s.data[offset:], data[:n])
	return n
}

// MimallocPaddingSize calculates padded size following mimalloc-style size class patterns.
// This helps reduce memory fragmentation and improve malloc efficiency.
func MimallocPaddingSize(originSize int) int {
	const wordSize = 8     // 8 bytes on 64-bit
	const minAllocSize = 8 // Minimum allocation
	const smallPageSize = 4096

	if originSize <= 0 {
		return minAllocSize
	}

	// Tiny allocations (<=64 bytes): align to word size (8 bytes)
	if originSize <= 64 {
		return (originSize + wordSize - 1) & ^(wordSize - 1)
	}

	// Small allocations (65-512 bytes): align to 16 bytes (for SIMD)
	if originSize <= 512 {
		return (originSize + 15) & ^15
	}

	// Medium allocations (513 bytes - 8KB): use mimalloc-style 12.5% spacing
	if originSize <= 8192 {
		wsize := (originSize + wordSize - 1) / wordSize
		b := bitWidth(wsize - 1)

		var bin int
		if b < 3 {
			bin = wsize
		} else {
			bin = (b << 2) + (((wsize - 1) >> (b - 2)) & 0x03)
		}

		binB := bin >> 2
		binExtra := bin & 0x03
		var resultWsize int
		if binB < 3 {
			resultWsize = bin
		} else {
			resultWsize = (1 << (binB - 1)) + ((binExtra + 1) << (binB - 3))
		}
		return resultWsize * wordSize
	}

	// Large allocations (>8KB): align to page size (4KB)
	return (originSize + smallPageSize - 1) & ^(smallPageSize - 1)
}

// bitWidth returns the number of bits needed to represent x.
func bitWidth(x int) int {
	if x <= 0 {
		return 0
	}
	n := 0
	for x > 0 {
		x >>= 1
		n++
	}
	return n
}

// AllocateTemporaryBufferBlock allocates a temporary buffer with size padding.
func AllocateTemporaryBufferBlock(originSize int) *StaticBufferBlock {
	if originSize <= 0 {
		return nil
	}
	realSize := MimallocPaddingSize(originSize)
	return &StaticBufferBlock{
		data: make([]byte, realSize),
		size: realSize,
		used: originSize,
	}
}
