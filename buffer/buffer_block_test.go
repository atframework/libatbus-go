package libatbus_buffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// BufferBlock Tests
// =============================================================================

// TestBufferBlockNew tests creating a new buffer block
func TestBufferBlockNew(t *testing.T) {
	// Test normal creation
	block := NewBufferBlock(99)
	assert.NotNil(t, block, "block should not be nil")
	assert.Equal(t, 99, block.Size(), "size should be 99")
	assert.Equal(t, 99, block.RawSize(), "raw size should be 99")
	assert.Equal(t, 0, block.Used(), "used should be 0")

	// Test zero size
	nilBlock := NewBufferBlock(0)
	assert.Nil(t, nilBlock, "block should be nil for size 0")

	// Test negative size
	nilBlock = NewBufferBlock(-1)
	assert.Nil(t, nilBlock, "block should be nil for negative size")
}

// TestBufferBlockFromSlice tests creating a buffer block from existing slice
func TestBufferBlockFromSlice(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5}
	block := NewBufferBlockFromSlice(data)
	assert.NotNil(t, block, "block should not be nil")
	assert.Equal(t, 5, block.Size(), "size should be 5")
	assert.Equal(t, data, block.RawData(), "raw data should match")

	// Test nil slice
	nilBlock := NewBufferBlockFromSlice(nil)
	assert.Nil(t, nilBlock, "block should be nil for nil slice")
}

// TestBufferBlockPop tests the pop operation
func TestBufferBlockPop(t *testing.T) {
	block := NewBufferBlock(99)
	assert.NotNil(t, block)

	// Pop 50 bytes
	block.Pop(50)
	assert.Equal(t, 49, block.Size(), "size should be 49 after pop 50")
	assert.Equal(t, 99, block.RawSize(), "raw size should still be 99")
	assert.Equal(t, 50, block.Used(), "used should be 50")

	// Data should start at offset 50
	data := block.Data()
	rawData := block.RawData()
	assert.Equal(t, rawData[50:], data, "data should start at offset 50")

	// Pop more than remaining
	block.Pop(100)
	assert.Equal(t, 0, block.Size(), "size should be 0 after popping more than available")
	assert.Equal(t, 99, block.RawSize(), "raw size should still be 99")
	assert.Equal(t, 99, block.Used(), "used should be 99")
}

// TestBufferBlockReset tests the reset operation
func TestBufferBlockReset(t *testing.T) {
	block := NewBufferBlock(99)
	block.Pop(50)
	assert.Equal(t, 50, block.Used())

	block.Reset()
	assert.Equal(t, 0, block.Used(), "used should be 0 after reset")
	assert.Equal(t, 99, block.Size(), "size should be restored after reset")
}

// TestBufferBlockClone tests the clone operation
func TestBufferBlockClone(t *testing.T) {
	block := NewBufferBlock(99)
	// Fill with data
	for i := 0; i < 99; i++ {
		block.RawData()[i] = byte(i)
	}
	block.Pop(10)

	clone := block.Clone()
	assert.NotNil(t, clone)
	assert.Equal(t, block.Size(), clone.Size())
	assert.Equal(t, block.Used(), clone.Used())
	assert.Equal(t, block.RawData(), clone.RawData())

	// Modify clone should not affect original
	clone.RawData()[0] = 255
	assert.NotEqual(t, block.RawData()[0], clone.RawData()[0])
}

// TestBufferBlockNil tests operations on nil block
func TestBufferBlockNil(t *testing.T) {
	var block *BufferBlock = nil

	assert.Nil(t, block.Data())
	assert.Nil(t, block.RawData())
	assert.Equal(t, 0, block.Size())
	assert.Equal(t, 0, block.RawSize())
	assert.Equal(t, 0, block.Used())
	assert.Nil(t, block.Pop(10))
	assert.Nil(t, block.Clone())
}

// TestPaddingSize tests the padding size calculation
func TestPaddingSize(t *testing.T) {
	// Already aligned
	assert.Equal(t, 8, PaddingSize(8))
	assert.Equal(t, 16, PaddingSize(16))

	// Needs padding
	assert.Equal(t, 8, PaddingSize(1))
	assert.Equal(t, 8, PaddingSize(7))
	assert.Equal(t, 16, PaddingSize(9))
	assert.Equal(t, 16, PaddingSize(15))

	// Zero
	assert.Equal(t, 0, PaddingSize(0))
}

