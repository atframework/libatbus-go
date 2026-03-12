package libatbus_buffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStaticBufferBlock(t *testing.T) {
	// Test: Create a new StaticBufferBlock with valid size
	block := NewStaticBufferBlock(100)

	assert.NotNil(t, block)
	assert.Equal(t, 100, block.Size())
	assert.Equal(t, 0, block.Used())
	assert.NotNil(t, block.Data())
	assert.Equal(t, 100, len(block.Data()))
}

func TestNewStaticBufferBlockInvalidSize(t *testing.T) {
	// Test: Creating with zero or negative size returns nil
	assert.Nil(t, NewStaticBufferBlock(0))
	assert.Nil(t, NewStaticBufferBlock(-1))
}

func TestNewStaticBufferBlockWithUsed(t *testing.T) {
	// Test: Create with specified size and used
	block := NewStaticBufferBlockWithUsed(100, 50)

	assert.NotNil(t, block)
	assert.Equal(t, 100, block.Size())
	assert.Equal(t, 50, block.Used())
}

func TestNewStaticBufferBlockWithUsedClamping(t *testing.T) {
	// Test: Used is clamped to valid range
	block1 := NewStaticBufferBlockWithUsed(100, 200)
	assert.Equal(t, 100, block1.Used()) // clamped to size

	block2 := NewStaticBufferBlockWithUsed(100, -10)
	assert.Equal(t, 0, block2.Used()) // clamped to 0
}

func TestNewStaticBufferBlockFromData(t *testing.T) {
	// Test: Create from existing data
	data := []byte("Hello, World!")
	block := NewStaticBufferBlockFromData(data)

	assert.NotNil(t, block)
	assert.Equal(t, len(data), block.Size())
	assert.Equal(t, len(data), block.Used())
	assert.Equal(t, data, block.UsedSpan())

	// Verify data is copied
	data[0] = 'X'
	assert.NotEqual(t, data[0], block.Data()[0])
}

func TestNewStaticBufferBlockFromDataNil(t *testing.T) {
	// Test: Creating from nil or empty data returns nil
	assert.Nil(t, NewStaticBufferBlockFromData(nil))
	assert.Nil(t, NewStaticBufferBlockFromData([]byte{}))
}

func TestStaticBufferBlockSetUsed(t *testing.T) {
	// Test: SetUsed with various values
	block := NewStaticBufferBlock(100)

	block.SetUsed(50)
	assert.Equal(t, 50, block.Used())

	block.SetUsed(200) // exceeds size
	assert.Equal(t, 100, block.Used())

	block.SetUsed(-10) // negative
	assert.Equal(t, 0, block.Used())
}

func TestStaticBufferBlockMaxSpan(t *testing.T) {
	// Test: MaxSpan returns entire buffer
	block := NewStaticBufferBlock(100)
	span := block.MaxSpan()

	assert.NotNil(t, span)
	assert.Equal(t, 100, len(span))
}

func TestStaticBufferBlockUsedSpan(t *testing.T) {
	// Test: UsedSpan returns only used portion
	block := NewStaticBufferBlock(100)
	assert.Nil(t, block.UsedSpan()) // used is 0

	block.SetUsed(50)
	span := block.UsedSpan()
	assert.NotNil(t, span)
	assert.Equal(t, 50, len(span))
}

func TestStaticBufferBlockIsEmpty(t *testing.T) {
	// Test: IsEmpty checks used == 0
	block := NewStaticBufferBlock(100)
	assert.True(t, block.IsEmpty())

	block.SetUsed(10)
	assert.False(t, block.IsEmpty())
}

func TestStaticBufferBlockReset(t *testing.T) {
	// Test: Reset sets used to 0
	block := NewStaticBufferBlockWithUsed(100, 50)
	assert.Equal(t, 50, block.Used())

	block.Reset()
	assert.Equal(t, 0, block.Used())
}

