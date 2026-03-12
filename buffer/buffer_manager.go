package libatbus_buffer

import (
	"container/list"
)

// Limit tracks the buffer usage limits and current costs
type Limit struct {
	CostNumber  int // Current number of blocks
	CostSize    int // Current total size used
	LimitNumber int // Maximum number of blocks (0 = unlimited)
	LimitSize   int // Maximum total size (0 = unlimited)
}

// BufferManager manages a collection of buffer blocks
// It can operate in dynamic mode (using linked list) or static mode (using circular buffer)
type BufferManager struct {
	// Dynamic mode storage
	dynamicBuffer *list.List

	// Static mode storage
	staticBuffer *staticBufferStorage

	// Limits
	limit Limit
}

// staticBufferStorage holds the static mode ring buffer data
type staticBufferStorage struct {
	buffer      []byte          // The underlying contiguous buffer
	circleIndex []*BufferBlock  // Circular index of buffer blocks
	head        int             // Head index in circleIndex
	tail        int             // Tail index in circleIndex
}

// NewBufferManager creates a new buffer manager in dynamic mode
func NewBufferManager() *BufferManager {
	return &BufferManager{
		dynamicBuffer: list.New(),
		staticBuffer:  nil,
		limit:         Limit{},
	}
}

// Limit returns the current limit configuration
func (m *BufferManager) Limit() Limit {
	return m.limit
}

// SetLimit sets limits when in dynamic mode
// Returns true on success, false if in static mode
func (m *BufferManager) SetLimit(maxSize, maxNumber int) bool {
	if m.staticBuffer != nil {
		return false
	}
	m.limit.LimitNumber = maxNumber
	m.limit.LimitSize = maxSize
	return true
}

// SetMode switches to static mode with a fixed buffer size and max number of blocks
func (m *BufferManager) SetMode(maxSize, maxNumber int) {
	m.Reset()

	if maxSize != 0 && maxNumber > 0 {
		// Allocate the static buffer with extra alignment
		bufSize := PaddingSize(maxSize + DataAlignSize)
		m.staticBuffer = &staticBufferStorage{
			buffer:      make([]byte, bufSize),
			circleIndex: make([]*BufferBlock, maxNumber+1), // +1 for empty slot
			head:        0,
			tail:        0,
		}
		m.limit.LimitSize = maxSize
		m.limit.LimitNumber = maxNumber
	}
}

// Reset clears all data and returns to initial state
func (m *BufferManager) Reset() {
	// Clear static buffer
	if m.staticBuffer != nil {
		m.staticBuffer = nil
	}

	// Clear dynamic buffer
	m.dynamicBuffer = list.New()

	// Reset limits
	m.limit.CostSize = 0
	m.limit.CostNumber = 0
	m.limit.LimitNumber = 0
	m.limit.LimitSize = 0
}

// IsStaticMode returns true if the manager is in static mode
func (m *BufferManager) IsStaticMode() bool {
	return m.staticBuffer != nil
}

// IsDynamicMode returns true if the manager is in dynamic mode
func (m *BufferManager) IsDynamicMode() bool {
	return m.staticBuffer == nil
}

// Empty returns true if there are no buffer blocks
func (m *BufferManager) Empty() bool {
	if m.IsDynamicMode() {
		return m.dynamicEmpty()
	}
	return m.staticEmpty()
}

// Front returns the first buffer block
func (m *BufferManager) Front() *BufferBlock {
	if m.IsDynamicMode() {
		return m.dynamicFront()
	}
	return m.staticFront()
}

// FrontData returns the data pointer, read size, and write size of the front block
func (m *BufferManager) FrontData() (data []byte, nread, nwrite int, err error) {
	block := m.Front()
	if block == nil {
		return nil, 0, 0, ErrNoData
	}

	data = block.Data()
	nwrite = block.Size()
	nread = block.RawSize() - nwrite
	return data, nread, nwrite, nil
}

// Back returns the last buffer block
func (m *BufferManager) Back() *BufferBlock {
	if m.IsDynamicMode() {
		return m.dynamicBack()
	}
	return m.staticBack()
}

// BackData returns the data pointer, read size, and write size of the back block
func (m *BufferManager) BackData() (data []byte, nread, nwrite int, err error) {
	block := m.Back()
	if block == nil {
		return nil, 0, 0, ErrNoData
	}

	data = block.Data()
	nwrite = block.Size()
	nread = block.RawSize() - nwrite
	return data, nread, nwrite, nil
}