// TestFullSize tests the full size calculation
func TestFullSize(t *testing.T) {
	// Full size should be head size + padding size
	for _, s := range []int{1, 8, 99, 256} {
		fullSize := FullSize(s)
		assert.Equal(t, HeadSize(s)+PaddingSize(s), fullSize)
	}
}

// =============================================================================
// BufferManager Dynamic Mode Tests
// =============================================================================

// TestBufferManagerDynamicEmpty tests empty state in dynamic mode
func TestBufferManagerDynamicEmpty(t *testing.T) {
	mgr := NewBufferManager()
	assert.True(t, mgr.Empty(), "new manager should be empty")
	assert.True(t, mgr.IsDynamicMode(), "should be in dynamic mode")
	assert.False(t, mgr.IsStaticMode(), "should not be in static mode")
}

// TestBufferManagerDynamicSetLimit tests setting limits in dynamic mode
func TestBufferManagerDynamicSetLimit(t *testing.T) {
	mgr := NewBufferManager()
	assert.True(t, mgr.SetLimit(1023, 10), "SetLimit should succeed in dynamic mode")

	limit := mgr.Limit()
	assert.Equal(t, 1023, limit.LimitSize)
	assert.Equal(t, 10, limit.LimitNumber)
}

// TestBufferManagerDynamicPushBack tests push back in dynamic mode (CASE_TEST: dynamic_buffer_manager_bf)
func TestBufferManagerDynamicPushBack(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1023, 10)

	// Size limit test
	data, err := mgr.PushBack(256)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	data, err = mgr.PushBack(256)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	data, err = mgr.PushBack(256)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	// Should hit size limit
	data, err = mgr.PushBack(256)
	assert.ErrorIs(t, err, ErrBuffLimit)
	assert.Nil(t, data)

	// Should fit exactly
	data, err = mgr.PushBack(255)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	limit := mgr.Limit()
	assert.Equal(t, 1023, limit.CostSize)
	assert.Equal(t, 4, limit.CostNumber)
}

// TestBufferManagerDynamicPushPopCycle tests push/pop cycle in dynamic mode
func TestBufferManagerDynamicPushPopCycle(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1023, 3)

	// Run cycle 3 times to test reuse
	for cycle := 0; cycle < 3; cycle++ {
		// Push 3 blocks
		data0, err := mgr.PushBack(99)
		assert.NoError(t, err)
		fillBytes(data0, 0xFF)

		data1, err := mgr.PushBack(28)
		assert.NoError(t, err)
		fillBytes(data1, 0x00)

		data2, err := mgr.PushBack(17)
		assert.NoError(t, err)
		fillBytes(data2, 0xFF)

		// Should hit limit
		_, err = mgr.PushBack(63)
		assert.ErrorIs(t, err, ErrBuffLimit)

		limit := mgr.Limit()
		assert.Equal(t, 144, limit.CostSize)
		assert.Equal(t, 3, limit.CostNumber)

		// Pop and check front
		frontData, _, nwrite, err := mgr.FrontData()
		assert.NoError(t, err)
		assert.Equal(t, 99, nwrite)
		assert.Equal(t, byte(0xFF), frontData[0])

		err = mgr.PopFront(128, true)
		assert.NoError(t, err)

		limit = mgr.Limit()
		assert.Equal(t, 45, limit.CostSize)
		assert.Equal(t, 2, limit.CostNumber)

		// Check next front
		frontData, _, nwrite, err = mgr.FrontData()
		assert.NoError(t, err)
		assert.Equal(t, 28, nwrite)
		assert.Equal(t, byte(0x00), frontData[0])

		err = mgr.PopFront(100, true)
		assert.NoError(t, err)

		limit = mgr.Limit()
		assert.Equal(t, 17, limit.CostSize)
		assert.Equal(t, 1, limit.CostNumber)

		// Check last block
		frontData, _, nwrite, err = mgr.FrontData()
		assert.NoError(t, err)
		assert.Equal(t, 17, nwrite)
		assert.Equal(t, byte(0xFF), frontData[0])

		// Pop partial (not removing block)
		err = mgr.PopFront(10, false)
		assert.NoError(t, err)
		limit = mgr.Limit()
		assert.Equal(t, 7, limit.CostSize)
		assert.Equal(t, 1, limit.CostNumber)

		frontData, _, nwrite, err = mgr.FrontData()
		assert.NoError(t, err)
		assert.Equal(t, 7, nwrite)

		// Pop all
		err = mgr.PopFront(10, true)
		assert.NoError(t, err)
		assert.True(t, mgr.Empty())
		limit = mgr.Limit()
		assert.Equal(t, 0, limit.CostSize)
		assert.Equal(t, 0, limit.CostNumber)

		// Pop from empty
		_, _, _, err = mgr.FrontData()
		assert.ErrorIs(t, err, ErrNoData)

		err = mgr.PopFront(10, true)
		assert.ErrorIs(t, err, ErrNoData)
	}
}

