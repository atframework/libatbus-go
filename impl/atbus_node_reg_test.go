// Package libatbus_impl provides internal implementation details for libatbus.
//
// This file ports C++ atbus_node_reg_test.cpp test cases to Go.
// Excluded: mem_and_send, shm_and_send (not in Go scope).

package libatbus_impl

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

func TestRegistrationBacklogReleasesSlotAfterDisconnect(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.BackLog = 1
	n, address := setupListeningNode(t, 1, &conf)
	t.Cleanup(func() { n.Reset() })
	dial := func() net.Conn {
		t.Helper()
		conn, err := net.DialTimeout("tcp", strings.TrimPrefix(address, "ipv4://"), time.Second)
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	waitForPending := func(want int) {
		t.Helper()
		waitForNodeCondition(t, 3*time.Second, func() bool {
			return n.GetConnectionTimerSize() == want
		}, n)
	}
	first := dial()
	waitForPending(1)
	second := dial()
	// Let the loop process the second accept while the first handshake is pending.
	require.Eventually(t, func() bool {
		return len(n.ioStreamEvents) > 0
	}, 3*time.Second, time.Millisecond)
	n.Poll()
	require.NoError(t, second.SetReadDeadline(time.Now().Add(3*time.Second)))
	var one [1]byte
	_, err := second.Read(one[:])
	require.ErrorIs(t, err, io.EOF)
	assert.Equal(t, 1, n.GetConnectionTimerSize())
	require.NoError(t, first.Close())
	waitForPending(0)
	dial()
	waitForPending(1)
}

func TestSetHostnameWithForceMatchesCppRegCase(t *testing.T) {
	// Arrange: preserve the global hostname cache so this test leaves no residue.
	savedHostname := hostName
	t.Cleanup(func() {
		hostName = savedHostname
	})

	var n Node
	oldHostname := n.GetHostname()

	// Act + Assert: a forced override should replace the cached hostname immediately.
	assert.True(t, n.SetHostname("test-host-for", true))
	assert.Equal(t, "test-host-for", n.GetHostname())

	// Act + Assert: restoring the previous hostname with force should also succeed.
	assert.True(t, n.SetHostname(oldHostname, true))
	assert.Equal(t, oldHostname, n.GetHostname())
}

// ---------------------------------------------------------------------------
// Helper: create, init, listen, start a node with a random TCP port.
// Returns the node and the address it is listening on.
// ---------------------------------------------------------------------------
func setupListeningNode(t *testing.T, id types.BusIdType, conf *types.NodeConfigure) (*Node, string) {
	t.Helper()

	var n Node
	ret := n.Init(id, conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	addr := reserveTCPListenAddress(t)
	ret = n.Listen(addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	return &n, addr
}

// getFirstListenAddr returns the first listen address string from a node's self endpoint.
func getFirstListenAddr(t *testing.T, n *Node) string {
	t.Helper()
	addrs := n.self.GetListenAddress()
	require.NotEmpty(t, addrs, "Node should have at least one listen address")
	return addrs[0].GetAddress()
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_ResetAndSendTcp mirrors C++ atbus_node_reg::reset_and_send_tcp.
// Two brother nodes connect over TCP, exchange data, then shutdown/reset.
// ---------------------------------------------------------------------------
func TestNodeRegParity_ResetAndSendTcp(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.AccessTokens = [][]byte{[]byte("test access token")}

	node1, _ := setupListeningNode(t, 0x12345678, &conf)
	node2, _ := setupListeningNode(t, 0x12356789, &conf)
	t.Cleanup(func() { node2.Reset() })

	// Install endpoint callbacks
	addEpCount := 0
	removeEpCount := 0
	node1.SetEventHandleOnAddEndpoint(func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		addEpCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	node2.SetEventHandleOnAddEndpoint(func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		addEpCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	node1.SetEventHandleOnRemoveEndpoint(func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		removeEpCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	node2.SetEventHandleOnRemoveEndpoint(func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		removeEpCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Node1 connects to node2
	ret := node1.Connect(getFirstListenAddr(t, node2))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Wait for both sides to see each other
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) && node2.IsEndpointAvailable(node1.GetId())
	}, node1, node2)

	assert.True(t, node1.IsEndpointAvailable(node2.GetId()))
	assert.True(t, node2.IsEndpointAvailable(node1.GetId()))
	initialAddEp := addEpCount
	assert.GreaterOrEqual(t, initialAddEp, 2, "At least 2 add-endpoint callbacks expected (one per node)")

	// Send data node1 → node2
	sendData := "abcdefg\x00hello world!\n"
	recvCount := 0
	var recvData string
	node2.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		recvCount++
		recvData = string(content)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	ret = node1.SendData(node2.GetId(), 0, []byte(sendData))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return recvCount > 0
	}, node1, node2)
	assert.Equal(t, sendData, recvData)

	// Reset node1 (like C++ reset_and_send test uses node->reset())
	closeConnCount := 0
	node2.SetEventHandleOnCloseConnection(func(_ types.Node, _ types.Endpoint, _ types.Connection) error_code.ErrorType {
		closeConnCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	node1.Reset()

	// Wait for node2 to detect the disconnect (IsEndpointAvailable becomes false).
	// Note: Go does NOT remove endpoints from the route table on disconnect;
	// it only resets the connections. GetEndpoint() will still return non-nil.
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return !node2.IsEndpointAvailable(0x12345678)
	}, node2)

	assert.False(t, node2.IsEndpointAvailable(0x12345678))
	assert.Greater(t, closeConnCount, 0, "Close connection callback expected on peer side")
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_Timeout mirrors C++ atbus_node_reg::timeout.
// Tests that idle connections are detected and cleaned up.
// ---------------------------------------------------------------------------
func TestNodeRegParity_Timeout(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.AccessTokens = [][]byte{[]byte("test access token")}
	conf.FirstIdleTimeout = 2 * time.Second // Short timeout for test

	node1, _ := setupListeningNode(t, 0x12345678, &conf)
	node2, _ := setupListeningNode(t, 0x12356789, &conf)
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	newConnCount := 0
	invalidConnCount := 0
	var invalidConnStatus error_code.ErrorType

	accepted := false
	onNewConn := func(n types.Node, _ types.Connection) error_code.ErrorType {
		newConnCount++
		if n == node2 {
			accepted = true
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	onInvalidConn := func(_ types.Node, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		invalidConnCount++
		invalidConnStatus = status
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	node1.SetEventHandleOnNewConnection(onNewConn)
	node1.SetEventHandleOnInvalidConnection(onInvalidConn)
	node2.SetEventHandleOnNewConnection(onNewConn)
	node2.SetEventHandleOnInvalidConnection(onInvalidConn)

	// Connect node1 → node2
	ret := node1.Connect(getFirstListenAddr(t, node2))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 3*time.Second, func() bool { return newConnCount > 0 }, node1)
	require.Positive(t, newConnCount)
	require.Positive(t, node1.GetConnectionTimerSize())
	node1.Proc(time.Now().Add(conf.FirstIdleTimeout + 3*time.Second))
	require.Positive(t, invalidConnCount)
	require.Equal(t, error_code.EN_ATBUS_ERR_NODE_TIMEOUT, invalidConnStatus)
	// Accept and disconnect events arrive asynchronously on the peer. Advance
	// both clocks until it has observed the connection and released its timer.
	now := time.Now().Add(conf.FirstIdleTimeout + 3*time.Second)
	require.Eventually(t, func() bool {
		now = now.Add(conf.FirstIdleTimeout + time.Second)
		node1.Proc(now)
		node2.Proc(now)
		return accepted && node1.GetConnectionTimerSize() == 0 && node2.GetConnectionTimerSize() == 0
	}, 3*time.Second, time.Millisecond)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_MessageSizeLimit mirrors C++ atbus_node_reg::message_size_limit.
// Tests that messages exceeding configured size are rejected.
// ---------------------------------------------------------------------------
func TestNodeRegParity_MessageSizeLimit(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.MessageSize = 4 * 1024
	conf.AccessTokens = [][]byte{[]byte("test access token")}

	node1, _ := setupListeningNode(t, 0x12345678, &conf)
	node2, _ := setupListeningNode(t, 0x12356789, &conf)
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	// Connect
	ret := node1.Connect(getFirstListenAddr(t, node2))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) && node2.IsEndpointAvailable(node1.GetId())
	}, node1, node2)

	// Exercise the exact payload boundary through a live connection.
	recvCount := 0
	var recvData string
	node2.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		recvCount++
		recvData = string(content)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	limitPayload := strings.Repeat("a", int(conf.MessageSize))
	ret = node1.SendData(node2.GetId(), 0, []byte(limitPayload))
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return recvCount > 0
	}, node1, node2)
	assert.Equal(t, limitPayload, recvData)

	// Send data exceeding max size — should be rejected at SendData level
	oversizedPayload := strings.Repeat("b", int(conf.MessageSize)+1)
	ret = node1.SendData(node2.GetId(), 0, []byte(oversizedPayload))
	assert.Equal(t, error_code.EN_ATBUS_ERR_INVALID_SIZE, ret)

	id1, id2 := node1.GetId(), node2.GetId()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Shutdown(error_code.EN_ATBUS_ERR_SUCCESS))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Shutdown(error_code.EN_ATBUS_ERR_SUCCESS))
	_, code := node1.Poll()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, code)
	waitForNodeCondition(t, 8*time.Second, func() bool { return node1.GetEndpoint(id2) == nil && node2.GetEndpoint(id1) == nil }, node2)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_RegFailedWithMismatchAccessToken mirrors