func TestStaticBufferBlockWrite(t *testing.T) {
	// Test: Write appends data
	block := NewStaticBufferBlock(100)

	n := block.Write([]byte("Hello"))
	assert.Equal(t, 5, n)
	assert.Equal(t, 5, block.Used())
	assert.Equal(t, []byte("Hello"), block.UsedSpan())

	n = block.Write([]byte(", World!"))
	assert.Equal(t, 8, n)
	assert.Equal(t, 13, block.Used())
	assert.Equal(t, []byte("Hello, World!"), block.UsedSpan())
}

func TestStaticBufferBlockWriteOverflow(t *testing.T) {
	// Test: Write stops at buffer boundary
	block := NewStaticBufferBlock(10)

	n := block.Write([]byte("Hello, World!")) // 13 bytes
	assert.Equal(t, 10, n)                    // only 10 written
	assert.Equal(t, 10, block.Used())
}

func TestStaticBufferBlockWriteAt(t *testing.T) {
	// Test: WriteAt writes at specific offset
	block := NewStaticBufferBlock(100)

	n := block.WriteAt(10, []byte("Hello"))
	assert.Equal(t, 5, n)
	assert.Equal(t, 0, block.Used()) // Used not updated by WriteAt
	assert.Equal(t, []byte("Hello"), block.Data()[10:15])
}

func TestStaticBufferBlockNilSafety(t *testing.T) {
	// Test: All methods handle nil receiver gracefully
	var block *StaticBufferBlock

	assert.Nil(t, block.Data())
	assert.Equal(t, 0, block.Size())
	assert.Equal(t, 0, block.Used())
	assert.Nil(t, block.MaxSpan())
	assert.Nil(t, block.UsedSpan())
	assert.True(t, block.IsEmpty())
	assert.Equal(t, 0, block.Write([]byte("test")))
	assert.Equal(t, 0, block.WriteAt(0, []byte("test")))

	// These should not panic
	block.SetUsed(10)
	block.Reset()
}

func TestMimallocPaddingSize(t *testing.T) {
	// Test: Size padding follows mimalloc patterns
	testCases := []struct {
		input    int
		expected int
	}{
		{0, 8},        // minimum allocation
		{1, 8},        // tiny: align to 8
		{7, 8},        // tiny: align to 8
		{8, 8},        // tiny: exact 8
		{9, 16},       // tiny: align to 8
		{64, 64},      // tiny: exact 64
		{65, 80},      // small: align to 16
		{100, 112},    // small: align to 16
		{512, 512},    // small: exact 512
		{8192, 8192},  // medium boundary
		{8193, 12288}, // large: align to 4KB
		{10000, 12288},
	}

	for _, tc := range testCases {
		result := MimallocPaddingSize(tc.input)
		assert.GreaterOrEqual(t, result, tc.input, "MimallocPaddingSize(%d) should be >= input", tc.input)
		// Verify alignment patterns
		if tc.input <= 64 && tc.input > 0 {
			assert.Equal(t, 0, result%8, "Tiny sizes should be 8-byte aligned")
		} else if tc.input > 64 && tc.input <= 512 {
			assert.Equal(t, 0, result%16, "Small sizes should be 16-byte aligned")
		} else if tc.input > 8192 {
			assert.Equal(t, 0, result%4096, "Large sizes should be 4KB aligned")
		}
	}
}

func TestAllocateTemporaryBufferBlock(t *testing.T) {
	// Test: Allocate with padding
	block := AllocateTemporaryBufferBlock(100)

	assert.NotNil(t, block)
	assert.GreaterOrEqual(t, block.Size(), 100)
	assert.Equal(t, 100, block.Used())
}

func TestAllocateTemporaryBufferBlockInvalid(t *testing.T) {
	// Test: Invalid size returns nil
	assert.Nil(t, AllocateTemporaryBufferBlock(0))
	assert.Nil(t, AllocateTemporaryBufferBlock(-1))
}

func TestBitWidth(t *testing.T) {
	// Test: bitWidth calculation
	testCases := []struct {
		input    int
		expected int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 3},
		{7, 3},
		{8, 4},
		{15, 4},
		{16, 5},
		{255, 8},
		{256, 9},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, bitWidth(tc.input), "bitWidth(%d)", tc.input)
	}
}