// TestBufferManagerDynamicPushFront tests push front in dynamic mode
func TestBufferManagerDynamicPushFront(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1023, 10)

	// Size limit test
	data, err := mgr.PushFront(256)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	data, err = mgr.PushFront(256)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	data, err = mgr.PushFront(256)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	// Should hit size limit
	data, err = mgr.PushFront(256)
	assert.ErrorIs(t, err, ErrBuffLimit)
	assert.Nil(t, data)

	// Should fit exactly
	data, err = mgr.PushFront(255)
	assert.NoError(t, err)
	assert.NotNil(t, data)

	limit := mgr.Limit()
	assert.Equal(t, 1023, limit.CostSize)
	assert.Equal(t, 4, limit.CostNumber)
}

// TestBufferManagerDynamicPushFrontPopBackCycle tests push front/pop back cycle
func TestBufferManagerDynamicPushFrontPopBackCycle(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1023, 3)

	// Run cycle 3 times
	for cycle := 0; cycle < 3; cycle++ {
		// Push 3 blocks from front
		data0, err := mgr.PushFront(99)
		assert.NoError(t, err)
		fillBytes(data0, 0xFF)

		data1, err := mgr.PushFront(28)
		assert.NoError(t, err)
		fillBytes(data1, 0x00)

		data2, err := mgr.PushFront(17)
		assert.NoError(t, err)
		fillBytes(data2, 0xFF)

		// Should hit limit
		_, err = mgr.PushFront(63)
		assert.ErrorIs(t, err, ErrBuffLimit)

		limit := mgr.Limit()
		assert.Equal(t, 144, limit.CostSize)
		assert.Equal(t, 3, limit.CostNumber)

		// Pop from back
		backData, _, nwrite, err := mgr.BackData()
		assert.NoError(t, err)
		assert.Equal(t, 99, nwrite)
		assert.Equal(t, byte(0xFF), backData[0])

		err = mgr.PopBack(128, true)
		assert.NoError(t, err)

		limit = mgr.Limit()
		assert.Equal(t, 45, limit.CostSize)
		assert.Equal(t, 2, limit.CostNumber)

		// Continue popping
		err = mgr.PopBack(100, true)
		assert.NoError(t, err)

		err = mgr.PopBack(100, true)
		assert.NoError(t, err)

		assert.True(t, mgr.Empty())
	}
}

// =============================================================================
// BufferManager Static Mode Tests
// =============================================================================

// TestBufferManagerStaticSetMode tests setting static mode
func TestBufferManagerStaticSetMode(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetMode(1023, 10)

	assert.True(t, mgr.IsStaticMode(), "should be in static mode")
	assert.False(t, mgr.IsDynamicMode(), "should not be in dynamic mode")

	limit := mgr.Limit()
	assert.Equal(t, 1023, limit.LimitSize)
	assert.Equal(t, 10, limit.LimitNumber)

	// SetLimit should fail in static mode
	assert.False(t, mgr.SetLimit(2048, 10), "SetLimit should fail in static mode")
}

