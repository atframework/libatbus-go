//go:build !windows

package libatbus_channel_iostream

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	error_code "github.com/atframework/libatbus-go/error_code"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// An exec child must neither inherit the listening lock nor release its parent's lock.
func TestIoStreamUnixAddressLockCloseOnExec(t *testing.T) {
	const childPath = "LIBATBUS_TEST_EXEC_LOCK"
	if path := os.Getenv(childPath); path != "" {
		fd, err := strconv.Atoi(os.Getenv("LIBATBUS_TEST_EXEC_FD"))
		require.NoError(t, err)
		var original, inherited unix.Stat_t
		require.NoError(t, unix.Stat(path, &original))
		if unix.Fstat(fd, &inherited) == nil && inherited.Dev == original.Dev && inherited.Ino == original.Ino {
			t.Fatal("listener lock descriptor survived exec")
		}
		file, locked := tryFlockFile(path)
		if locked {
			unlockFlockFile(file)
			t.Fatal("child acquired parent's lock")
		}
		return
	}
	sock := getUnixSocketPath(t)
	path := sock + ".lock"
	addr := "unix://" + sock
	t.Cleanup(func() { os.Remove(path) })
	c := NewIoStreamChannel(context.Background(), nil)
	c.conf.OverwriteListenPath = false
	t.Cleanup(func() { c.Close() })
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, c.Listen(addr))
	f := c.addressLocks[addr]
	require.NotNil(t, f)
	flags, err := unix.FcntlInt(f.Fd(), unix.F_GETFD, 0)
	require.NoError(t, err)
	require.NotZero(t, flags&unix.FD_CLOEXEC)
	exe, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, exe, "-test.run=^TestIoStreamUnixAddressLockCloseOnExec$")
	child.Env = append(os.Environ(), childPath+"="+path, fmt.Sprintf("LIBATBUS_TEST_EXEC_FD=%d", f.Fd()))
	out, err := child.CombinedOutput()
	require.NoError(t, err, "%s", out)
	file, locked := tryFlockFile(path)
	if locked {
		unlockFlockFile(file)
		t.Fatal("parent lost lock after child exit")
	}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, c.Close())
	file, locked = tryFlockFile(path)
	require.True(t, locked)
	unlockFlockFile(file)
}