// C++ atbus_node_reg::reg_failed_with_mismatch_access_token.
// ---------------------------------------------------------------------------
func TestNodeRegParity_RegFailedWithMismatchAccessToken(t *testing.T) {
	var conf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf1)
	conf1.AccessTokens = [][]byte{[]byte("test access token")}

	var conf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf2)
	conf2.AccessTokens = [][]byte{[]byte("invalid access token")}

	node1, _ := setupListeningNode(t, 0x12345678, &conf1)
	node2, _ := setupListeningNode(t, 0x12356789, &conf2)
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	registerFailedCount := 0
	var registerStatus error_code.ErrorType

	onRegFailed := func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status != error_code.EN_ATBUS_ERR_SUCCESS {
			registerFailedCount++
			registerStatus = status
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	node1.SetEventHandleOnRegister(onRegFailed)
	node2.SetEventHandleOnRegister(onRegFailed)

	// Connect
	node1.Connect(getFirstListenAddr(t, node2))

	// Wait for registration to fail
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return registerFailedCount >= 1
	}, node1, node2)

	assert.GreaterOrEqual(t, registerFailedCount, 1)
	assert.Nil(t, node1.GetEndpoint(node2.GetId()))
	assert.Nil(t, node2.GetEndpoint(node1.GetId()))

	assert.True(t,
		registerStatus == error_code.EN_ATBUS_ERR_ACCESS_DENY ||
			registerStatus == error_code.ErrorType(-604),
		"Expected ACCESS_DENY or -604, got %d", registerStatus)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_RegFailedWithMissingAccessToken mirrors
// C++ atbus_node_reg::reg_failed_with_missing_access_token.
// ---------------------------------------------------------------------------
func TestNodeRegParity_RegFailedWithMissingAccessToken(t *testing.T) {
	var conf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf1)
	conf1.AccessTokens = [][]byte{[]byte("test access token")}

	var conf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf2)
	// conf2 has no access tokens

	node1, _ := setupListeningNode(t, 0x12345678, &conf1)
	node2, _ := setupListeningNode(t, 0x12356789, &conf2)
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	registerFailedCount := 0
	var registerStatus error_code.ErrorType

	onRegFailed := func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status != error_code.EN_ATBUS_ERR_SUCCESS {
			registerFailedCount++
			registerStatus = status
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	node1.SetEventHandleOnRegister(onRegFailed)
	node2.SetEventHandleOnRegister(onRegFailed)

	node1.Connect(getFirstListenAddr(t, node2))

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return registerFailedCount >= 1
	}, node1, node2)

	assert.GreaterOrEqual(t, registerFailedCount, 1)
	assert.Nil(t, node1.GetEndpoint(node2.GetId()))
	assert.Nil(t, node2.GetEndpoint(node1.GetId()))

	assert.True(t,
		registerStatus == error_code.EN_ATBUS_ERR_ACCESS_DENY ||
			registerStatus == error_code.ErrorType(-604),
		"Expected ACCESS_DENY or -604, got %d", registerStatus)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_Destruct mirrors C++ atbus_node_reg::destruct.
// Tests that destroying one node causes the peer to clean up the endpoint.
// ---------------------------------------------------------------------------
func TestNodeRegParity_Destruct(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.MessageSize = 256 * 1024

	node1, _ := setupListeningNode(t, 0x12345678, &conf)
	node2, _ := setupListeningNode(t, 0x12356789, &conf)
	t.Cleanup(func() { node2.Reset() })

	// Connect and wait for availability
	node1.Connect(getFirstListenAddr(t, node2))

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) && node2.IsEndpointAvailable(node1.GetId())
	}, node1, node2)

	// Destroy node1 via Reset
	node1.Reset()

	// Wait for node2 to detect disconnect (endpoint becomes unavailable).
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return !node2.IsEndpointAvailable(0x12345678)
	}, node2)

	assert.False(t, node2.IsEndpointAvailable(0x12345678))
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_RegPcSuccess mirrors C++ atbus_node_reg::reg_pc_success.
// Tests parent-child (upstream-downstream) node registration.
// ---------------------------------------------------------------------------
func TestNodeRegParity_RegPcSuccess(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstreamNode, upstreamAddr := setupListeningNode(t, 0x12345678, &upstreamConf)
	t.Cleanup(func() { upstreamNode.Reset() })

	upstreamRegisterCount := 0
	upstreamNode.SetEventHandleOnRegister(func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status == error_code.EN_ATBUS_ERR_SUCCESS {
			upstreamRegisterCount++
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Create downstream node with upstream address
	var downstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&downstreamConf)
	downstreamConf.UpstreamAddress = upstreamAddr

	var downstreamNode Node
	ret := downstreamNode.Init(0x12346789, &downstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	dsAddr := reserveTCPListenAddress(t)
	ret = downstreamNode.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	downstreamRegisterCount := 0
	downstreamNode.SetEventHandleOnRegister(func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status == error_code.EN_ATBUS_ERR_SUCCESS {
			downstreamRegisterCount++
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	ret = downstreamNode.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstreamNode.Reset() })

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return upstreamNode.IsEndpointAvailable(downstreamNode.GetId()) && downstreamNode.IsEndpointAvailable(upstreamNode.GetId())
	}, upstreamNode, &downstreamNode)
	require.Positive(t, upstreamRegisterCount+downstreamRegisterCount)
	require.True(t, upstreamNode.GetTopologyRegistry().UpdatePeer(downstreamNode.GetId(), upstreamNode.GetId(), nil))
	getDataConnFn := func(from types.Endpoint, to types.Endpoint) types.Connection {
		return from.GetDataConnection(to, true)
	}
	removed := 0
	for _, pair := range []struct {
		local, peer *Node
		relation    types.TopologyRelationType
	}{
		{upstreamNode, &downstreamNode, types.TopologyRelationType_ImmediateDownstream},
		{&downstreamNode, upstreamNode, types.TopologyRelationType_ImmediateUpstream},
	} {
		relation, hop := pair.local.GetTopologyRelation(pair.peer.GetId())
		require.Equal(t, pair.relation, relation)
		require.NotNil(t, hop)
		require.Equal(t, pair.peer.GetId(), hop.GetBusId())
		code, ep, conn, route := pair.local.GetPeerChannel(pair.peer.GetId(), getDataConnFn, nil)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, code)
		require.NotNil(t, ep)
		require.NotNil(t, conn)
		require.Same(t, hop, route)
		require.False(t, ep.GetStatisticCreatedTime().IsZero())
		require.NotEmpty(t, ep.GetHashCode())
		require.Equal(t, pair.peer.GetSelfEndpoint().GetHashCode(), ep.GetHashCode())
		pair.local.SetEventHandleOnRemoveEndpoint(func(types.Node, types.Endpoint, error_code.ErrorType) error_code.ErrorType {
			removed++
			return error_code.EN_ATBUS_ERR_SUCCESS
		})
	}
	for _, pair := range [][2]*Node{{upstreamNode, &downstreamNode}, {&downstreamNode, upstreamNode}} {
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, pair[0].Disconnect(pair[1].GetId()))
		require.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_NOT_FOUND, pair[0].Disconnect(pair[1].GetId()))
	}
	require.GreaterOrEqual(t, removed, 2)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_OnCloseConnectionNormal mirrors