// TestBufferManagerStaticPushBack tests push back in static mode (CASE_TEST: static_buffer_manager_bf)
func TestBufferManagerStaticPushBack(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetMode(1023, 3)

	// Run cycle 3 times
	for cycle := 0; cycle < 3; cycle++ {
		// Push 3 blocks
		data0, err := mgr.PushBack(99)
		assert.NoError(t, err)
		fillBytes(data0, 0xFF)

		data1, err := mgr.PushBack(28)
		assert.NoError(t, err)
		fillBytes(data1, 0x00)

		data2, err := mgr.PushBack(17)
		assert.NoError(t, err)
		fillBytes(data2, 0xFF)

		// Should hit limit
		_, err = mgr.PushBack(63)
		assert.ErrorIs(t, err, ErrBuffLimit)

		limit := mgr.Limit()
		assert.Equal(t, 144, limit.CostSize)
		assert.Equal(t, 3, limit.CostNumber)

		// Pop and check
		frontData, _, nwrite, err := mgr.FrontData()
		assert.NoError(t, err)
		assert.Equal(t, 99, nwrite)
		assert.Equal(t, byte(0xFF), frontData[0])

		err = mgr.PopFront(128, true)
		assert.NoError(t, err)

		limit = mgr.Limit()
		assert.Equal(t, 45, limit.CostSize)
		assert.Equal(t, 2, limit.CostNumber)

		// Pop remaining
		err = mgr.PopFront(100, true)
		assert.NoError(t, err)

		err = mgr.PopFront(100, true)
		assert.NoError(t, err)

		assert.True(t, mgr.Empty())
		limit = mgr.Limit()
		assert.Equal(t, 0, limit.CostSize)
		assert.Equal(t, 0, limit.CostNumber)
	}
}

// TestBufferManagerStaticPushFront tests push front in static mode
func TestBufferManagerStaticPushFront(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetMode(1023, 3)

	// Run cycle 3 times
	for cycle := 0; cycle < 3; cycle++ {
		// Push 3 blocks from front
		data0, err := mgr.PushFront(99)
		assert.NoError(t, err)
		fillBytes(data0, 0xFF)

		data1, err := mgr.PushFront(28)
		assert.NoError(t, err)
		fillBytes(data1, 0x00)

		data2, err := mgr.PushFront(17)
		assert.NoError(t, err)
		fillBytes(data2, 0xFF)

		// Should hit limit
		_, err = mgr.PushFront(63)
		assert.ErrorIs(t, err, ErrBuffLimit)

		limit := mgr.Limit()
		assert.Equal(t, 144, limit.CostSize)
		assert.Equal(t, 3, limit.CostNumber)

		// Pop from back
		backData, _, nwrite, err := mgr.BackData()
		assert.NoError(t, err)
		assert.Equal(t, 99, nwrite)
		assert.Equal(t, byte(0xFF), backData[0])

		err = mgr.PopBack(128, true)
		assert.NoError(t, err)

		// Pop remaining
		err = mgr.PopBack(100, true)
		assert.NoError(t, err)

		err = mgr.PopBack(100, true)
		assert.NoError(t, err)

		assert.True(t, mgr.Empty())
	}
}

// =============================================================================
// Merge Tests
// =============================================================================

// TestBufferManagerDynamicMergeBack tests merge back in dynamic mode
func TestBufferManagerDynamicMergeBack(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1024, 10)

	// Push initial block
	data, err := mgr.PushBack(100)
	assert.NoError(t, err)
	fillBytes(data, 0xAA)

	// Merge additional space
	mergeData, err := mgr.MergeBack(50)
	assert.NoError(t, err)
	assert.NotNil(t, mergeData)
	assert.Equal(t, 50, len(mergeData))
	fillBytes(mergeData, 0xBB)

	// Check total size
	limit := mgr.Limit()
	assert.Equal(t, 150, limit.CostSize)
	assert.Equal(t, 1, limit.CostNumber)

	// Verify back block has merged data
	back := mgr.Back()
	assert.NotNil(t, back)
	assert.Equal(t, 150, back.RawSize())
}

// TestBufferManagerDynamicMergeFront tests merge front in dynamic mode
func TestBufferManagerDynamicMergeFront(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1024, 10)

	// Push initial block
	data, err := mgr.PushBack(100)
	assert.NoError(t, err)
	fillBytes(data, 0xAA)

	// Merge additional space at front
	mergeData, err := mgr.MergeFront(50)
	assert.NoError(t, err)
	assert.NotNil(t, mergeData)
	assert.Equal(t, 50, len(mergeData))
	fillBytes(mergeData, 0xBB)

	// Check total size
	limit := mgr.Limit()
	assert.Equal(t, 150, limit.CostSize)
	assert.Equal(t, 1, limit.CostNumber)

	// Verify front block has merged data
	front := mgr.Front()
	assert.NotNil(t, front)
	assert.Equal(t, 150, front.RawSize())
}

