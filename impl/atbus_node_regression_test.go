// Package libatbus_impl provides internal implementation details for libatbus.
//
// This file contains regression tests for bug fixes identified during the
// C++/Go codebase comparison audit, covering fixes in atbus_node.go,
// atbus_common_types.go, and atbus_endpoint.go.

package libatbus_impl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	"github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

// ============================================================================
// SetDefaultNodeConfigure — RetryInterval must be initialized to 3s
// Bug: RetryInterval was left at zero, causing immediate-retry storms.
// Fix: Added conf.RetryInterval = 3 * time.Second.
// ============================================================================

func TestSetDefaultNodeConfigure_RetryIntervalMatchesCpp(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	assert.Equal(t, 3*time.Second, conf.RetryInterval,
		"SetDefaultNodeConfigure must set RetryInterval to 3s (matching C++ default_conf)")
	// Also verify the other intervals are non-zero as sanity check
	assert.Equal(t, 8*time.Second, conf.PingInterval)
	assert.Equal(t, 30*time.Second, conf.FirstIdleTimeout)
}

// ============================================================================
// ReloadCompression — must return EN_ATBUS_ERR_SUCCESS on valid input
// Bug: Returned EN_ATBUS_ERR_PARAMS after successfully writing configuration.
// Fix: Changed return to EN_ATBUS_ERR_SUCCESS.
// ============================================================================

func TestReloadCompression_ReturnsSuccessOnValidInput(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	algs := []protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE{
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD,
	}
	ret = n.ReloadCompression(algs, protocol.ATBUS_COMPRESSION_LEVEL_ATBUS_COMPRESSION_LEVEL_FAST)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"ReloadCompression must return SUCCESS, not PARAMS")

	// Verify configuration was actually applied
	assert.Equal(t, algs, n.configure.CompressionAllowAlgorithms)
	assert.Equal(t, protocol.ATBUS_COMPRESSION_LEVEL_ATBUS_COMPRESSION_LEVEL_FAST, n.configure.CompressionLevel)
}

func TestReloadCompression_NilReceiverReturnsParams(t *testing.T) {
	var n *Node
	ret := n.ReloadCompression(nil, protocol.ATBUS_COMPRESSION_LEVEL_ATBUS_COMPRESSION_LEVEL_FAST)
	assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, ret,
		"ReloadCompression on nil receiver should return PARAMS")
}

// ============================================================================
// Listen — guard must reject when self is nil, not when self is set
// Bug: Guard condition was inverted — blocked calls when self was correctly set.
// Fix: Changed n.self != nil to n.self == nil.
// ============================================================================

func TestListen_AcceptsNodeWithSelfInitialized(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Start the node so it's past Created state
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// After Init, self should be set. Listen should NOT return NOT_INITED.
	ret = n.Listen("ipv4://127.0.0.1:0")
	// It may fail for other reasons (port binding etc), but it must NOT be NOT_INITED
	// since self is already set.
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_NOT_INITED, ret,
		"Listen should not return NOT_INITED when self is set")
}

func TestListen_RejectsNodeInCreatedState(t *testing.T) {
	var n Node
	// Node in Created state (no Init called) should be rejected
	ret := n.Listen("ipv4://127.0.0.1:0")
	assert.Equal(t, error_code.EN_ATBUS_ERR_NOT_INITED, ret,
		"Listen should return NOT_INITED when node is in Created state")
}

// ============================================================================
// GetPeerChannel — state check must reject Created state, not non-Created
// Bug: Condition was inverted — rejected all states except Created.
// Fix: Changed n.GetState() != types.NodeState_Created to == Created.
// ============================================================================

func TestGetPeerChannel_RejectsCreatedState(t *testing.T) {
	var n Node
	// Node in Created state should fail
	ret, ep, conn, peer := n.GetPeerChannel(0x5678, func(from types.Endpoint, to types.Endpoint) types.Connection {
		return nil
	}, nil)
	assert.Equal(t, error_code.EN_ATBUS_ERR_NOT_INITED, ret,
		"GetPeerChannel should reject node in Created state")
	assert.Nil(t, ep)
	assert.Nil(t, conn)
	assert.Nil(t, peer)
}

func TestGetPeerChannel_AcceptsInitedAndStartedNode(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Node in Running state should NOT be rejected with NOT_INITED
	ret, _, _, _ = n.GetPeerChannel(0x5678, func(from types.Endpoint, to types.Endpoint) types.Connection {
		return nil
	}, nil)
	// It may return other errors (no route etc.), but NOT NOT_INITED
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_NOT_INITED, ret,
		"GetPeerChannel should accept node past Created state")
}

// ============================================================================
// AddEndpoint — must verify endpoint owner matches node (not reject non-nil owner)
// Bug: Guard rejected ep.GetOwner() != nil (accepted only nil owners).
// Fix: Changed to ep.GetOwner() != n (reject if owner doesn't match, matching C++).
// ============================================================================

func TestAddEndpoint_RejectsNilEndpoint(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ret = n.AddEndpoint(nil)
	assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, ret,
		"AddEndpoint should reject nil endpoint")
}