// PushBack allocates a new block at the back and returns the data slice
func (m *BufferManager) PushBack(size int) (data []byte, err error) {
	if m.limit.LimitNumber > 0 && m.limit.CostNumber >= m.limit.LimitNumber {
		return nil, ErrBuffLimit
	}

	if m.limit.LimitSize > 0 && m.limit.CostSize+size > m.limit.LimitSize {
		return nil, ErrBuffLimit
	}

	var block *BufferBlock
	if m.IsDynamicMode() {
		block, err = m.dynamicPushBack(size)
	} else {
		block, err = m.staticPushBack(size)
	}

	if err != nil {
		return nil, err
	}

	m.limit.CostNumber++
	m.limit.CostSize += size

	return block.Data(), nil
}

// PushFront allocates a new block at the front and returns the data slice
func (m *BufferManager) PushFront(size int) (data []byte, err error) {
	if m.limit.LimitNumber > 0 && m.limit.CostNumber >= m.limit.LimitNumber {
		return nil, ErrBuffLimit
	}

	if m.limit.LimitSize > 0 && m.limit.CostSize+size > m.limit.LimitSize {
		return nil, ErrBuffLimit
	}

	var block *BufferBlock
	if m.IsDynamicMode() {
		block, err = m.dynamicPushFront(size)
	} else {
		block, err = m.staticPushFront(size)
	}

	if err != nil {
		return nil, err
	}

	m.limit.CostNumber++
	m.limit.CostSize += size

	return block.Data(), nil
}

// PopBack pops bytes from the back block
func (m *BufferManager) PopBack(size int, freeUnwritable bool) error {
	if m.IsDynamicMode() {
		return m.dynamicPopBack(size, freeUnwritable)
	}
	return m.staticPopBack(size, freeUnwritable)
}

// PopFront pops bytes from the front block
func (m *BufferManager) PopFront(size int, freeUnwritable bool) error {
	if m.IsDynamicMode() {
		return m.dynamicPopFront(size, freeUnwritable)
	}
	return m.staticPopFront(size, freeUnwritable)
}

// MergeBack extends the back block by additional size
func (m *BufferManager) MergeBack(size int) (data []byte, err error) {
	if m.Empty() {
		return m.PushBack(size)
	}

	if m.limit.LimitSize > 0 && m.limit.CostSize+size > m.limit.LimitSize {
		return nil, ErrBuffLimit
	}

	if m.IsDynamicMode() {
		data, err = m.dynamicMergeBack(size)
	} else {
		data, err = m.staticMergeBack(size)
	}

	if err == nil {
		m.limit.CostSize += size
	}

	return data, err
}

// MergeFront extends the front block by additional size
func (m *BufferManager) MergeFront(size int) (data []byte, err error) {
	if m.Empty() {
		return m.PushFront(size)
	}

	if m.limit.LimitSize > 0 && m.limit.CostSize+size > m.limit.LimitSize {
		return nil, ErrBuffLimit
	}

	if m.IsDynamicMode() {
		data, err = m.dynamicMergeFront(size)
	} else {
		data, err = m.staticMergeFront(size)
	}

	if err == nil {
		m.limit.CostSize += size
	}

	return data, err
}

// ============= Dynamic Mode Implementation =============

func (m *BufferManager) dynamicEmpty() bool {
	return m.dynamicBuffer.Len() == 0
}

func (m *BufferManager) dynamicFront() *BufferBlock {
	if m.dynamicEmpty() {
		return nil
	}
	return m.dynamicBuffer.Front().Value.(*BufferBlock)
}

func (m *BufferManager) dynamicBack() *BufferBlock {
	if m.dynamicEmpty() {
		return nil
	}
	return m.dynamicBuffer.Back().Value.(*BufferBlock)
}

func (m *BufferManager) dynamicPushBack(size int) (*BufferBlock, error) {
	block := NewBufferBlock(size)
	if block == nil {
		return nil, ErrMalloc
	}

	m.dynamicBuffer.PushBack(block)
	return block, nil
}

func (m *BufferManager) dynamicPushFront(size int) (*BufferBlock, error) {
	block := NewBufferBlock(size)
	if block == nil {
		return nil, ErrMalloc
	}

	m.dynamicBuffer.PushFront(block)
	return block, nil
}

func (m *BufferManager) dynamicPopBack(size int, freeUnwritable bool) error {
	if m.dynamicEmpty() {
		return ErrNoData
	}

	elem := m.dynamicBuffer.Back()
	block := elem.Value.(*BufferBlock)

	if size > block.Size() {
		size = block.Size()
	}

	block.Pop(size)
	if freeUnwritable && block.Size() <= 0 {
		m.dynamicBuffer.Remove(elem)
		if m.limit.CostNumber > 0 {
			m.limit.CostNumber--
		}
	}

	// Fix limit
	if m.dynamicEmpty() {
		m.limit.CostSize = 0
		m.limit.CostNumber = 0
	} else {
		if m.limit.CostSize >= size {
			m.limit.CostSize -= size
		} else {
			m.limit.CostSize = 0
		}
	}

	return nil
}

