package libatbus_buffer

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func ringOffset(t *testing.T, m *BufferManager, data []byte) int {
	t.Helper()
	for i := range m.staticBuffer.buffer {
		if &m.staticBuffer.buffer[i] == &data[0] {
			return i
		}
	}
	t.Fatal("block is outside the static buffer")
	return 0
}

// Exercise both ends of the ring, including a consumed block retained by the caller.
func TestStaticBufferManagerCircle(t *testing.T) {
	for _, direction := range []string{"back", "front"} {
		t.Run(direction, func(t *testing.T) {
			m := NewBufferManager()
			m.SetMode(1023, 10)
			push, pop, first := m.PushBack, m.PopFront, m.Front
			lastSize := PaddingSize(1025) - 3*256 - 1 - HeadSize(256)
			tooLarge := lastSize + 2
			if direction == "front" {
				push, pop, first = m.PushFront, m.PopBack, m.Back
				lastSize = 255 - HeadSize(255) - PaddingSize(1)
				tooLarge = 257 - HeadSize(257)
			}
			blockSize := 256 - HeadSize(256)
			for i := 0; i < 3; i++ {
				data, err := push(blockSize)
				require.NoError(t, err)
				fillBytes(data, byte(i+1))
			}
			for _, size := range []int{256 + 2*HeadSize(256), tooLarge} {
				data, err := push(size)
				require.ErrorIs(t, err, ErrBuffLimit)
				require.Nil(t, data)
			}
			_, err := push(lastSize)
			require.NoError(t, err)
			require.Equal(t, 3*blockSize+lastSize, m.Limit().CostSize)
			require.NoError(t, pop(256, false))
			require.Equal(t, 4, m.Count())
			require.Zero(t, first().Size())
			require.Equal(t, bytes.Repeat([]byte{1}, blockSize), first().RawData())
			require.NoError(t, pop(0, true))
			require.Equal(t, 3, m.Count())
			require.Equal(t, byte(2), first().Data()[0])
			secondOffset := ringOffset(t, m, first().Data())
			_, err = push(256)
			require.ErrorIs(t, err, ErrBuffLimit)
			require.NoError(t, pop(blockSize-1, true))
			require.Equal(t, 1, first().Size())
			wrapped, err := push(128)
			require.NoError(t, err)
			fillBytes(wrapped, 0xef)
			_, err = push(128)
			require.ErrorIs(t, err, ErrBuffLimit)
			require.NoError(t, pop(1, true))
			require.Equal(t, byte(3), first().Data()[0])
			thirdOffset := ringOffset(t, m, first().Data())
			next, err := push(384 - 2*HeadSize(100) - PaddingSize(1))
			require.NoError(t, err)
			require.Equal(t, 4, m.Count())
			require.Equal(t, bytes.Repeat([]byte{0xef}, 128), wrapped)
			wrappedOffset, nextOffset := ringOffset(t, m, wrapped), ringOffset(t, m, next)
			if direction == "back" {
				require.Less(t, wrappedOffset, nextOffset)
				require.LessOrEqual(t, nextOffset, secondOffset)
				require.Less(t, secondOffset, thirdOffset)
			} else {
				require.Greater(t, wrappedOffset, nextOffset)
				require.GreaterOrEqual(t, nextOffset, secondOffset)
				require.Greater(t, secondOffset, thirdOffset)
			}
		})
	}
}

