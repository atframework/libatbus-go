// Package libatbus_impl provides internal implementation details for libatbus.
//
// This file contains targeted tests for NodeGetPeerOptions.blacklist routing
// behavior in GetPeerChannel.

package libatbus_impl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

// testGetCtrlConnectionFn returns a connection-getter function suitable for
// GetPeerChannel that retrieves the control connection (via self.GetCtrlConnection).
func testGetCtrlConnectionFn() func(from types.Endpoint, to types.Endpoint) types.Connection {
	return func(from types.Endpoint, to types.Endpoint) types.Connection {
		return from.GetCtrlConnection(to)
	}
}

// setupNodeWithChild creates a started Node with one child endpoint that has
// a connected control connection. Returns (node, child endpoint, connection).
func setupNodeWithChild(t *testing.T, nodeId, childId types.BusIdType) (*Node, *Endpoint, *Connection) {
	t.Helper()

	var n Node
	ret := n.Init(nodeId, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, childId, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep)
	ret = n.AddEndpoint(ep)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connected)
	require.True(t, ep.AddConnection(conn, false))

	return &n, ep, conn
}

// ============================================================================
// isInGetPeerBlacklist — unit tests
// ============================================================================

func TestIsInGetPeerBlacklist_NilOptions_ReturnsFalse(t *testing.T) {
	assert.False(t, isInGetPeerBlacklist(0x1234, nil))
}

func TestIsInGetPeerBlacklist_EmptyBlacklist_ReturnsFalse(t *testing.T) {
	opts := types.CreateNodeGetPeerOptions()
	assert.False(t, isInGetPeerBlacklist(0x1234, opts))
}

func TestIsInGetPeerBlacklist_IdInList_ReturnsTrue(t *testing.T) {
	opts := types.CreateNodeGetPeerOptions()
	opts.SetBlacklist([]types.BusIdType{0x1000, 0x1234, 0x2000})
	assert.True(t, isInGetPeerBlacklist(0x1234, opts))
}

func TestIsInGetPeerBlacklist_IdNotInList_ReturnsFalse(t *testing.T) {
	opts := types.CreateNodeGetPeerOptions()
	opts.SetBlacklist([]types.BusIdType{0x1000, 0x2000})
	assert.False(t, isInGetPeerBlacklist(0x1234, opts))
}

// ============================================================================
// GetPeerChannel — direct route blacklisted
// ============================================================================

func TestGetPeerChannel_DirectRouteBlacklisted_ReturnsInvalidId(t *testing.T) {
	// Arrange: node with a direct child endpoint
	n, ep, _ := setupNodeWithChild(t, 0x12345678, 0x12345679)

	opts := types.CreateNodeGetPeerOptions()
	opts.SetBlacklist([]types.BusIdType{ep.GetId()})
	// Also set NoUpstream to prevent upstream fallback
	opts.SetFlag(types.NodeGetPeerOptionFlag_NoUpstream, true)

	// Act: try to route to the blacklisted endpoint
	retCode, retEp, retConn, _ := n.GetPeerChannel(ep.GetId(), testGetCtrlConnectionFn(), opts)

	// Assert: should return invalid ID because the only route is blacklisted
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, retCode)
	assert.Nil(t, retEp)
	assert.Nil(t, retConn)
}

func TestGetPeerChannel_DirectRouteNotBlacklisted_Succeeds(t *testing.T) {
	// Arrange
	n, ep, conn := setupNodeWithChild(t, 0x12345678, 0x12345679)

	opts := types.CreateNodeGetPeerOptions()
	// Blacklist a DIFFERENT id
	opts.SetBlacklist([]types.BusIdType{0x99999999})

	// Act
	retCode, retEp, retConn, _ := n.GetPeerChannel(ep.GetId(), testGetCtrlConnectionFn(), opts)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, retCode)
	assert.Equal(t, types.Endpoint(ep), retEp)
	assert.Equal(t, types.Connection(conn), retConn)
}

func TestGetPeerChannel_NoBlacklist_DirectRouteSucceeds(t *testing.T) {
	// Arrange
	n, ep, conn := setupNodeWithChild(t, 0x12345678, 0x12345679)

	opts := types.CreateNodeGetPeerOptions()

	// Act
	retCode, retEp, retConn, _ := n.GetPeerChannel(ep.GetId(), testGetCtrlConnectionFn(), opts)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, retCode)
	assert.Equal(t, types.Endpoint(ep), retEp)
	assert.Equal(t, types.Connection(conn), retConn)
}

// ============================================================================
// GetPeerChannel — upstream blacklisted
// ============================================================================

