// Package libatbus_impl provides internal implementation details for libatbus.
//
// This file contains targeted tests for fault_tolerant counting, threshold
// enforcement, and OnInvalidConnection callback behavior.

package libatbus_impl

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

// ============================================================================
// addEndpointFault — threshold enforcement
// ============================================================================

func TestAddEndpointFault_BelowThreshold_DoesNotRemove(t *testing.T) {
	// Arrange: create node with FaultTolerant=3 so that faults 1 and 2 should NOT trigger removal.
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.FaultTolerant = 3

	var n Node
	ret := n.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, 0x12345679, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep)
	ret = n.AddEndpoint(ep)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act: accumulate faults below threshold (fault 1, 2)
	removed1 := n.addEndpointFault(ep, nil)
	removed2 := n.addEndpointFault(ep, nil)

	// Assert: endpoint should still be present
	assert.False(t, removed1, "fault 1 should not trigger removal (threshold=3)")
	assert.False(t, removed2, "fault 2 should not trigger removal (threshold=3)")
	assert.NotNil(t, n.GetEndpoint(0x12345679), "endpoint should still exist in route")
}

func TestAddEndpointFault_AtThreshold_RemovesEndpoint(t *testing.T) {
	// Arrange: FaultTolerant=2 (default), so fault count reaching 2 should trigger removal.
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, 0x12345679, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep)
	ret = n.AddEndpoint(ep)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act: first fault should NOT remove
	removed1 := n.addEndpointFault(ep, nil)
	assert.False(t, removed1, "first fault should not trigger removal (threshold=2)")

	// Act: second fault should trigger removal (count=2 >= threshold=2)
	removed2 := n.addEndpointFault(ep, nil)

	// Assert
	assert.True(t, removed2, "second fault should trigger removal (count=2 >= threshold=2)")
	assert.Nil(t, n.GetEndpoint(0x12345679), "endpoint should be removed from route")
}

func TestAddEndpointFault_HighThreshold_AccumulatesCorrectly(t *testing.T) {
	// Arrange: set a higher threshold (5) and verify counting.
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.FaultTolerant = 5

	var n Node
	ret := n.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, 0x12345680, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep)
	ret = n.AddEndpoint(ep)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act: accumulate 4 faults (all below threshold)
	for i := 0; i < 4; i++ {
		removed := n.addEndpointFault(ep, nil)
		assert.False(t, removed, "fault %d should not trigger removal", i+1)
	}
	assert.NotNil(t, n.GetEndpoint(0x12345680))

	// Act: 5th fault reaches threshold
	removed := n.addEndpointFault(ep, nil)
	assert.True(t, removed, "fault 5 should trigger removal (count=5 >= threshold=5)")
	assert.Nil(t, n.GetEndpoint(0x12345680))
}

func TestAddEndpointFault_NilEndpoint_ReturnsFalse(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	assert.False(t, n.addEndpointFault(nil, nil))
}

func TestAddEndpointFault_NilNode_ReturnsFalse(t *testing.T) {
	var n *Node
	assert.False(t, n.addEndpointFault(nil, nil))
}

// ============================================================================
// addConnectionFault — threshold enforcement
// ============================================================================

func TestAddConnectionFault_BelowThreshold_DoesNotReset(t *testing.T) {
	// Arrange
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.FaultTolerant = 3

	var n Node
	ret := n.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connected)

	// Act: faults 1, 2 — below threshold of 3
	reset1 := n.addConnectionFault(conn)
	reset2 := n.addConnectionFault(conn)

	// Assert: connection should NOT be reset
	assert.False(t, reset1)
	assert.False(t, reset2)
	assert.NotEqual(t, types.ConnectionState_Disconnected, conn.GetStatus())
}

func TestAddConnectionFault_AtThreshold_ResetsConnection(t *testing.T) {
	// Arrange: default FaultTolerant=2
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, 0x12345679, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connected)
	ep.AddConnection(conn, false)

	// Act: first fault
	reset1 := n.addConnectionFault(conn)
	assert.False(t, reset1, "first fault should not reset (threshold=2)")

	// Act: second fault reaches threshold
	reset2 := n.addConnectionFault(conn)

	// Assert
	assert.True(t, reset2, "second fault should reset connection (count=2 >= threshold=2)")
}

func TestAddConnectionFault_NilConnection_ReturnsFalse(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	assert.False(t, n.addConnectionFault(nil))
}

// ============================================================================
// ClearStatisticFault — reset after success
// ============================================================================

func TestEndpointClearStatisticFault_ResetsCounter(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, 0x12345679, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep)

	// Accumulate faults
	ep.AddStatisticFault()
	ep.AddStatisticFault()

	// Clear
	ep.ClearStatisticFault()

	// After clear, the next AddStatisticFault should return 1 (not 3)
	count := ep.AddStatisticFault()
	assert.Equal(t, uint64(1), count, "fault counter should restart from 0 after clear")
}

func TestConnectionClearStatisticFault_ResetsCounter(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	conn.AddStatisticFault()
	conn.AddStatisticFault()
	conn.ClearStatisticFault()

	count := conn.AddStatisticFault()
	assert.Equal(t, uint64(1), count)
}

// ============================================================================
// AddStatisticFault — atomic counter correctness
// ============================================================================

func TestEndpointAddStatisticFault_Monotonic(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, 0x12345679, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep)

	for i := uint64(1); i <= 10; i++ {
		count := ep.AddStatisticFault()
		assert.Equal(t, i, count, "AddStatisticFault should return monotonically increasing count")
	}
}