// TestBufferManagerMergeOnEmpty tests merge on empty manager
func TestBufferManagerMergeOnEmpty(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1024, 10)

	// Merge on empty should create new block
	data, err := mgr.MergeBack(100)
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Equal(t, 100, len(data))

	limit := mgr.Limit()
	assert.Equal(t, 100, limit.CostSize)
	assert.Equal(t, 1, limit.CostNumber)
}

// =============================================================================
// Reset Tests
// =============================================================================

// TestBufferManagerReset tests the reset operation
func TestBufferManagerReset(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1024, 10)

	// Push some data
	mgr.PushBack(100)
	mgr.PushBack(200)

	limit := mgr.Limit()
	assert.Equal(t, 300, limit.CostSize)
	assert.Equal(t, 2, limit.CostNumber)

	// Reset
	mgr.Reset()

	assert.True(t, mgr.Empty())
	limit = mgr.Limit()
	assert.Equal(t, 0, limit.CostSize)
	assert.Equal(t, 0, limit.CostNumber)
	assert.Equal(t, 0, limit.LimitSize)
	assert.Equal(t, 0, limit.LimitNumber)
}

// =============================================================================
// ForEach and Count Tests
// =============================================================================

// TestBufferManagerCount tests counting blocks
func TestBufferManagerCount(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1024, 10)

	assert.Equal(t, 0, mgr.Count())

	mgr.PushBack(10)
	assert.Equal(t, 1, mgr.Count())

	mgr.PushBack(20)
	assert.Equal(t, 2, mgr.Count())

	mgr.PushBack(30)
	assert.Equal(t, 3, mgr.Count())

	mgr.PopFront(100, true)
	assert.Equal(t, 2, mgr.Count())
}

// TestBufferManagerForEach tests iterating over blocks
func TestBufferManagerForEach(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1024, 10)

	sizes := []int{10, 20, 30, 40}
	for _, s := range sizes {
		mgr.PushBack(s)
	}

	var collected []int
	mgr.ForEach(func(block *BufferBlock) bool {
		collected = append(collected, block.Size())
		return true
	})

	assert.Equal(t, sizes, collected)
}

// TestBufferManagerForEachEarlyStop tests early stopping in ForEach
func TestBufferManagerForEachEarlyStop(t *testing.T) {
	mgr := NewBufferManager()
	mgr.SetLimit(1024, 10)

	for i := 0; i < 5; i++ {
		mgr.PushBack(10)
	}

	count := 0
	mgr.ForEach(func(block *BufferBlock) bool {
		count++
		return count < 3 // Stop after 3
	})

	assert.Equal(t, 3, count)
}

// =============================================================================
// Helper Functions
// =============================================================================

func fillBytes(data []byte, value byte) {
	for i := range data {
		data[i] = value
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkBufferBlockNew benchmarks creating new buffer blocks
func BenchmarkBufferBlockNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewBufferBlock(1024)
	}
}

// BenchmarkBufferBlockPop benchmarks pop operation
func BenchmarkBufferBlockPop(b *testing.B) {
	block := NewBufferBlock(1024 * 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block.Pop(1)
		if block.Size() == 0 {
			block.Reset()
		}
	}
}

// BenchmarkBufferManagerPushBack benchmarks push back in dynamic mode
func BenchmarkBufferManagerPushBack(b *testing.B) {
	mgr := NewBufferManager()
	mgr.SetLimit(0, 0) // No limit
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.PushBack(64)
		if mgr.Count() > 1000 {
			mgr.Reset()
		}
	}
}

// BenchmarkBufferManagerPushPopCycle benchmarks push/pop cycle
func BenchmarkBufferManagerPushPopCycle(b *testing.B) {
	mgr := NewBufferManager()
	mgr.SetLimit(0, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.PushBack(64)
		mgr.PopFront(64, true)
	}
}