func TestGetPeerChannel_UpstreamBlacklisted_FallbackToUpstreamBlocked(t *testing.T) {
	// Arrange: node with upstream set, requesting an unknown target so it falls back to upstream
	var n Node
	ret := n.Init(0x12345679, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Set up upstream endpoint
	upstreamEp := CreateEndpoint(&n, 0x12345678, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, upstreamEp)
	n.upstream.node = upstreamEp

	upConn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, upConn)
	upConn.setStatus(types.ConnectionState_Connected)
	upstreamEp.AddConnection(upConn, false)

	// Blacklist the upstream node
	opts := types.CreateNodeGetPeerOptions()
	opts.SetBlacklist([]types.BusIdType{upstreamEp.GetId()})

	// Act: try to route to upstream ID directly
	retCode, retEp, _, _ := n.GetPeerChannel(upstreamEp.GetId(), testGetCtrlConnectionFn(), opts)

	// Assert: upstream is blacklisted, so should return no endpoint
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, retCode)
	assert.Nil(t, retEp)
}

func TestGetPeerChannel_UpstreamBlacklisted_UnknownTargetReturnsInvalidId(t *testing.T) {
	// Arrange: node with upstream, try routing to unknown target (would normally go via upstream)
	var n Node
	ret := n.Init(0x12345679, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	upstreamEp := CreateEndpoint(&n, 0x12345678, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, upstreamEp)
	n.upstream.node = upstreamEp

	upConn := CreateConnection(&n, "ipv4://127.0.0.1:0")
	require.NotNil(t, upConn)
	upConn.setStatus(types.ConnectionState_Connected)
	upstreamEp.AddConnection(upConn, false)

	// Blacklist upstream
	opts := types.CreateNodeGetPeerOptions()
	opts.SetBlacklist([]types.BusIdType{upstreamEp.GetId()})

	// Act: unknown target, should try upstream fallback but it's blacklisted
	retCode, retEp, _, _ := n.GetPeerChannel(0xAAAAAAAA, testGetCtrlConnectionFn(), opts)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, retCode)
	assert.Nil(t, retEp)
}

// ============================================================================
// GetPeerChannel — edge cases
// ============================================================================

func TestGetPeerChannel_TargetIsSelf_ReturnsInvalidId(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	opts := types.CreateNodeGetPeerOptions()
	retCode, _, _, _ := n.GetPeerChannel(0x12345678, testGetCtrlConnectionFn(), opts)
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, retCode)
}

func TestGetPeerChannel_NilNode_ReturnsParams(t *testing.T) {
	var n *Node
	opts := types.CreateNodeGetPeerOptions()
	retCode, _, _, _ := n.GetPeerChannel(0x1234, testGetCtrlConnectionFn(), opts)
	assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, retCode)
}

func TestGetPeerChannel_NotInited_ReturnsNotInited(t *testing.T) {
	// A zero-value Node has state Created (before Init), which triggers NOT_INITED.
	var n Node

	opts := types.CreateNodeGetPeerOptions()
	retCode, _, _, _ := n.GetPeerChannel(0x12345679, testGetCtrlConnectionFn(), opts)
	assert.Equal(t, error_code.EN_ATBUS_ERR_NOT_INITED, retCode)
}

// ============================================================================
// NodeGetPeerOptions — blacklist getter/setter coverage
// ============================================================================

func TestNodeGetPeerOptions_SetAndGetBlacklist(t *testing.T) {
	opts := types.CreateNodeGetPeerOptions()
	assert.Nil(t, opts.GetBlacklist())

	bl := []types.BusIdType{0x1000, 0x2000, 0x3000}
	opts.SetBlacklist(bl)
	assert.Equal(t, bl, opts.GetBlacklist())
}

func TestNodeGetPeerOptions_NilReceiver(t *testing.T) {
	var opts *types.NodeGetPeerOptions
	assert.Nil(t, opts.GetBlacklist())
	// Should not panic
	require.NotPanics(t, func() {
		opts.SetBlacklist([]types.BusIdType{0x1234})
	})
}

func TestNodeGetPeerOptions_FlagOperations(t *testing.T) {
	opts := types.CreateNodeGetPeerOptions()
	assert.False(t, opts.GetFlag(types.NodeGetPeerOptionFlag_NoUpstream))

	opts.SetFlag(types.NodeGetPeerOptionFlag_NoUpstream, true)
	assert.True(t, opts.GetFlag(types.NodeGetPeerOptionFlag_NoUpstream))

	opts.SetFlag(types.NodeGetPeerOptionFlag_NoUpstream, false)
	assert.False(t, opts.GetFlag(types.NodeGetPeerOptionFlag_NoUpstream))
}