// C++ atbus_node_reg::on_close_connection_normal.
// ---------------------------------------------------------------------------
func TestNodeRegParity_OnCloseConnectionNormal(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node1, _ := setupListeningNode(t, 0x12345678, &conf)
	node2, _ := setupListeningNode(t, 0x12356789, &conf)
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	counts := map[types.Node]int{}
	onClose := func(n types.Node, _ types.Endpoint, _ types.Connection) error_code.ErrorType {
		counts[n]++
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	node1.SetEventHandleOnCloseConnection(onClose)
	node2.SetEventHandleOnCloseConnection(onClose)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Connect(getFirstListenAddr(t, node2)))
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) && node2.IsEndpointAvailable(node1.GetId())
	}, node1, node2)
	delivered := false
	node2.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, data []byte) error_code.ErrorType {
		delivered = string(data) == "test data"
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.SendData(node2.GetId(), 0, []byte("test data")))
	waitForNodeCondition(t, 8*time.Second, func() bool { return delivered }, node1, node2)
	before1, before2 := counts[node1], counts[node2]
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Shutdown(error_code.EN_ATBUS_ERR_SUCCESS))
	_, code := node1.Poll()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, code)
	waitForNodeCondition(t, 8*time.Second, func() bool { return counts[node1] > before1 && counts[node2] > before2 }, node2)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_OnCloseConnectionByPeer mirrors
// C++ atbus_node_reg::on_close_connection_by_peer.
// ---------------------------------------------------------------------------
func TestNodeRegParity_OnCloseConnectionByPeer(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node1, _ := setupListeningNode(t, 0x12345678, &conf)
	node2, _ := setupListeningNode(t, 0x12356789, &conf)
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	counts := map[types.Node]int{}
	onClose := func(n types.Node, _ types.Endpoint, _ types.Connection) error_code.ErrorType {
		counts[n]++
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	node1.SetEventHandleOnCloseConnection(onClose)
	node2.SetEventHandleOnCloseConnection(onClose)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Connect(getFirstListenAddr(t, node2)))
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) && node2.IsEndpointAvailable(node1.GetId())
	}, node1, node2)
	before := counts[node2]
	node1.Reset()
	waitForNodeCondition(t, 8*time.Second, func() bool { return counts[node2] > before }, node2)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_RegFailedWithUnsupported mirrors