func TestConnectionAddStatisticFault_Monotonic(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)

	for i := uint64(1); i <= 10; i++ {
		count := conn.AddStatisticFault()
		assert.Equal(t, i, count)
	}
}

func TestEndpointAddStatisticFault_NilReceiver(t *testing.T) {
	var ep *Endpoint
	assert.Equal(t, uint64(0), ep.AddStatisticFault())
}

func TestConnectionAddStatisticFault_NilReceiver(t *testing.T) {
	var conn *Connection
	assert.Equal(t, uint64(0), conn.AddStatisticFault())
}

// ============================================================================
// OnInvalidConnection callback — fires on connecting timeout
// ============================================================================

func TestOnInvalidConnection_FiresOnConnectingTimeout(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Track callback invocations
	// Match C++ node::proc timeout cleanup: the timed-out connection should only
	// notify once, even though Reset() internally removes the timer entry.
	var callbackCount int32
	var lastCallbackErrCode atomic.Value
	n.SetEventHandleOnInvalidConnection(func(node types.Node, conn types.Connection, errCode error_code.ErrorType) error_code.ErrorType {
		callbackCount++
		lastCallbackErrCode.Store(errCode)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Create a connection that is NOT connected (simulates connecting state)
	conn := CreateConnection(&n, "ipv4://127.0.0.1:9999")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connecting)

	// Add to connecting list with a timeout in the past
	pastTimeout := time.Now().Add(-1 * time.Second)
	n.eventTimer.connectingList.Put(conn.GetAddress().GetAddress(),
		types.TimerDescPair[*Connection]{
			Timeout: pastTimeout,
			Value:   conn,
		})

	// Act: process connecting timeout with current time
	n.processConnectingTimeout(time.Now())

	// Assert: callback fires exactly once (C++ parity)
	assert.Equal(t, int32(1), callbackCount,
		"OnInvalidConnection callback should fire exactly once for a timed-out connection")
	stored := lastCallbackErrCode.Load()
	require.NotNil(t, stored)
	assert.Equal(t, error_code.EN_ATBUS_ERR_NODE_TIMEOUT, stored.(error_code.ErrorType))
}

func TestOnInvalidConnection_NotFiredWhenConnectionAlreadyConnected(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	var callbackCount int32
	n.SetEventHandleOnInvalidConnection(func(node types.Node, conn types.Connection, errCode error_code.ErrorType) error_code.ErrorType {
		callbackCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Connection that IS already connected — should be skipped, not timed out
	conn := CreateConnection(&n, "ipv4://127.0.0.1:9998")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connected)

	pastTimeout := time.Now().Add(-1 * time.Second)
	n.eventTimer.connectingList.Put(conn.GetAddress().GetAddress(),
		types.TimerDescPair[*Connection]{
			Timeout: pastTimeout,
			Value:   conn,
		})

	// Act
	n.processConnectingTimeout(time.Now())

	// Assert: callback should NOT fire for already-connected connections
	assert.Equal(t, int32(0), callbackCount, "should not fire for already-connected")
}

func TestOnInvalidConnection_NotFiredWhenNoCallback(t *testing.T) {
	// Arrange: no callback set
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:9997")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connecting)

	pastTimeout := time.Now().Add(-1 * time.Second)
	n.eventTimer.connectingList.Put(conn.GetAddress().GetAddress(),
		types.TimerDescPair[*Connection]{
			Timeout: pastTimeout,
			Value:   conn,
		})

	// Act + Assert: should not panic even without callback
	require.NotPanics(t, func() {
		n.processConnectingTimeout(time.Now())
	})
}

func TestOnInvalidConnection_NotFiredWhenTimeoutNotReached(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	var callbackCount int32
	n.SetEventHandleOnInvalidConnection(func(node types.Node, conn types.Connection, errCode error_code.ErrorType) error_code.ErrorType {
		callbackCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	conn := CreateConnection(&n, "ipv4://127.0.0.1:9996")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connecting)

	// Timeout in the future
	futureTimeout := time.Now().Add(10 * time.Second)
	n.eventTimer.connectingList.Put(conn.GetAddress().GetAddress(),
		types.TimerDescPair[*Connection]{
			Timeout: futureTimeout,
			Value:   conn,
		})

	// Act
	n.processConnectingTimeout(time.Now())

	// Assert
	assert.Equal(t, int32(0), callbackCount, "should not fire when timeout not yet reached")
}

// ============================================================================
// SetEventHandleOnInvalidConnection — getter/setter
// ============================================================================

func TestSetEventHandleOnInvalidConnection_GetterReturnsSetValue(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	assert.Nil(t, n.GetEventHandleOnInvalidConnection(), "should be nil initially")

	handler := func(node types.Node, conn types.Connection, errCode error_code.ErrorType) error_code.ErrorType {
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	n.SetEventHandleOnInvalidConnection(handler)
	assert.NotNil(t, n.GetEventHandleOnInvalidConnection(), "should be set after SetEventHandle")
}

func TestSetEventHandleOnInvalidConnection_NilNode(t *testing.T) {
	var n *Node
	require.NotPanics(t, func() {
		n.SetEventHandleOnInvalidConnection(nil)
	})
	assert.Nil(t, n.GetEventHandleOnInvalidConnection())
}

// ============================================================================
// FaultTolerant default value
// ============================================================================

func TestFaultTolerantDefault_IsTwo(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	assert.Equal(t, uint32(2), conf.FaultTolerant,
		"default FaultTolerant should be 2 (matching C++ default)")
}
