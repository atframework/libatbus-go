//go:build !windows

package libatbus_channel_iostream

import (
	"os"
	"syscall"
)

// tryFlockFile tries to acquire an exclusive, non-blocking file lock.
// Returns the *os.File on success and ok=true.
// Returns nil and ok=false if the lock could not be acquired.
func tryFlockFile(path string) (lockFile *os.File, ok bool) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		return nil, false
	}

	return f, true
}

// unlockFlockFile releases the file lock and closes the file.
func unlockFlockFile(f *os.File) {
	if f == nil {
		return
	}
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
}
