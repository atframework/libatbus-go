//go:build windows

package libatbus_channel_iostream

import (
	"os"
)

// tryFlockFile is a no-op on Windows (Unix socket listen-path locking is not supported).
// Returns nil, true to indicate the lock step is considered passed.
func tryFlockFile(path string) (lockFile *os.File, ok bool) {
	return nil, true
}

// unlockFlockFile is a no-op on Windows.
func unlockFlockFile(f *os.File) {
}