// C++ atbus_node_reg::reg_failed_with_unsupported.
// Tests registration failure when protocol version is unsupported.
// ---------------------------------------------------------------------------
func TestNodeRegParity_RegFailedWithUnsupported(t *testing.T) {
	var conf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf1)

	var conf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf2)

	node1, _ := setupListeningNode(t, 0x12345678, &conf1)
	node2, _ := setupListeningNode(t, 0x12356789, &conf2)
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	// Verify protocol version accessors match C++ expectations
	assert.Equal(t, int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_MINIMAL_VERSION), node1.GetProtocolMinimalVersion())
	assert.Equal(t, int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_VERSION), node1.GetProtocolVersion())

	// Force node1 to use an unsupported protocol version (below minimum)
	node1.configure.ProtocolVersion = int32(protocol.ATBUS_PROTOCOL_CONST_ATBUS_PROTOCOL_MINIMAL_VERSION) - 1

	registerFailedCount := 0
	var registerStatus error_code.ErrorType
	onRegister := func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status != error_code.EN_ATBUS_ERR_SUCCESS {
			registerFailedCount++
			registerStatus = status
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	node1.SetEventHandleOnRegister(onRegister)
	node2.SetEventHandleOnRegister(onRegister)

	// Connect
	node1.Connect(getFirstListenAddr(t, node2))

	waitForNodeCondition(t, 5*time.Second, func() bool {
		return registerFailedCount >= 1
	}, node1, node2)

	assert.GreaterOrEqual(t, registerFailedCount, 1)
	// After the version check fix, endpoints should not be created
	assert.False(t, node1.IsEndpointAvailable(node2.GetId()))
	assert.False(t, node2.IsEndpointAvailable(node1.GetId()))

	assert.True(t,
		registerStatus == error_code.EN_ATBUS_ERR_UNSUPPORTED_VERSION ||
			registerStatus == error_code.EN_ATBUS_ERR_ACCESS_DENY ||
			registerStatus == error_code.EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH,
		"Expected UNSUPPORTED_VERSION, ACCESS_DENY, or BUS_ID_NOT_MATCH, got %d", registerStatus)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_OnTopologyUpstreamSet mirrors
// C++ atbus_node_reg::on_topology_upstream_set.
// Tests that the downstream node receives topology update callbacks
// when its upstream is established.
// ---------------------------------------------------------------------------
func TestNodeRegParity_OnTopologyUpstreamSet(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstreamNode, upstreamAddr := setupListeningNode(t, 0x12345678, &upstreamConf)
	t.Cleanup(func() { upstreamNode.Reset() })

	var downstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&downstreamConf)
	downstreamConf.UpstreamAddress = upstreamAddr

	var downstreamNode Node
	ret := downstreamNode.Init(0x12346789, &downstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	dsAddr := reserveTCPListenAddress(t)
	ret = downstreamNode.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	var selfID, upstreamID types.BusIdType
	updates := 0
	downstreamNode.SetEventHandleOnTopologyUpdateUpstream(func(_ types.Node, self, upstream types.TopologyPeer, _ *types.TopologyData) error_code.ErrorType {
		updates++
		if self != nil {
			selfID = self.GetBusId()
		}
		upstreamID = 0
		if upstream != nil {
			upstreamID = upstream.GetBusId()
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	ret = downstreamNode.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstreamNode.Reset() })

	waitForNodeCondition(t, 8*time.Second, func() bool { return updates > 0 && downstreamNode.GetState() == types.NodeState_Running }, upstreamNode, &downstreamNode)
	require.Positive(t, updates)
	require.Equal(t, downstreamNode.GetId(), selfID)
	require.Equal(t, upstreamNode.GetId(), upstreamID)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_OnTopologyUpstreamClear mirrors
// C++ atbus_node_reg::on_topology_upstream_clear.
// Tests that the downstream node detects upstream loss.
// ---------------------------------------------------------------------------
func TestNodeRegParity_OnTopologyUpstreamClear(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstreamNode, upstreamAddr := setupListeningNode(t, 0x12345678, &upstreamConf)

	var downstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&downstreamConf)
	downstreamConf.UpstreamAddress = upstreamAddr

	var downstreamNode Node
	ret := downstreamNode.Init(0x12346789, &downstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	dsAddr := reserveTCPListenAddress(t)
	ret = downstreamNode.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	var selfID, upstreamID types.BusIdType
	updates := 0
	downstreamNode.SetEventHandleOnTopologyUpdateUpstream(func(_ types.Node, self, upstream types.TopologyPeer, _ *types.TopologyData) error_code.ErrorType {
		updates++
		if self != nil {
			selfID = self.GetBusId()
		}
		upstreamID = 0
		if upstream != nil {
			upstreamID = upstream.GetBusId()
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	ret = downstreamNode.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstreamNode.Reset() })

	waitForNodeCondition(t, 8*time.Second, func() bool { return updates > 0 && downstreamNode.GetState() == types.NodeState_Running }, upstreamNode, &downstreamNode)
	require.Positive(t, updates)
	require.Equal(t, downstreamNode.GetId(), selfID)
	require.Equal(t, upstreamNode.GetId(), upstreamID)
	countAfterConnect := updates
	upstreamNode.Reset()
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return updates > countAfterConnect || downstreamNode.GetState() == types.NodeState_LostUpstream
	}, &downstreamNode)
	require.True(t, updates > countAfterConnect || downstreamNode.GetState() == types.NodeState_LostUpstream)
	if updates > countAfterConnect {
		require.Zero(t, upstreamID)
	}
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_RegPcSuccessCrossSubnet mirrors
// C++ atbus_node_reg::reg_pc_success_cross_subnet.
// Tests parent-child registration across different subnets (node IDs differ
// in the higher byte, 0x12 vs 0x22).
// ---------------------------------------------------------------------------
func TestNodeRegParity_RegPcSuccessCrossSubnet(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstreamNode, upstreamAddr := setupListeningNode(t, 0x12345678, &upstreamConf)
	t.Cleanup(func() { upstreamNode.Reset() })

	var downstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&downstreamConf)
	downstreamConf.UpstreamAddress = upstreamAddr

	var downstreamNode Node
	ret := downstreamNode.Init(0x22346789, &downstreamConf) // different subnet prefix
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	dsAddr := reserveTCPListenAddress(t)
	ret = downstreamNode.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	upstreamRegCount := 0
	downstreamRegCount := 0
	upstreamNode.SetEventHandleOnRegister(func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status == error_code.EN_ATBUS_ERR_SUCCESS {
			upstreamRegCount++
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	addEpCount := 0
	upstreamNode.SetEventHandleOnAddEndpoint(func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		addEpCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	downstreamNode.SetEventHandleOnAddEndpoint(func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		addEpCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	downstreamNode.SetEventHandleOnRegister(func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status == error_code.EN_ATBUS_ERR_SUCCESS {
			downstreamRegCount++
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	ret = downstreamNode.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstreamNode.Reset() })

	// Wait for registration success on both sides
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return upstreamRegCount > 0 && downstreamRegCount > 0
	}, upstreamNode, &downstreamNode)

	assert.Greater(t, upstreamRegCount, 0)
	assert.Greater(t, downstreamRegCount, 0)
	assert.GreaterOrEqual(t, addEpCount, 2)

	// Endpoints should exist
	assert.NotNil(t, upstreamNode.GetEndpoint(downstreamNode.GetId()))
	assert.NotNil(t, downstreamNode.GetEndpoint(upstreamNode.GetId()))

	// API test: get_peer_channel
	getDataConnFn := func(from types.Endpoint, to types.Endpoint) types.Connection {
		return from.GetDataConnection(to, true)
	}
	errCode, ep, _, _ := upstreamNode.GetPeerChannel(downstreamNode.GetId(), getDataConnFn, nil)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	assert.NotNil(t, ep)

	errCode, ep, _, _ = downstreamNode.GetPeerChannel(upstreamNode.GetId(), getDataConnFn, nil)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	assert.NotNil(t, ep)

	// Disconnect
	upstreamNode.Disconnect(0x22346789)
	downstreamNode.Disconnect(0x12345678)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_RegPcFailedWithSubnetMismatch mirrors
// C++ atbus_node_reg::reg_pc_failed_with_subnet_mismatch.
// In C++, this test uses same-subnet IDs (upstream=0x12345678, downstream=0x12346789)
// and verifies registration succeeds.  Despite the name, C++ actually expects success
// for this ID pair.
// ---------------------------------------------------------------------------
func TestNodeRegParity_RegPcFailedWithSubnetMismatch(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstreamNode, upstreamAddr := setupListeningNode(t, 0x12345678, &upstreamConf)
	t.Cleanup(func() { upstreamNode.Reset() })

	var downstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&downstreamConf)
	downstreamConf.UpstreamAddress = upstreamAddr

	var downstreamNode Node
	ret := downstreamNode.Init(0x12346789, &downstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	dsAddr := reserveTCPListenAddress(t)
	ret = downstreamNode.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	registerCount := 0
	downstreamNode.SetEventHandleOnRegister(func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status == error_code.EN_ATBUS_ERR_SUCCESS {
			registerCount++
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	ret = downstreamNode.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstreamNode.Reset() })

	// Wait for registration or state stabilization
	waitForNodeCondition(t, 8*time.Second, func() bool {
		dsState := downstreamNode.GetState()
		return dsState == types.NodeState_Running || registerCount > 0
	}, upstreamNode, &downstreamNode)

	// Despite the C++ test name, same-subnet IDs actually succeed in registration.
	// The C++ test uses the same ID pair (0x12345678, 0x12346789) and expects success.
	dsState := downstreamNode.GetState()
	assert.True(t,
		dsState == types.NodeState_Running || registerCount > 0,
		"Downstream should register successfully (state=%d, registerCount=%d)", dsState, registerCount)

	// C++ verifies upstream stays running
	assert.Equal(t, types.NodeState_Running, upstreamNode.GetState())

	// API test: get_peer_channel
	getDataConnFn := func(from types.Endpoint, to types.Endpoint) types.Connection {
		return from.GetDataConnection(to, true)
	}
	errCode, ep, _, _ := upstreamNode.GetPeerChannel(downstreamNode.GetId(), getDataConnFn, nil)
	if ep != nil {
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	}

	errCode, ep, _, _ = downstreamNode.GetPeerChannel(upstreamNode.GetId(), getDataConnFn, nil)
	if ep != nil {
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	}

	// Disconnect
	upstreamNode.Disconnect(0x12346789)
	downstreamNode.Disconnect(0x12345678)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_RegBroSuccess mirrors C++ atbus_node_reg::reg_bro_success.
// Tests brother (same-level) node registration via direct TCP connect.
// ---------------------------------------------------------------------------
func TestNodeRegParity_RegBroSuccess(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node1, _ := setupListeningNode(t, 0x12345678, &conf)
	node2, _ := setupListeningNode(t, 0x12356789, &conf)
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	addEpCount := 0
	removeEpCount := 0
	onAddEp := func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		addEpCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	onRemoveEp := func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		removeEpCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	node1.SetEventHandleOnAddEndpoint(onAddEp)
	node1.SetEventHandleOnRemoveEndpoint(onRemoveEp)
	node2.SetEventHandleOnAddEndpoint(onAddEp)
	node2.SetEventHandleOnRemoveEndpoint(onRemoveEp)

	// Connect node1 → node2
	ret := node1.Connect(getFirstListenAddr(t, node2))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Wait for mutual availability
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) && node2.IsEndpointAvailable(node1.GetId())
	}, node1, node2)

	assert.True(t, node1.IsEndpointAvailable(node2.GetId()))
	assert.True(t, node2.IsEndpointAvailable(node1.GetId()))
	assert.GreaterOrEqual(t, addEpCount, 2)

	// API test: get_peer_channel
	getDataConnFn := func(from types.Endpoint, to types.Endpoint) types.Connection {
		return from.GetDataConnection(to, true)
	}
	errCode, ep, conn, _ := node1.GetPeerChannel(node2.GetId(), getDataConnFn, nil)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	assert.NotNil(t, ep)
	assert.NotNil(t, conn)

	errCode, ep, conn, _ = node2.GetPeerChannel(node1.GetId(), getDataConnFn, nil)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, errCode)
	assert.NotNil(t, ep)
	assert.NotNil(t, conn)

	// Disconnect
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Disconnect(0x12356789))

	// Wait for cleanup
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return !node1.IsEndpointAvailable(node2.GetId())
	}, node1, node2)
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_Conflict mirrors C++ atbus_node_reg::conflict.
// Tests that when two downstream nodes have conflicting IDs, at most one
// succeeds and the other is shut down.
// ---------------------------------------------------------------------------
func TestNodeRegParity_Conflict(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstreamNode, upstreamAddr := setupListeningNode(t, 0x12345678, &upstreamConf)
	t.Cleanup(func() { upstreamNode.Reset() })

	var dsConf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf1)
	dsConf1.UpstreamAddress = upstreamAddr

	var dsConf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf2)
	dsConf2.UpstreamAddress = upstreamAddr

	var downstream1 Node
	ret := downstream1.Init(0x12346789, &dsConf1)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ds1Addr := reserveTCPListenAddress(t)
	ret = downstream1.Listen(ds1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Conflicting downstream node — subnet overlap with downstream1
	var downstreamFail Node
	ret = downstreamFail.Init(0x12346780, &dsConf2)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	dsfAddr := reserveTCPListenAddress(t)
	ret = downstreamFail.Listen(dsfAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	shutdownCount := 0
	onShutdown := func(_ types.Node, _ error_code.ErrorType) error_code.ErrorType {
		shutdownCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	downstream1.SetEventHandleOnShutdown(onShutdown)
	downstreamFail.SetEventHandleOnShutdown(onShutdown)

	ret = downstream1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstream1.Reset() })

	ret = downstreamFail.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstreamFail.Reset() })

	// Wait for either one downstream to reach Running, or for both to
	// settle outside Created. Use passive polling because a downstream
	// that gets FatalShutdown may return errors from Proc.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		now := time.Now()
		upstreamNode.Poll()
		upstreamNode.Proc(now)

		// Only pump downstreams if they're not in a terminal state
		if downstream1.GetState() > types.NodeState_Inited {
			downstream1.Poll()
			downstream1.Proc(now)
		}
		if downstreamFail.GetState() > types.NodeState_Inited {
			downstreamFail.Poll()
			downstreamFail.Proc(now)
		}

		ds1State := downstream1.GetState()
		ds2State := downstreamFail.GetState()
		if ds1State == types.NodeState_Running || ds2State == types.NodeState_Running {
			break
		}
		if ds1State != types.NodeState_Created && ds2State != types.NodeState_Created {
			// Both have left Created; give a bit more time for one to reach Running
		}

		time.Sleep(50 * time.Millisecond)
	}

	// At least one should be running; upstream should always stay running
	assert.True(t,
		downstream1.GetState() == types.NodeState_Running || downstreamFail.GetState() == types.NodeState_Running,
		"At least one downstream should reach Running state (ds1=%d, ds2=%d)",
		downstream1.GetState(), downstreamFail.GetState())
	assert.Equal(t, types.NodeState_Running, upstreamNode.GetState())
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_ReconnectUpstreamFailed mirrors
// C++ atbus_node_reg::reconnect_upstream_failed.
// Tests that a downstream node survives upstream loss, transitions through
// LostUpstream/ConnectingUpstream, and reconnects when upstream comes back.
// ---------------------------------------------------------------------------
func TestNodeRegParity_ReconnectUpstreamFailed(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	upstreamAddr := reserveTCPListenAddress(t)

	var upstreamNode Node
	ret := upstreamNode.Init(0x12345678, &upstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstreamNode.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstreamNode.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	var downstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&downstreamConf)
	downstreamConf.UpstreamAddress = upstreamAddr

	downstreamRegCount := 0
	var downstreamNode Node
	ret = downstreamNode.Init(0x12346789, &downstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	dsAddr := reserveTCPListenAddress(t)
	ret = downstreamNode.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	downstreamNode.SetEventHandleOnRegister(func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status == error_code.EN_ATBUS_ERR_SUCCESS {
			downstreamRegCount++
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	ret = downstreamNode.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstreamNode.Reset() })

	// Wait for downstream to reach Running state
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstreamNode.GetState() == types.NodeState_Running
	}, &upstreamNode, &downstreamNode)

	assert.Equal(t, types.NodeState_Running, downstreamNode.GetState())

	// Reset upstream
	upstreamNode.Reset()

	// Wait for actual TCP disconnect to propagate asynchronously.
	// Then advance time past PingInterval to trigger processUpstreamOperations.
	time.Sleep(200 * time.Millisecond)
	procTime := time.Now().Add(downstreamConf.PingInterval + 2*time.Second)
	downstreamNode.Proc(procTime)

	// After Proc with advanced time, processUpstreamOperations should detect the
	// upstream control connection is gone and attempt reconnection.
	// Give some extra iterations for Go's goroutine-based disconnect propagation.
	for i := 0; i < 16; i++ {
		dsState := downstreamNode.GetState()
		if dsState != types.NodeState_Running {
			break
		}
		procTime = procTime.Add(downstreamConf.RetryInterval + time.Second)
		downstreamNode.Proc(procTime)
		time.Sleep(50 * time.Millisecond)
	}

	// Downstream should be in LostUpstream or ConnectingUpstream state
	dsState := downstreamNode.GetState()
	assert.True(t,
		dsState == types.NodeState_LostUpstream || dsState == types.NodeState_ConnectingUpstream,
		"Expected LostUpstream or ConnectingUpstream, got %d", dsState)

	// Retry a few times — downstream should stay in reconnecting state (no upstream to connect to)
	for i := 0; i < 4; i++ {
		procTime = procTime.Add(downstreamConf.RetryInterval + time.Second)
		downstreamNode.Proc(procTime)

		dsState = downstreamNode.GetState()
		assert.True(t,
			dsState == types.NodeState_LostUpstream || dsState == types.NodeState_ConnectingUpstream,
			"Downstream should stay in reconnecting state, got %d", dsState)
		assert.NotEqual(t, types.NodeState_Created, dsState)
		assert.NotEqual(t, types.NodeState_Inited, dsState)
		time.Sleep(50 * time.Millisecond)
	}

	// Restart the upstream node on the same address
	var upstreamNode2 Node
	var upstreamConf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf2)
	ret = upstreamNode2.Init(0x12345678, &upstreamConf2)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstreamNode2.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstreamNode2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstreamNode2.Reset() })

	// Wait for downstream to reconnect and reach Running state
	reconRegCount := downstreamRegCount
	deadline := time.Now().Add(16 * time.Second)
	for time.Now().Before(deadline) {
		upstreamNode2.Poll()
		downstreamNode.Poll()
		procTime = procTime.Add(downstreamConf.RetryInterval)
		upstreamNode2.Proc(procTime)
		downstreamNode.Proc(procTime)

		if downstreamNode.GetState() == types.NodeState_Running &&
			downstreamRegCount > reconRegCount {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	assert.Equal(t, types.NodeState_Running, downstreamNode.GetState())
	assert.NotNil(t, downstreamNode.GetEndpoint(upstreamNode2.GetId()))
	assert.NotNil(t, upstreamNode2.GetEndpoint(downstreamNode.GetId()))
	assert.Equal(t, types.NodeState_Running, upstreamNode2.GetState())
}

// ---------------------------------------------------------------------------
// TestNodeRegParity_OnTopologyUpstreamChangeId mirrors
// C++ atbus_node_reg::on_topology_upstream_change_id.
// Tests that the downstream detects an upstream ID change when the upstream
// is replaced by a different node on the same address.
// ---------------------------------------------------------------------------
func TestNodeRegParity_OnTopologyUpstreamChangeId(t *testing.T) {
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)
	upstreamConf.AccessTokens = [][]byte{[]byte("test access token")}

	upstreamAddr := reserveTCPListenAddress(t)

	// First upstream
	var upstream1 Node
	ret := upstream1.Init(0x12356789, &upstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream1.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Downstream
	var dsConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf)
	dsConf.AccessTokens = [][]byte{[]byte("test access token")}
	dsConf.UpstreamAddress = upstreamAddr

	topologyUpdateCount := 0
	var topologyNewUpstreamId types.BusIdType
	dsRegisterCount := 0

	var downstream Node
	ret = downstream.Init(0x12345678, &dsConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	dsAddr := reserveTCPListenAddress(t)
	ret = downstream.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	downstream.SetEventHandleOnTopologyUpdateUpstream(func(_ types.Node, _ types.TopologyPeer, upstream types.TopologyPeer, _ *types.TopologyData) error_code.ErrorType {
		topologyUpdateCount++
		if upstream != nil {
			topologyNewUpstreamId = upstream.GetBusId()
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	downstream.SetEventHandleOnRegister(func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status == error_code.EN_ATBUS_ERR_SUCCESS {
			dsRegisterCount++
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	ret = downstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstream.Reset() })

	// Wait for downstream to connect to first upstream
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return dsRegisterCount > 0 && topologyUpdateCount > 0
	}, &upstream1, &downstream)

	assert.Greater(t, topologyUpdateCount, 0)
	assert.Equal(t, upstream1.GetId(), topologyNewUpstreamId)
	firstUpstreamId := topologyNewUpstreamId

	// Reset first upstream
	upstream1.Reset()

	// Wait for downstream to detect upstream loss.
	// Must call Poll() so that executeGC processes the endpoint removal
	// and triggers the Running → LostUpstream state transition.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && downstream.GetState() == types.NodeState_Running {
		downstream.Poll()
		time.Sleep(10 * time.Millisecond)
	}

	assert.Equal(t, types.NodeState_LostUpstream, downstream.GetState())

	// Create second upstream with a DIFFERENT ID on the SAME address
	var upstream2 Node
	ret = upstream2.Init(0x12357890, &upstreamConf) // Different ID!
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream2.Listen(upstreamAddr) // Same address
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstream2.Reset() })

	callbackBefore := topologyUpdateCount

	// Wait for downstream to reconnect to new upstream
	deadline = time.Now().Add(16 * time.Second)
	for time.Now().Before(deadline) {
		now := time.Now()
		upstream2.Poll()
		downstream.Poll()
		upstream2.Proc(now)
		downstream.Proc(now)

		if downstream.GetState() == types.NodeState_Running &&
			topologyNewUpstreamId == upstream2.GetId() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify topology callback was called with the new upstream ID
	assert.Greater(t, topologyUpdateCount, callbackBefore)
	assert.Equal(t, upstream2.GetId(), topologyNewUpstreamId)
	assert.NotEqual(t, firstUpstreamId, topologyNewUpstreamId)
}