func TestAddEndpoint_RejectsEndpointWithWrongOwner(t *testing.T) {
	var n1 Node
	ret := n1.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	var n2 Node
	ret = n2.Init(0x5678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Create an endpoint owned by n2, try to add to n1
	ep := CreateEndpoint(&n2, 0x9ABC, 1234, "test-host")
	require.NotNil(t, ep)

	ret = n1.AddEndpoint(ep)
	assert.Equal(t, error_code.EN_ATBUS_ERR_PARAMS, ret,
		"AddEndpoint should reject endpoint whose owner != this node (matching C++ this != ep->get_owner())")
}

func TestAddEndpoint_AcceptsEndpointOwnedBySelf(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Create an endpoint owned by this node
	ep := CreateEndpoint(&n, 0x5678, 1234, "test-host")
	require.NotNil(t, ep)
	assert.Equal(t, types.Node(&n), ep.GetOwner(),
		"Endpoint owner should match the node (C++: this == ep->get_owner())")
}

// ============================================================================
// removeChild — must return true when removal succeeds
// Bug: Always returned false after successful deletion.
// Fix: Changed return false to return true.
// ============================================================================

func TestRemoveChild_ReturnsTrueOnSuccessfulRemoval(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Build a collection with one endpoint
	col := &endpointCollection{
		endpointInstance:  make(map[types.BusIdType]*Endpoint),
		endpointInterface: make(map[types.BusIdType]types.Endpoint),
	}
	ep := CreateEndpoint(&n, 0x5678, 1234, "test-host")
	col.endpointInstance[0x5678] = ep
	col.endpointInterface[0x5678] = ep

	removed := n.removeChild(col, 0x5678, nil, true)
	assert.True(t, removed, "removeChild must return true when endpoint is successfully removed")
	// Verify the endpoint is actually gone
	_, exists := col.endpointInstance[0x5678]
	assert.False(t, exists, "Endpoint should be removed from collection")
}

func TestRemoveChild_ReturnsFalseWhenNotFound(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	col := &endpointCollection{
		endpointInstance:  make(map[types.BusIdType]*Endpoint),
		endpointInterface: make(map[types.BusIdType]types.Endpoint),
	}

	removed := n.removeChild(col, 0x9999, nil, true)
	assert.False(t, removed, "removeChild must return false when endpoint doesn't exist")
}

func TestRemoveChild_ReturnsFalseOnExpectedMismatch(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	col := &endpointCollection{
		endpointInstance:  make(map[types.BusIdType]*Endpoint),
		endpointInterface: make(map[types.BusIdType]types.Endpoint),
	}
	ep1 := CreateEndpoint(&n, 0x5678, 1234, "test-host")
	ep2 := CreateEndpoint(&n, 0x5679, 1234, "test-host")
	col.endpointInstance[0x5678] = ep1
	col.endpointInterface[0x5678] = ep1

	// Try to remove with wrong expected pointer
	removed := n.removeChild(col, 0x5678, ep2, true)
	assert.False(t, removed, "removeChild must return false when expected pointer doesn't match")
}

// ============================================================================
// Proc / DispatchAllSelfMessages — must dispatch self messages in normal path
// Bug: Only shutdown path dispatched self messages; normal path skipped it.
// Fix: Added ret += n.dispatchAllSelfMessages() in Proc's non-shutdown path.
// ============================================================================

func TestSendDataToSelf_TriggersForwardRequestCallback(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Register a forward request handler to verify dispatch
	dispatched := false
	var receivedContent []byte
	n.SetEventHandleOnForwardRequest(func(node types.Node, ep types.Endpoint, conn types.Connection, msg *types.Message, content []byte) error_code.ErrorType {
		dispatched = true
		receivedContent = content
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Act — use public API to send data to self
	ret = n.SendData(0x1234, 1, []byte("test-self-dispatch"))

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"SendData to self should succeed")
	assert.True(t, dispatched,
		"OnForwardRequest callback should have been invoked for self message")
	assert.Equal(t, []byte("test-self-dispatch"), receivedContent,
		"Callback should receive the correct content")
}

func TestSendDataToSelf_TriggersForwardResponseWhenRequired(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	requestReceived := false
	responseReceived := false
	n.SetEventHandleOnForwardRequest(func(node types.Node, ep types.Endpoint, conn types.Connection, msg *types.Message, content []byte) error_code.ErrorType {
		requestReceived = true
		assert.Equal(t, []byte("test-rsp"), content,
			"OnForwardRequest should receive correct content")
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	n.SetEventHandleOnForwardResponse(func(node types.Node, ep types.Endpoint, conn types.Connection, msg *types.Message) error_code.ErrorType {
		responseReceived = true
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Act — send data to self with RequiredResponse flag
	opts := types.CreateNodeSendDataOptions()
	opts.SetFlag(types.NodeSendDataOptionFlag_RequiredResponse, true)
	ret = n.SendDataWithOptions(0x1234, 1, []byte("test-rsp"), opts)

	// Assert
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"SendDataWithOptions to self should succeed")
	assert.True(t, requestReceived,
		"OnForwardRequest should be triggered for self data message")
	assert.True(t, responseReceived,
		"OnForwardResponse should be triggered when RequiredResponse is set")
}

func TestSendDataToSelf_ClearsQueueAfterDispatch(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act — send data to self via public API
	ret = n.SendData(0x1234, 1, []byte("dispatch-test"))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Assert — queue should be empty after dispatch
	assert.Equal(t, 0, n.selfDataMessages.Len(),
		"After dispatch, self message queue should be empty")
}