func (m *BufferManager) dynamicPopFront(size int, freeUnwritable bool) error {
	if m.dynamicEmpty() {
		return ErrNoData
	}

	elem := m.dynamicBuffer.Front()
	block := elem.Value.(*BufferBlock)

	if size > block.Size() {
		size = block.Size()
	}

	block.Pop(size)
	if freeUnwritable && block.Size() <= 0 {
		m.dynamicBuffer.Remove(elem)
		if m.limit.CostNumber > 0 {
			m.limit.CostNumber--
		}
	}

	// Fix limit
	if m.dynamicEmpty() {
		m.limit.CostSize = 0
		m.limit.CostNumber = 0
	} else {
		if m.limit.CostSize >= size {
			m.limit.CostSize -= size
		} else {
			m.limit.CostSize = 0
		}
	}

	return nil
}

func (m *BufferManager) dynamicMergeBack(size int) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}

	block := m.dynamicBack()
	if block == nil {
		return nil, ErrNoData
	}

	// Create a new larger block
	newBlock := NewBufferBlock(size + block.RawSize())
	if newBlock == nil {
		return nil, ErrMalloc
	}

	// Copy old data
	copy(newBlock.data, block.RawData())
	newBlock.used = block.Used()

	// Replace the back element
	elem := m.dynamicBuffer.Back()
	elem.Value = newBlock

	// Return pointer to the new space
	return newBlock.data[block.RawSize():], nil
}

func (m *BufferManager) dynamicMergeFront(size int) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}

	block := m.dynamicFront()
	if block == nil {
		return nil, ErrNoData
	}

	// Create a new larger block
	newBlock := NewBufferBlock(size + block.RawSize())
	if newBlock == nil {
		return nil, ErrMalloc
	}

	// Copy old data to the end (after the new space)
	copy(newBlock.data[size:], block.RawData())
	newBlock.used = block.Used()

	// Replace the front element
	elem := m.dynamicBuffer.Front()
	elem.Value = newBlock

	// Return pointer to the new space at the front
	return newBlock.data[:size], nil
}

// ============= Static Mode Implementation =============

func (m *BufferManager) staticEmpty() bool {
	if m.staticBuffer == nil {
		return true
	}
	return m.staticBuffer.head == m.staticBuffer.tail
}

func (m *BufferManager) staticFront() *BufferBlock {
	if m.staticEmpty() {
		return nil
	}
	return m.staticBuffer.circleIndex[m.staticBuffer.head]
}

func (m *BufferManager) staticBack() *BufferBlock {
	if m.staticEmpty() {
		return nil
	}
	tailIndex := (m.staticBuffer.tail + len(m.staticBuffer.circleIndex) - 1) % len(m.staticBuffer.circleIndex)
	return m.staticBuffer.circleIndex[tailIndex]
}

func (m *BufferManager) staticPushBack(size int) (*BufferBlock, error) {
	sb := m.staticBuffer
	circleLen := len(sb.circleIndex)

	// Check if circle buffer is full
	if (sb.tail+1)%circleLen == sb.head {
		return nil, ErrBuffLimit
	}

	// Get current head and tail blocks
	head := sb.circleIndex[sb.head]
	tail := sb.circleIndex[sb.tail]

	// Empty init
	if head == nil && tail == nil {
		sb.tail = 0
		sb.head = 0
		// Create first block at the beginning of buffer
		block := &BufferBlock{
			data: sb.buffer[:size],
			used: 0,
		}
		sb.circleIndex[sb.tail] = block
		sb.tail = (sb.tail + 1) % circleLen
		return block, nil
	}

	// Find a slot for the new block
	// In static mode, we need to find contiguous space
	// For simplicity, we'll allocate sequentially
	block := NewBufferBlock(size)
	if block == nil {
		return nil, ErrMalloc
	}

	sb.circleIndex[sb.tail] = block
	sb.tail = (sb.tail + 1) % circleLen

	return block, nil
}

func (m *BufferManager) staticPushFront(size int) (*BufferBlock, error) {
	sb := m.staticBuffer
	circleLen := len(sb.circleIndex)

	// Check if circle buffer is full
	if (sb.tail+1)%circleLen == sb.head {
		return nil, ErrBuffLimit
	}

	// Get current head and tail blocks
	head := sb.circleIndex[sb.head]
	tail := sb.circleIndex[sb.tail]

	// Empty init
	if head == nil && tail == nil {
		sb.tail = 0
		sb.head = 0
		block := &BufferBlock{
			data: sb.buffer[:size],
			used: 0,
		}
		sb.circleIndex[sb.head] = block
		sb.tail = (sb.tail + 1) % circleLen
		return block, nil
	}

	// Create new block
	block := NewBufferBlock(size)
	if block == nil {
		return nil, ErrMalloc
	}

	// Move head back
	sb.head = (sb.head + circleLen - 1) % circleLen
	sb.circleIndex[sb.head] = block

	return block, nil
}