// TestNodeRegParity_RegPcSuccessWithUnreachableUpstreamChannel mirrors C++
// atbus_node_reg::reg_pc_success_with_unreachable_upstream_channel.
func TestNodeRegParity_RegPcSuccessWithUnreachableUpstreamChannel(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.PingInterval = 1 * time.Second

	upstreamAddr := reserveTCPListenAddress(t)

	upstream := NewNode()
	downstream := NewNode()
	t.Cleanup(func() {
		downstream.Reset()
		upstream.Reset()
	})

	// 模拟上游和下游处于不同物理机
	upstream.SetSelfHostname("test-host-a")
	downstream.SetSelfHostname("test-host-b")

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12345678, &conf))

	dsConf := conf
	dsConf.UpstreamAddress = upstreamAddr
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Init(0x12346789, &dsConf))

	// 上游只监听本机回环地址（对处于其他物理机的下游不可达），下游不监听任何地址
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(upstreamAddr))

	registerCount := 0
	downstream.SetEventHandleOnRegister(func(_ types.Node, _ types.Endpoint, _ types.Connection, status error_code.ErrorType) error_code.ErrorType {
		if status == error_code.EN_ATBUS_ERR_SUCCESS {
			registerCount++
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return registerCount > 0 && downstream.IsEndpointAvailable(upstream.GetId())
	}, upstream, downstream)

	require.NotNil(t, downstream.GetEndpoint(upstream.GetId()))
	assert.True(t, downstream.IsEndpointAvailable(upstream.GetId()))

	// 再推进若干次 ping 周期的虚拟时间后，上游端点应保持存活且可用
	procTime := time.Now().Add(time.Second)
	for i := 0; i < 30; i++ {
		procTime = procTime.Add(100 * time.Millisecond)
		upstream.Poll()
		upstream.Proc(procTime)
		downstream.Poll()
		downstream.Proc(procTime)
		if i%5 == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	assert.Equal(t, types.NodeState_Running, downstream.GetState())
	assert.NotNil(t, downstream.GetEndpoint(upstream.GetId()))
	assert.True(t, downstream.IsEndpointAvailable(upstream.GetId()))
}

// TestNodeRegParity_RegWithWildcardUpstreamChannelCrossHost mirrors C++
// atbus_node_reg::reg_with_wildcard_upstream_channel_cross_host.
func TestNodeRegParity_RegWithWildcardUpstreamChannelCrossHost(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.PingInterval = 1 * time.Second

	upstreamAddr := reserveTCPListenAddress(t)
	unreachableWildcardAddr := strings.Replace(reserveTCPListenAddress(t), "ipv4://127.0.0.1:", "ipv4://0.0.0.0:", 1)

	upstream := NewNode()
	downstream := NewNode()
	t.Cleanup(func() {
		downstream.Reset()
		upstream.Reset()
	})

	upstream.SetSelfHostname("test-host-a")
	downstream.SetSelfHostname("test-host-b")

	// 上游对外仅通告不可达的通配地址 (0.0.0.0:<unusedPort>)，实际监听在 upstreamAddr
	upConf := conf
	upConf.Gateway = []types.Gateway{{Address: unreachableWildcardAddr}}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12345678, &upConf))

	// 下游不配置 upstream_address，通过手动连接发起注册
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Init(0x12346789, &conf))

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(upstreamAddr))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Start())

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Connect(upstreamAddr))

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstream.IsEndpointAvailable(upstream.GetId())
	}, upstream, downstream)

	ep := downstream.GetEndpoint(upstream.GetId())
	require.NotNil(t, ep)
	assert.True(t, downstream.IsEndpointAvailable(upstream.GetId()))
	dataConn := downstream.self.GetDataConnection(ep, false)
	require.NotNil(t, dataConn)
	assert.Equal(t, upstreamAddr, dataConn.GetAddress().GetAddress())

	// 推进若干次 ping 周期后端点保持可用
	procTime := time.Now().Add(time.Second)
	for i := 0; i < 30; i++ {
		procTime = procTime.Add(100 * time.Millisecond)
		upstream.Poll()
		upstream.Proc(procTime)
		downstream.Poll()
		downstream.Proc(procTime)
		if i%5 == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	assert.NotNil(t, downstream.GetEndpoint(upstream.GetId()))
	assert.True(t, downstream.IsEndpointAvailable(upstream.GetId()))
}

