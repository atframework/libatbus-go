package libatbus_buffer

// ReadVint reads a variable-length encoded integer from the buffer.
// Each byte uses 7 bits for data and the highest bit (0x80) indicates continuation.
// Returns the decoded value and the number of bytes consumed.
// If the buffer is empty or truncated (continuation bit set but no more data), returns (0, 0).
func ReadVint(data []byte) (uint64, int) {
	if len(data) == 0 {
		return 0, 0
	}

	var out uint64
	left := len(data)

	for i := 0; i < len(data); i++ {
		left--
		out <<= 7
		out |= uint64(data[i] & 0x7F)

		if (data[i] & 0x80) == 0 {
			// No continuation bit, this is the last byte
			break
		} else if left == 0 {
			// Continuation bit set but no more data - truncated
			return 0, 0
		}
	}

	return out, len(data) - left
}

// WriteVint writes a variable-length encoded integer to the buffer.
// Each byte uses 7 bits for data and the highest bit (0x80) indicates continuation.
// Returns the number of bytes written.
// If the buffer is too small to hold the encoded value, returns 0.
func WriteVint(value uint64, data []byte) int {
	if len(data) == 0 {
		return 0
	}

	used := 1
	d := 0

	// Write the first 7-bit chunk (lowest bits, will be last after reversal)
	data[d] = byte(value & 0x7F)
	value >>= 7

	// Write remaining 7-bit chunks with continuation bit
	for value != 0 && used+1 <= len(data) {
		used++
		d++
		data[d] = byte(0x80 | (value & 0x7F))
		value >>= 7
	}

	// Check if we ran out of buffer space
	if value != 0 {
		return 0
	}

	// Reverse the bytes so the most significant comes first
	if d > 0 {
		for i, j := 0, d; i < j; i, j = i+1, j-1 {
			data[i], data[j] = data[j], data[i]
		}
	}

	return used
}

// VintEncodedSize returns the number of bytes needed to encode the given value.
func VintEncodedSize(value uint64) int {
	if value == 0 {
		return 1
	}

	size := 0
	for value != 0 {
		size++
		value >>= 7
	}
	return size
}