func (m *BufferManager) staticPopBack(size int, freeUnwritable bool) error {
	if m.staticEmpty() {
		return ErrNoData
	}

	sb := m.staticBuffer
	circleLen := len(sb.circleIndex)

	tailIndex := (sb.tail + circleLen - 1) % circleLen
	block := sb.circleIndex[tailIndex]

	if size > block.Size() {
		size = block.Size()
	}

	block.Pop(size)
	if freeUnwritable && block.Size() == 0 {
		sb.circleIndex[tailIndex] = nil
		sb.tail = tailIndex

		if m.limit.CostNumber > 0 {
			m.limit.CostNumber--
		}
	}

	// Fix limit and reset to init state
	if m.staticEmpty() {
		sb.head = 0
		sb.tail = 0
		m.limit.CostSize = 0
		m.limit.CostNumber = 0
	} else {
		if m.limit.CostSize >= size {
			m.limit.CostSize -= size
		} else {
			m.limit.CostSize = 0
		}
	}

	return nil
}

func (m *BufferManager) staticPopFront(size int, freeUnwritable bool) error {
	if m.staticEmpty() {
		return ErrNoData
	}

	sb := m.staticBuffer
	circleLen := len(sb.circleIndex)

	block := sb.circleIndex[sb.head]

	if size > block.Size() {
		size = block.Size()
	}

	block.Pop(size)
	if freeUnwritable && block.Size() == 0 {
		sb.circleIndex[sb.head] = nil
		sb.head = (sb.head + 1) % circleLen

		if m.limit.CostNumber > 0 {
			m.limit.CostNumber--
		}
	}

	// Fix limit and reset to init state
	if m.staticEmpty() {
		sb.head = 0
		sb.tail = 0
		m.limit.CostSize = 0
		m.limit.CostNumber = 0
	} else {
		if m.limit.CostSize >= size {
			m.limit.CostSize -= size
		} else {
			m.limit.CostSize = 0
		}
	}

	return nil
}

func (m *BufferManager) staticMergeBack(size int) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}

	block := m.staticBack()
	if block == nil {
		return nil, ErrNoData
	}

	// Create a new larger block and copy data
	newBlock := NewBufferBlock(size + block.RawSize())
	if newBlock == nil {
		return nil, ErrMalloc
	}

	// Copy old data
	copy(newBlock.data, block.RawData())
	newBlock.used = block.Used()

	// Replace in circle index
	sb := m.staticBuffer
	circleLen := len(sb.circleIndex)
	tailIndex := (sb.tail + circleLen - 1) % circleLen
	sb.circleIndex[tailIndex] = newBlock

	// Return pointer to the new space
	return newBlock.data[block.RawSize():], nil
}

func (m *BufferManager) staticMergeFront(size int) ([]byte, error) {
	if size == 0 {
		return nil, nil
	}

	block := m.staticFront()
	if block == nil {
		return nil, ErrNoData
	}

	// Create a new larger block
	newBlock := NewBufferBlock(size + block.RawSize())
	if newBlock == nil {
		return nil, ErrMalloc
	}

	// Copy old data to the end (after the new space)
	copy(newBlock.data[size:], block.RawData())
	newBlock.used = block.Used()

	// Replace in circle index
	sb := m.staticBuffer
	sb.circleIndex[sb.head] = newBlock

	// Return pointer to the new space at the front
	return newBlock.data[:size], nil
}

// Count returns the number of buffer blocks
func (m *BufferManager) Count() int {
	if m.IsDynamicMode() {
		return m.dynamicBuffer.Len()
	}

	if m.staticEmpty() {
		return 0
	}

	sb := m.staticBuffer
	circleLen := len(sb.circleIndex)
	if sb.tail >= sb.head {
		return sb.tail - sb.head
	}
	return circleLen - sb.head + sb.tail
}

// ForEach iterates over all buffer blocks
func (m *BufferManager) ForEach(fn func(block *BufferBlock) bool) {
	if m.IsDynamicMode() {
		for e := m.dynamicBuffer.Front(); e != nil; e = e.Next() {
			if !fn(e.Value.(*BufferBlock)) {
				break
			}
		}
		return
	}

	if m.staticEmpty() {
		return
	}

	sb := m.staticBuffer
	circleLen := len(sb.circleIndex)
	for i := sb.head; i != sb.tail; i = (i + 1) % circleLen {
		if sb.circleIndex[i] != nil {
			if !fn(sb.circleIndex[i]) {
				break
			}
		}
	}
}