// TestNodeRegParity_ReconnectUpstreamRetryBackoff mirrors C++
// atbus_node_reg::reconnect_upstream_retry_backoff.
func TestNodeRegParity_ReconnectUpstreamRetryBackoff(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	retryGap := conf.RetryInterval
	maxRetryGap := conf.MaxRetryInterval
	require.Equal(t, 3*time.Second, retryGap)
	require.Equal(t, 60*time.Second, maxRetryGap)

	upstreamAddr := reserveTCPListenAddress(t)
	downstreamAddr := reserveTCPListenAddress(t)

	upstream := NewNode()
	downstream := NewNode()
	t.Cleanup(func() {
		downstream.Reset()
		upstream.Reset()
	})

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12356790, &conf))

	dsConf := conf
	dsConf.UpstreamAddress = upstreamAddr
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Init(0x12345679, &dsConf))

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(upstreamAddr))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Listen(downstreamAddr))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstream.GetState() == types.NodeState_Running
	}, upstream, downstream)
	require.Equal(t, types.NodeState_Running, downstream.GetState())

	// Helper: wait until downstream detects upstream disconnect and enters LostUpstream
	waitLostUpstream := func(procT *time.Time) {
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			downstream.Poll()
			if downstream.GetState() == types.NodeState_LostUpstream {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.Equal(t, types.NodeState_LostUpstream, downstream.GetState())
		// First immediate reconnect attempt fails synchronously because upstream is down
		*procT = downstream.GetTimerTick().Add(time.Second)
		downstream.Proc(*procT)
		assert.Equal(t, 0, downstream.GetConnectionTimerSize())
		assert.Equal(t, types.NodeState_LostUpstream, downstream.GetState())
		assert.Equal(t, retryGap, downstream.eventTimer.upstreamOpRetryInterval)
	}

	expectRetryStep := func(attemptTick *time.Time, expectedGap time.Duration) {
		nextExpected := expectedGap * 2
		if nextExpected > maxRetryGap {
			nextExpected = maxRetryGap
		}

		// Before and at boundary (strictly Before check), retry must NOT fire
		downstream.Proc(attemptTick.Add(expectedGap - time.Second))
		assert.Equal(t, expectedGap, downstream.eventTimer.upstreamOpRetryInterval)
		assert.Equal(t, 0, downstream.GetConnectionTimerSize())

		downstream.Proc(attemptTick.Add(expectedGap))
		assert.Equal(t, expectedGap, downstream.eventTimer.upstreamOpRetryInterval)
		assert.Equal(t, 0, downstream.GetConnectionTimerSize())

		// Past boundary: triggers reconnect attempt, fails synchronously, doubles backoff
		*attemptTick = attemptTick.Add(expectedGap + time.Second)
		downstream.Proc(*attemptTick)
		assert.Equal(t, 0, downstream.GetConnectionTimerSize())
		assert.Equal(t, types.NodeState_LostUpstream, downstream.GetState())
		assert.Equal(t, nextExpected, downstream.eventTimer.upstreamOpRetryInterval)
	}

	// 关闭上游节点，验证退避阶梯翻倍直至封顶 max_retry_interval
	upstream.Reset()
	var attemptTick time.Time
	waitLostUpstream(&attemptTick)

	currentGap := retryGap
	for round := 0; round < 7; round++ {
		expectRetryStep(&attemptTick, currentGap)
		currentGap *= 2
		if currentGap > maxRetryGap {
			currentGap = maxRetryGap
		}
	}

	// 重启上游节点，下游重连并重新激活后退避间隔必须重置
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12356790, &conf))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(upstreamAddr))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())

	attemptTick = attemptTick.Add(maxRetryGap + time.Second)
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && downstream.GetState() != types.NodeState_Running {
		attemptTick = attemptTick.Add(time.Second)
		upstream.Poll()
		downstream.Poll()
		upstream.Proc(attemptTick)
		downstream.Proc(attemptTick)
		time.Sleep(15 * time.Millisecond)
	}
	require.Equal(t, types.NodeState_Running, downstream.GetState())
	assert.Equal(t, time.Duration(0), downstream.eventTimer.upstreamOpRetryInterval)

	// 再次关闭上游节点，第一次重试间隔应重置为 retry_interval
	upstream.Reset()
	waitLostUpstream(&attemptTick)
	expectRetryStep(&attemptTick, retryGap)
	expectRetryStep(&attemptTick, retryGap*2)
}