// Other live blocks must keep their contents when alternating both ends of a wrapped ring.
func TestStaticBufferManagerMixedOperations(t *testing.T) {
	type expectedBlock struct {
		raw  []byte
		used int
	}
	rng := rand.New(rand.NewSource(7349))
	m := NewBufferManager()
	m.SetMode(255, 6)
	var expected []expectedBlock
	for step := 0; step < 4000; step++ {
		op, size := rng.Intn(6), 1+rng.Intn(57)
		if len(expected) == 0 {
			op %= 2
		}
		index := 0
		if op%2 == 1 && len(expected) > 0 {
			index = len(expected) - 1
		}
		var data []byte
		var err error
		switch op {
		case 0, 1:
			if op == 0 {
				data, err = m.PushFront(size)
			} else {
				data, err = m.PushBack(size)
			}
			if err == nil {
				b := expectedBlock{raw: bytes.Repeat([]byte{byte(step)}, size)}
				if op == 0 {
					expected = append([]expectedBlock{b}, expected...)
				} else {
					expected = append(expected, b)
				}
			}
		case 2, 3:
			if op == 2 {
				data, err = m.MergeFront(size)
			} else {
				data, err = m.MergeBack(size)
			}
			if err == nil {
				expected[index].raw = append(expected[index].raw, bytes.Repeat([]byte{byte(step)}, size)...)
			}
		case 4, 5:
			if op == 4 {
				err = m.PopFront(size, true)
			} else {
				err = m.PopBack(size, true)
			}
			require.NoError(t, err)
			expected[index].used += size
			if expected[index].used >= len(expected[index].raw) {
				expected = append(expected[:index], expected[index+1:]...)
			}
		}
		if err != nil {
			require.ErrorIs(t, err, ErrBuffLimit)
		}
		fillBytes(data, byte(step))
		i, total := 0, 0
		m.ForEach(func(block *BufferBlock) bool {
			require.Less(t, i, len(expected))
			require.Equal(t, expected[i].raw, block.RawData(), "step %d, operation %d, block %d", step, op, i)
			require.Equal(t, expected[i].used, block.Used())
			total += block.Size()
			i++
			return true
		})
		require.Equal(t, len(expected), i)
		require.Equal(t, total, m.Limit().CostSize)
	}
}

// Layouts mirror the in-place, wrapped and relocated merge cases in buffer_test.cpp.
func TestStaticBufferManagerMergeLayouts(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		blocks                                   []int // sizes including aligned block headers
		popFirst, pushAfterPop, front, pushFront bool
		rejected, accepted                       int
		sameAddress                              bool
	}{
		{"back_in_place", []int{128}, false, false, false, false, PaddingSize(257) - 127, 128, true},
		{"back_relocate", []int{152, 64}, true, false, false, false, 81, 80, false},
		{"back_wrapped", []int{136, 72, 64}, true, true, false, false, 69, 64, true},
		{"front_at_end", []int{128}, false, false, true, true, 129, 120, false},
		{"front_before_head", []int{136, 64}, true, false, true, false, 137, 136, false},
		{"front_relocate", []int{64, 32, 32}, true, false, true, false, PaddingSize(257) - 63, 65, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewBufferManager()
			m.SetMode(255, 3)
			push := m.PushBack
			if tc.pushFront {
				push = m.PushFront
			}
			for i, n := range tc.blocks {
				if tc.pushAfterPop && i == len(tc.blocks)-1 {
					require.NoError(t, m.PopFront(255, true))
				}
				_, err := push(n - HeadSize(n))
				require.NoError(t, err)
			}
			if tc.popFirst && !tc.pushAfterPop {
				require.NoError(t, m.PopFront(255, true))
			}
			merge, block := m.MergeBack, m.Back
			if tc.front {
				merge, block = m.MergeFront, m.Front
			}
			originalBlock := block()
			old := originalBlock.RawData()
			fillBytes(old, 0xec)
			cost := m.Limit()
			extra, err := merge(tc.rejected)
			require.ErrorIs(t, err, ErrBuffLimit)
			require.Nil(t, extra)
			extra, err = merge(0)
			require.NoError(t, err)
			require.Nil(t, extra)
			require.Equal(t, cost, m.Limit())
			require.Same(t, &old[0], &block().RawData()[0])
			extra, err = merge(tc.accepted)
			require.NoError(t, err)
			raw := block().RawData()
			require.Equal(t, tc.sameAddress, &old[0] == &raw[0])
			oldOffset, newOffset := ringOffset(t, m, old), ringOffset(t, m, raw)
			switch tc.name {
			case "back_relocate":
				require.Less(t, newOffset, oldOffset)
			case "back_wrapped":
				require.Less(t, ringOffset(t, m, extra), ringOffset(t, m, m.Front().RawData()))
			case "front_before_head", "front_at_end":
				require.Equal(t, oldOffset-PaddingSize(tc.accepted), newOffset)
			case "front_relocate":
				require.Greater(t, newOffset, oldOffset)
			}
			if tc.sameAddress {
				require.Same(t, originalBlock, block())
				require.Equal(t, len(old)+tc.accepted, originalBlock.RawSize())
			}
			require.Equal(t, bytes.Repeat([]byte{0xec}, len(old)), raw[:len(old)])
			require.Len(t, extra, tc.accepted)
			require.Same(t, &extra[0], &raw[len(old)])
			require.Equal(t, cost.CostNumber, m.Count())
			require.Equal(t, cost.CostSize+tc.accepted, m.Limit().CostSize)
		})
	}
}
