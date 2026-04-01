package libatbus_impl

import (
	"testing"

	buffer "github.com/atframework/libatbus-go/buffer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

func TestCreateConnectionInitializesAddressAndContext(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3001, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")

	// Assert
	require.NotNil(t, conn)
	require.NotNil(t, conn.GetAddress())
	assert.Equal(t, "ipv4://127.0.0.1:0", conn.GetAddress().GetAddress())
	assert.Equal(t, types.ConnectionState_Disconnected, conn.GetStatus())
	assert.NotNil(t, conn.GetConnectionContext())
}

func TestConnectionSetTemporarySetsTemporaryFlag(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3002, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	// Act
	conn.SetTemporary()

	// Assert
	assert.True(t, conn.CheckFlag(types.ConnectionFlag_Temporary))
}

func TestConnectionIsRunningReflectsStatus(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3003, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	// Act + Assert
	conn.setStatus(types.ConnectionState_Disconnected)
	assert.False(t, conn.IsRunning())

	conn.setStatus(types.ConnectionState_Connecting)
	assert.True(t, conn.IsRunning())

	conn.setStatus(types.ConnectionState_Handshaking)
	assert.True(t, conn.IsRunning())

	conn.setStatus(types.ConnectionState_Connected)
	assert.True(t, conn.IsRunning())

	conn.setStatus(types.ConnectionState_Disconnecting)
	assert.False(t, conn.IsRunning())
}

type connectionTestIoStreamConn struct {
	address     types.ChannelAddress
	privateData interface{}
	handles     types.IoStreamCallbackEventHandleSet
}

func (c *connectionTestIoStreamConn) GetAddress() types.ChannelAddress {
	return c.address
}

func (c *connectionTestIoStreamConn) GetStatus() types.IoStreamConnectionStatus {
	return types.IoStreamConnectionStatus_Connected
}

func (c *connectionTestIoStreamConn) SetFlag(types.IoStreamConnectionFlag, bool) {}

func (c *connectionTestIoStreamConn) GetFlag(types.IoStreamConnectionFlag) bool {
	return false
}

func (c *connectionTestIoStreamConn) GetChannel() types.IoStreamChannel {
	return nil
}

func (c *connectionTestIoStreamConn) GetEventHandleSet() *types.IoStreamCallbackEventHandleSet {
	return &c.handles
}

func (c *connectionTestIoStreamConn) GetProactivelyDisconnectCallback() types.IoStreamCallbackFunc {
	return nil
}

func (c *connectionTestIoStreamConn) GetReadBufferManager() *buffer.BufferManager {
	return nil
}

func (c *connectionTestIoStreamConn) SetPrivateData(data interface{}) {
	c.privateData = data
}

func (c *connectionTestIoStreamConn) GetPrivateData() interface{} {
	return c.privateData
}

func TestConnectionPushTracksStartAndImmediateFailureStatistics(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3004, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	// Act
	pushErr := conn.Push([]byte("abc"))

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_CLOSING, pushErr)
	assert.Equal(t, types.ConnectionStatistic{
		PushStartTimes:  1,
		PushStartSize:   3,
		PushFailedTimes: 1,
		PushFailedSize:  3,
	}, conn.GetStatistic())
}

func TestOnIoStreamWrittenTracksSuccessfulWriteStatistics(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3005, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	ioConn := &connectionTestIoStreamConn{address: conn.GetAddress()}
	ioConn.SetPrivateData(conn)

	// Act
	n.onIoStreamWritten(ioConn, int32(error_code.EN_ATBUS_ERR_SUCCESS), []byte("abcd"))

	// Assert
	assert.Equal(t, uint64(1), conn.GetStatistic().PushSuccessTimes)
	assert.Equal(t, uint64(4), conn.GetStatistic().PushSuccessSize)
	assert.Equal(t, uint64(0), conn.GetStatistic().PushFailedTimes)
	assert.Equal(t, uint64(0), conn.GetStatistic().PushFailedSize)
}

func TestOnIoStreamWrittenTracksFailedWriteStatistics(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3006, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	ioConn := &connectionTestIoStreamConn{address: conn.GetAddress()}
	ioConn.SetPrivateData(conn)

	// Act
	n.onIoStreamWritten(ioConn, int32(error_code.EN_ATBUS_ERR_WRITE_FAILED), []byte("de"))

	// Assert
	assert.Equal(t, uint64(0), conn.GetStatistic().PushSuccessTimes)
	assert.Equal(t, uint64(0), conn.GetStatistic().PushSuccessSize)
	assert.Equal(t, uint64(1), conn.GetStatistic().PushFailedTimes)
	assert.Equal(t, uint64(2), conn.GetStatistic().PushFailedSize)
}

func TestOnIoStreamReceivedTracksPullStatistics(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x3007, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	ioConn := &connectionTestIoStreamConn{address: conn.GetAddress()}
	ioConn.SetPrivateData(conn)

	// Act — payload will fail to unpack as a valid message, but pull stats
	// should still be incremented (matching C++ iostream_on_recv_cb).
	n.onIoStreamReceived(ioConn, 0, []byte("hello"))

	// Assert
	assert.Equal(t, uint64(1), conn.GetStatistic().PullStartTimes)
	assert.Equal(t, uint64(5), conn.GetStatistic().PullStartSize)
}