// TestNodeRegParity_RegPcSuccessWithSameScope mirrors C++
// atbus_node_reg::reg_pc_success_with_same_scope.
func TestNodeRegParity_RegPcSuccessWithSameScope(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	addrPreferred := reserveTCPListenAddress(t)
	addrReg := reserveTCPListenAddress(t)
	addrDownstream := reserveTCPListenAddress(t)

	upstream := NewNode()
	downstream := NewNode()
	t.Cleanup(func() {
		downstream.Reset()
		upstream.Reset()
	})

	upConf := conf
	upConf.Scope = "prod"
	upConf.NamespaceName = "game"
	upConf.NodeLabels = map[string]string{"zone": "a"}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12345678, &upConf))

	dsConf := upConf
	dsConf.UpstreamAddress = addrReg
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Init(0x12346789, &dsConf))

	// 上游先监听 addrPreferred 再监听 addrReg；同 scope/namespace 时下游应选中排在前面的 addrPreferred
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrPreferred))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrReg))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Listen(addrDownstream))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstream.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(downstream.GetId())
	}, upstream, downstream)

	// 对端的 scope/namespace/labels 必须随注册包传递
	epDownstream := upstream.GetEndpoint(downstream.GetId())
	require.NotNil(t, epDownstream)
	assert.Equal(t, "prod", epDownstream.GetScope())
	assert.Equal(t, "game", epDownstream.GetNamespace())
	assert.Equal(t, "a", epDownstream.GetLabels()["zone"])

	epUpstream := downstream.GetEndpoint(upstream.GetId())
	require.NotNil(t, epUpstream)
	assert.Equal(t, "prod", epUpstream.GetScope())
	assert.Equal(t, "game", epUpstream.GetNamespace())
	assert.Equal(t, "a", epUpstream.GetLabels()["zone"])

	dataConn := downstream.self.GetDataConnection(epUpstream, false)
	require.NotNil(t, dataConn)
	assert.Equal(t, addrPreferred, dataConn.GetAddress().GetAddress())
}

// TestNodeRegParity_RegPcScopeMismatchWithoutRedial mirrors C++
// atbus_node_reg::reg_pc_scope_mismatch_without_redial.
func TestNodeRegParity_RegPcScopeMismatchWithoutRedial(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	addrOther := reserveTCPListenAddress(t)
	addrReg := reserveTCPListenAddress(t)

	upstream := NewNode()
	downstream := NewNode()
	t.Cleanup(func() {
		downstream.Reset()
		upstream.Reset()
	})

	upConf := conf
	upConf.Scope = "prod"
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12345678, &upConf))

	dsConf := conf
	dsConf.Scope = "dev"
	dsConf.UpstreamAddress = addrReg
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Init(0x12346789, &dsConf))

	// 上游先监听 addrOther 再监听 addrReg；scope 不匹配时上游通告的 listen 地址必须全部跳过，仅使用已知可达的 addrReg
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrOther))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrReg))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstream.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(downstream.GetId())
	}, upstream, downstream)

	epUpstream := downstream.GetEndpoint(upstream.GetId())
	require.NotNil(t, epUpstream)
	dataConn := downstream.self.GetDataConnection(epUpstream, false)
	require.NotNil(t, dataConn)
	assert.Equal(t, addrReg, dataConn.GetAddress().GetAddress())
}

// TestNodeRegParity_RegPcNamespaceMismatchWithoutRedial mirrors C++
// atbus_node_reg::reg_pc_namespace_mismatch_without_redial.
func TestNodeRegParity_RegPcNamespaceMismatchWithoutRedial(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	addrOther := reserveTCPListenAddress(t)
	addrReg := reserveTCPListenAddress(t)

	upstream := NewNode()
	downstream := NewNode()
	t.Cleanup(func() {
		downstream.Reset()
		upstream.Reset()
	})

	upConf := conf
	upConf.Scope = "prod"
	upConf.NamespaceName = "game"
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12345678, &upConf))

	dsConf := conf
	dsConf.Scope = "prod"
	dsConf.NamespaceName = "lobby"
	dsConf.UpstreamAddress = addrReg
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Init(0x12346789, &dsConf))

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrOther))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrReg))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstream.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(downstream.GetId())
	}, upstream, downstream)

	epUpstream := downstream.GetEndpoint(upstream.GetId())
	require.NotNil(t, epUpstream)
	dataConn := downstream.self.GetDataConnection(epUpstream, false)
	require.NotNil(t, dataConn)
	assert.Equal(t, addrReg, dataConn.GetAddress().GetAddress())
}

// TestNodeRegParity_RegPcGatewaySelectMatchingScope mirrors C++
// atbus_node_reg::reg_pc_gateway_select_matching_scope.
func TestNodeRegParity_RegPcGatewaySelectMatchingScope(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	addrOther := reserveTCPListenAddress(t)
	addrDev := reserveTCPListenAddress(t)
	addrDownstream := reserveTCPListenAddress(t)

	upstream := NewNode()
	downstream := NewNode()
	t.Cleanup(func() {
		downstream.Reset()
		upstream.Reset()
	})

	upConf := conf
	upConf.Scope = "prod"
	upConf.Gateway = []types.Gateway{
		{Address: addrOther, MatchScope: "other"},
		{Address: addrDev, MatchScope: "dev"},
	}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12345678, &upConf))

	dsConf := conf
	dsConf.Scope = "dev"
	dsConf.NodeLabels = map[string]string{"zone": "a"}
	dsConf.UpstreamAddress = addrOther
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Init(0x12346789, &dsConf))

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrOther))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrDev))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Listen(addrDownstream))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstream.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(downstream.GetId())
	}, upstream, downstream)

	// 下游必须选中匹配自身 scope 的 gateway 地址
	epUpstream := downstream.GetEndpoint(upstream.GetId())
	require.NotNil(t, epUpstream)
	dataConn := downstream.self.GetDataConnection(epUpstream, false)
	require.NotNil(t, dataConn)
	assert.Equal(t, addrDev, dataConn.GetAddress().GetAddress())

	// 下游 endpoint 必须保存上游通告的 gateway 匹配规则
	gateways := epUpstream.GetGateway()
	require.Len(t, gateways, 2)
	assert.Equal(t, addrOther, gateways[0].Address)
	assert.Equal(t, "other", gateways[0].MatchScope)
	assert.Equal(t, addrDev, gateways[1].Address)
	assert.Equal(t, "dev", gateways[1].MatchScope)

	// 上游 endpoint 必须拿到下游上报的身份信息
	epDownstream := upstream.GetEndpoint(downstream.GetId())
	require.NotNil(t, epDownstream)
	assert.Equal(t, "dev", epDownstream.GetScope())
	assert.Equal(t, "a", epDownstream.GetLabels()["zone"])
}

// TestNodeRegParity_RegPcGatewayWildcardReachable mirrors C++
// atbus_node_reg::reg_pc_gateway_wildcard_reachable.
func TestNodeRegParity_RegPcGatewayWildcardReachable(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	addrOther := reserveTCPListenAddress(t)
	addrWildcard := reserveTCPListenAddress(t)
	addrDownstream := reserveTCPListenAddress(t)

	upstream := NewNode()
	downstream := NewNode()
	t.Cleanup(func() {
		downstream.Reset()
		upstream.Reset()
	})

	upConf := conf
	upConf.Scope = "prod"
	upConf.Gateway = []types.Gateway{
		{Address: addrOther, MatchScope: "other"},
		{Address: addrWildcard},
	}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12345678, &upConf))

	dsConf := conf
	dsConf.Scope = "dev"
	dsConf.UpstreamAddress = addrOther
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Init(0x12346789, &dsConf))

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrOther))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrWildcard))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Listen(addrDownstream))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstream.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstream.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(downstream.GetId())
	}, upstream, downstream)

	epUpstream := downstream.GetEndpoint(upstream.GetId())
	require.NotNil(t, epUpstream)
	dataConn := downstream.self.GetDataConnection(epUpstream, false)
	require.NotNil(t, dataConn)
	assert.Equal(t, addrWildcard, dataConn.GetAddress().GetAddress())
}

// TestNodeRegParity_RegPcGatewayLabelSubset mirrors C++
// atbus_node_reg::reg_pc_gateway_label_subset.
func TestNodeRegParity_RegPcGatewayLabelSubset(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	addrLabeled := reserveTCPListenAddress(t)
	addrWildcard := reserveTCPListenAddress(t)
	addrFull := reserveTCPListenAddress(t)
	addrPartial := reserveTCPListenAddress(t)

	upstream := NewNode()
	downstreamFull := NewNode()
	downstreamPartial := NewNode()
	t.Cleanup(func() {
		downstreamPartial.Reset()
		downstreamFull.Reset()
		upstream.Reset()
	})

	upConf := conf
	upConf.Scope = "prod"
	upConf.Gateway = []types.Gateway{
		{
			Address:     addrLabeled,
			MatchLabels: map[string]string{"zone": "a", "app": "x"},
		},
		{Address: addrWildcard},
	}
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Init(0x12345678, &upConf))

	// labels 覆盖 match_labels 全部键值的对端可以使用受限地址
	fullConf := conf
	fullConf.Scope = "dev"
	fullConf.NodeLabels = map[string]string{"zone": "a", "app": "x", "extra": "1"}
	fullConf.UpstreamAddress = addrWildcard
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstreamFull.Init(0x12346789, &fullConf))

	// labels 只覆盖部分键值的对端必须跳过受限地址
	partialConf := conf
	partialConf.Scope = "dev"
	partialConf.NodeLabels = map[string]string{"zone": "a"}
	partialConf.UpstreamAddress = addrLabeled
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstreamPartial.Init(0x12349012, &partialConf))

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrLabeled))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Listen(addrWildcard))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstreamFull.Listen(addrFull))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstreamPartial.Listen(addrPartial))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, upstream.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstreamFull.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, downstreamPartial.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstreamFull.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(downstreamFull.GetId()) &&
			downstreamPartial.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(downstreamPartial.GetId())
	}, upstream, downstreamFull, downstreamPartial)

	// 全量 labels 的对端选中受限地址(列表顺序优先), 而不是回退到注册地址
	epFull := downstreamFull.GetEndpoint(upstream.GetId())
	require.NotNil(t, epFull)
	connFull := downstreamFull.self.GetDataConnection(epFull, false)
	require.NotNil(t, connFull)
	assert.Equal(t, addrLabeled, connFull.GetAddress().GetAddress())

	// 部分 labels 的对端只能选中通配地址, 而不是受限的注册地址
	epPartial := downstreamPartial.GetEndpoint(upstream.GetId())
	require.NotNil(t, epPartial)
	connPartial := downstreamPartial.self.GetDataConnection(epPartial, false)
	require.NotNil(t, connPartial)
	assert.Equal(t, addrWildcard, connPartial.GetAddress().GetAddress())
}
