// Package libatbus_impl provides internal implementation details for libatbus.
//
// This file contains Go equivalents of additional C++ atbus_node_msg_test.cpp cases:
//   - topology_registry_multi_level_route
//   - topology_registry_multi_level_route_reverse
//   - transfer_failed
//   - transfer_failed_cross_upstreams
//   - crypto_config_key_exchange_algorithms
//   - crypto_config_cipher_algorithms
//   - crypto_config_comprehensive_matrix
//   - crypto_config_multiple_algorithms
//   - crypto_config_upstream_downstream
//   - crypto_config_disabled
//   - crypto_list_available_algorithms

package libatbus_impl

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

// ============ Multi-Level Topology Routing Tests ============

// TestNodeMsgParity_TopologyRegistryMultiLevelRoute mirrors C++ atbus_node_msg::topology_registry_multi_level_route.
// Creates a 3-node chain (upstream → mid → downstream) and verifies that a message
// from upstream can reach downstream via multi-hop routing through the mid node.
func TestNodeMsgParity_TopologyRegistryMultiLevelRoute(t *testing.T) {
	// Upstream node
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	var upstream Node
	ret := upstream.Init(0x12345678, &upstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	upstreamAddr := reserveTCPListenAddress(t)
	ret = upstream.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstream.Reset() })

	// Mid node (downstream of upstream)
	var midConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&midConf)
	midConf.UpstreamAddress = upstreamAddr

	var mid Node
	ret = mid.Init(0x12346789, &midConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	midAddr := reserveTCPListenAddress(t)
	ret = mid.Listen(midAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = mid.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { mid.Reset() })

	// Downstream node (downstream of mid)
	var dsConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf)
	dsConf.UpstreamAddress = midAddr

	var downstream Node
	ret = downstream.Init(0x12346890, &dsConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	dsAddr := reserveTCPListenAddress(t)
	ret = downstream.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = downstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstream.Reset() })

	// Wait for connections: mid↔upstream, downstream↔mid
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return mid.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(mid.GetId()) &&
			downstream.IsEndpointAvailable(mid.GetId()) &&
			mid.IsEndpointAvailable(downstream.GetId())
	}, &upstream, &mid, &downstream)

	// upstream should NOT have direct endpoint to downstream
	assert.Nil(t, upstream.GetEndpoint(downstream.GetId()))
	assert.Nil(t, downstream.GetEndpoint(upstream.GetId()))

	// Before topology update, upstream should not know how to route to downstream
	relation, nextHop := upstream.GetTopologyRelation(downstream.GetId())
	assert.Equal(t, types.TopologyRelationType_Invalid, relation)
	assert.Nil(t, nextHop)

	// Register topology information
	upstream.GetTopologyRegistry().UpdatePeer(mid.GetId(), upstream.GetId(), nil)
	upstream.GetTopologyRegistry().UpdatePeer(downstream.GetId(), mid.GetId(), nil)

	mid.GetTopologyRegistry().UpdatePeer(downstream.GetId(), mid.GetId(), nil)

	assert.True(t, downstream.GetTopologyRegistry().UpdatePeer(mid.GetId(), upstream.GetId(), nil))
	assert.False(t, downstream.GetTopologyRegistry().UpdatePeer(upstream.GetId(), mid.GetId(), nil))

	// After topology update, upstream should know route to downstream via mid
	relation, nextHop = upstream.GetTopologyRelation(downstream.GetId())
	assert.Equal(t, types.TopologyRelationType_TransitiveDownstream, relation)
	require.NotNil(t, nextHop)
	assert.Equal(t, mid.GetId(), nextHop.GetBusId())

	// downstream → upstream should be TransitiveUpstream via mid
	relation, nextHop = downstream.GetTopologyRelation(upstream.GetId())
	assert.Equal(t, types.TopologyRelationType_TransitiveUpstream, relation)
	require.NotNil(t, nextHop)
	assert.Equal(t, mid.GetId(), nextHop.GetBusId())

	// Send data from upstream to downstream and verify it arrives
	receivedCount := 0
	var lastReceivedData []byte
	downstream.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		receivedCount++
		lastReceivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	sendData := []byte("topology multi-level route\n")
	oldCount := receivedCount
	ret = upstream.SendData(downstream.GetId(), 0, sendData)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return receivedCount > oldCount
	}, &upstream, &mid, &downstream)

	assert.Equal(t, sendData, lastReceivedData)
}

// TestNodeMsgParity_TopologyRegistryMultiLevelRouteReverse mirrors C++ atbus_node_msg::topology_registry_multi_level_route_reverse.
// Creates a 3-node chain (upstream → mid → downstream) and verifies that a message
// from downstream can reach upstream via multi-hop routing.
func TestNodeMsgParity_TopologyRegistryMultiLevelRouteReverse(t *testing.T) {
	// Upstream node
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	var upstream Node
	ret := upstream.Init(0x12345678, &upstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	upstreamAddr := reserveTCPListenAddress(t)
	ret = upstream.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstream.Reset() })

	// Mid node (downstream of upstream)
	var midConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&midConf)
	midConf.UpstreamAddress = upstreamAddr

	var mid Node
	ret = mid.Init(0x12346789, &midConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	midAddr := reserveTCPListenAddress(t)
	ret = mid.Listen(midAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = mid.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { mid.Reset() })

	// Downstream node (downstream of mid)
	var dsConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf)
	dsConf.UpstreamAddress = midAddr

	var downstream Node
	ret = downstream.Init(0x12346890, &dsConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	dsAddr := reserveTCPListenAddress(t)
	ret = downstream.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = downstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstream.Reset() })

	// Wait for connections
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return mid.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(mid.GetId()) &&
			downstream.IsEndpointAvailable(mid.GetId()) &&
			mid.IsEndpointAvailable(downstream.GetId())
	}, &upstream, &mid, &downstream)

	// Register topology
	upstream.GetTopologyRegistry().UpdatePeer(mid.GetId(), upstream.GetId(), nil)
	upstream.GetTopologyRegistry().UpdatePeer(downstream.GetId(), mid.GetId(), nil)
	mid.GetTopologyRegistry().UpdatePeer(downstream.GetId(), mid.GetId(), nil)
	downstream.GetTopologyRegistry().UpdatePeer(mid.GetId(), upstream.GetId(), nil)

	// Send data from downstream → upstream (reverse direction)
	receivedCount := 0
	var lastReceivedData []byte
	upstream.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		receivedCount++
		lastReceivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	sendData := []byte("topology multi-level route reverse\n")
	ret = downstream.SendData(upstream.GetId(), 0, sendData)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return receivedCount > 0 && len(lastReceivedData) > 0
	}, &upstream, &mid, &downstream)

	assert.Equal(t, sendData, lastReceivedData)
}

// ============ Transfer Failure Tests ============

// TestNodeMsgParity_TransferFailed mirrors C++ atbus_node_msg::transfer_failed.
// downstream sends to a non-existent peer (registered in topology but no real node).
// The upstream should return a forward_response with failure.
func TestNodeMsgParity_TransferFailed(t *testing.T) {
	// Upstream node
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	var upstream Node
	ret := upstream.Init(0x12345678, &upstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	upstreamAddr := reserveTCPListenAddress(t)
	ret = upstream.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstream.Reset() })

	// Downstream 1
	var dsConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf)
	dsConf.UpstreamAddress = upstreamAddr

	var ds1 Node
	ret = ds1.Init(0x12346789, &dsConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ds1Addr := reserveTCPListenAddress(t)
	ret = ds1.Listen(ds1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = ds1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { ds1.Reset() })

	ds1.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Wait for registration
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return ds1.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(ds1.GetId())
	}, &upstream, &ds1)

	// Register topology: fake peers that don't actually exist
	for _, n := range []*Node{&upstream, &ds1} {
		n.GetTopologyRegistry().UpdatePeer(ds1.GetId(), upstream.GetId(), nil)
		n.GetTopologyRegistry().UpdatePeer(0x12346890, upstream.GetId(), nil)  // non-existent downstream
		n.GetTopologyRegistry().UpdatePeer(0x12356789, 0, nil)                // non-existent brother
	}

	// Track forward response failures
	failedCount := 0
	var lastStatus error_code.ErrorType
	ds1.SetEventHandleOnForwardResponse(func(_ types.Node, _ types.Endpoint, _ types.Connection, msg *types.Message) error_code.ErrorType {
		failedCount++
		if msg != nil && msg.GetHead() != nil {
			lastStatus = error_code.ErrorType(msg.GetHead().GetResultCode())
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	sendData := []byte("transfer through upstream\n")

	beforeFailed := failedCount

	// Send to non-existent downstream (0x12346890) — should trigger forward response failure
	ret = ds1.SendData(0x12346890, 0, sendData)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Send to non-existent brother (0x12356789) — should also trigger forward response failure
	ret = ds1.SendData(0x12356789, 0, sendData)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Wait for failure responses
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return failedCount >= beforeFailed+2
	}, &upstream, &ds1)

	assert.Equal(t, beforeFailed+2, failedCount)
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, lastStatus)
}

// TestNodeMsgParity_TransferFailedCrossUpstreams mirrors C++ atbus_node_msg::transfer_failed_cross_upstreams.
// Topology:
//
//	F1 <-----> F2
//	/            -(no connection to C2)
//	C1             C2
//
// C1 sends to C2 (registered under F2). Since F2 has no connection to C2,
// the transfer fails. After multiple failures, the F1-C1 connection should survive.
func TestNodeMsgParity_TransferFailedCrossUpstreams(t *testing.T) {
	// F1 (upstream 1)
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.FaultTolerant = 2

	var f1 Node
	ret := f1.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	f1Addr := reserveTCPListenAddress(t)
	ret = f1.Listen(f1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = f1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { f1.Reset() })

	// F2 (upstream 2)
	var f2 Node
	ret = f2.Init(0x12356789, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	f2Addr := reserveTCPListenAddress(t)
	ret = f2.Listen(f2Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = f2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { f2.Reset() })

	// C1 (downstream of F1)
	var c1Conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&c1Conf)
	c1Conf.FaultTolerant = 2
	c1Conf.UpstreamAddress = f1Addr

	var c1 Node
	ret = c1.Init(0x12346789, &c1Conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	c1Addr := reserveTCPListenAddress(t)
	ret = c1.Listen(c1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = c1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { c1.Reset() })

	// Connect F1 → F2 as peers
	f1.Connect(f2Addr)

	// Track endpoint removal
	removeEndpointCount := 0
	removeHandler := func(_ types.Node, _ types.Endpoint, _ error_code.ErrorType) error_code.ErrorType {
		removeEndpointCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	f1.SetEventHandleOnRemoveEndpoint(removeHandler)
	f2.SetEventHandleOnRemoveEndpoint(removeHandler)
	c1.SetEventHandleOnRemoveEndpoint(removeHandler)

	// Wait for all connections: C1↔F1, F1↔F2
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return c1.IsEndpointAvailable(f1.GetId()) &&
			f1.IsEndpointAvailable(c1.GetId()) &&
			f1.IsEndpointAvailable(f2.GetId()) &&
			f2.IsEndpointAvailable(f1.GetId())
	}, &f1, &f2, &c1)

	// Register topology: C1 under F1, fake C2 (0x12356666) under F2
	for _, n := range []*Node{&f1, &f2, &c1} {
		n.GetTopologyRegistry().UpdatePeer(c1.GetId(), f1.GetId(), nil)
		n.GetTopologyRegistry().UpdatePeer(0x12356666, f2.GetId(), nil) // non-existent C2
	}

	// Track forward response failures
	failedCount := 0
	c1.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	c1.SetEventHandleOnForwardResponse(func(_ types.Node, _ types.Endpoint, _ types.Connection, msg *types.Message) error_code.ErrorType {
		failedCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	beforeRemove := removeEndpointCount

	tryTimes := 5
	sendData := []byte("transfer through upstream\n")

	for i := 0; i < tryTimes; i++ {
		beforeCount := failedCount
		sendRet := c1.SendData(0x12356666, 0, sendData)
		if sendRet != error_code.EN_ATBUS_ERR_SUCCESS {
			continue
		}

		waitForNodeCondition(t, 8*time.Second, func() bool {
			return failedCount > beforeCount
		}, &f1, &f2, &c1)
	}

	// After multiple failures, the C1-F1 connection should still be alive
	assert.Equal(t, beforeRemove, removeEndpointCount,
		"no endpoints should be removed due to transfer failures")
	assert.True(t, c1.IsEndpointAvailable(f1.GetId()),
		"C1 to F1 connection should survive transfer failures")
	assert.True(t, f1.IsEndpointAvailable(c1.GetId()),
		"F1 to C1 connection should survive transfer failures")
}

// ============ Crypto Configuration Integration Tests ============

// allKeyExchangeAlgorithms returns all key exchange algorithms to test.
func allKeyExchangeAlgorithms() []struct {
	name    string
	enumVal protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
} {
	return []struct {
		name    string
		enumVal protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
	}{
		{"X25519", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519},
		{"SECP256R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1},
		{"SECP384R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1},
		{"SECP521R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1},
	}
}

// allCipherAlgorithms returns all cipher algorithms to test.
func allCipherAlgorithms() []struct {
	name    string
	enumVal protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
} {
	return []struct {
		name    string
		enumVal protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
	}{
		{"XXTEA", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA},
		{"AES-128-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC},
		{"AES-192-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC},
		{"AES-256-CBC", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC},
		{"AES-128-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM},
		{"AES-192-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM},
		{"AES-256-GCM", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM},
		{"ChaCha20", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20},
		{"ChaCha20-Poly1305-IETF", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF},
		{"XChaCha20-Poly1305-IETF", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF},
	}
}

// testEncryptedMessageBetweenPeerNodes creates two peer nodes with given crypto config,
// connects them, sends a message and verifies encrypted message delivery.
func testEncryptedMessageBetweenPeerNodes(t *testing.T, kex protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE,
	ciphers []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE, testMessage string) bool {
	t.Helper()

	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.CryptoKeyExchangeType = kex
	conf.CryptoAllowAlgorithms = ciphers

	var node1 Node
	ret := node1.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	n1Addr := reserveTCPListenAddress(t)
	ret = node1.Listen(n1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { node1.Reset() })

	var node2 Node
	ret = node2.Init(0x12356789, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	n2Addr := reserveTCPListenAddress(t)
	ret = node2.Listen(n2Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { node2.Reset() })

	// Connect node1 → node2
	node1.Connect(n2Addr)

	// Wait for bidirectional availability
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) &&
			node2.IsEndpointAvailable(node1.GetId())
	}, &node1, &node2)

	// Set up receiver
	receivedCount := 0
	var receivedData []byte
	node2.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		receivedCount++
		receivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	msgBytes := []byte(testMessage)
	initialCount := receivedCount
	sendRet := node1.SendData(node2.GetId(), 0, msgBytes)
	if sendRet != error_code.EN_ATBUS_ERR_SUCCESS {
		t.Logf("  [FAILED] send_data returned error: %d", sendRet)
		return false
	}

	waitForNodeCondition(t, 5*time.Second, func() bool {
		return receivedCount > initialCount && len(receivedData) > 0
	}, &node1, &node2)

	if !assert.Equal(t, msgBytes, receivedData) {
		t.Logf("  [FAILED] Message mismatch")
		return false
	}

	return true
}

// TestNodeMsgParity_CryptoConfigKeyExchangeAlgorithms mirrors C++ atbus_node_msg::crypto_config_key_exchange_algorithms.
// Tests sending encrypted messages with different key exchange algorithms using AES-256-GCM as cipher.
func TestNodeMsgParity_CryptoConfigKeyExchangeAlgorithms(t *testing.T) {
	defaultCipher := protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM

	passedCount := 0
	skippedCount := 0

	for _, kex := range allKeyExchangeAlgorithms() {
		t.Run(kex.name, func(t *testing.T) {
			testMessage := fmt.Sprintf("Encrypted message with %s key exchange!", kex.name)
			success := testEncryptedMessageBetweenPeerNodes(t, kex.enumVal,
				[]protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{defaultCipher}, testMessage)
			if success {
				passedCount++
				t.Logf("[PASS] %s key exchange test passed", kex.name)
			} else {
				t.Errorf("[FAIL] %s key exchange test failed", kex.name)
			}
		})
	}

	_ = skippedCount
	assert.Greater(t, passedCount, 0, "At least one key exchange algorithm should pass")
}

// TestNodeMsgParity_CryptoConfigCipherAlgorithms mirrors C++ atbus_node_msg::crypto_config_cipher_algorithms.
// Tests sending encrypted messages with different cipher algorithms using X25519 as key exchange.
func TestNodeMsgParity_CryptoConfigCipherAlgorithms(t *testing.T) {
	defaultKex := protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519

	passedCount := 0

	for _, cipher := range allCipherAlgorithms() {
		t.Run(cipher.name, func(t *testing.T) {
			testMessage := fmt.Sprintf("Encrypted message with %s cipher!", cipher.name)
			success := testEncryptedMessageBetweenPeerNodes(t, defaultKex,
				[]protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{cipher.enumVal}, testMessage)
			if success {
				passedCount++
				t.Logf("[PASS] %s cipher test passed", cipher.name)
			} else {
				t.Errorf("[FAIL] %s cipher test failed", cipher.name)
			}
		})
	}

	assert.Greater(t, passedCount, 0, "At least one cipher algorithm should pass")
}

// TestNodeMsgParity_CryptoConfigComprehensiveMatrix mirrors C++ atbus_node_msg::crypto_config_comprehensive_matrix.
// Tests all combinations of key exchange × cipher algorithms.
func TestNodeMsgParity_CryptoConfigComprehensiveMatrix(t *testing.T) {
	passedCount := 0
	failedCount := 0

	for _, kex := range allKeyExchangeAlgorithms() {
		for _, cipher := range allCipherAlgorithms() {
			name := fmt.Sprintf("%s+%s", kex.name, cipher.name)
			t.Run(name, func(t *testing.T) {
				testMessage := fmt.Sprintf("Matrix test: %s + %s", kex.name, cipher.name)
				success := testEncryptedMessageBetweenPeerNodes(t, kex.enumVal,
					[]protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{cipher.enumVal}, testMessage)
				if success {
					passedCount++
				} else {
					failedCount++
					t.Errorf("[FAIL] %s", name)
				}
			})
		}
	}

	assert.Greater(t, passedCount, 0, "At least one combination should pass")
	assert.Equal(t, 0, failedCount, "No combination should fail")
}

// TestNodeMsgParity_CryptoConfigMultipleAlgorithms mirrors C++ atbus_node_msg::crypto_config_multiple_algorithms.
// Tests with multiple allowed cipher algorithms configured (algorithm negotiation).
func TestNodeMsgParity_CryptoConfigMultipleAlgorithms(t *testing.T) {
	defaultKex := protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519

	// Configure multiple allowed algorithms in priority order
	allowedAlgorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA,
	}

	t.Logf("[TEST] Testing with %d allowed cipher algorithms", len(allowedAlgorithms))

	testMessage := "Test message with multiple allowed algorithms!"
	success := testEncryptedMessageBetweenPeerNodes(t, defaultKex, allowedAlgorithms, testMessage)
	assert.True(t, success, "Multiple algorithms test should pass")
}

// TestNodeMsgParity_CryptoConfigUpstreamDownstream mirrors C++ atbus_node_msg::crypto_config_upstream_downstream.
// Tests encrypted message delivery in an upstream-downstream topology.
func TestNodeMsgParity_CryptoConfigUpstreamDownstream(t *testing.T) {
	defaultKex := protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519
	defaultCipher := protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM

	// Upstream node with crypto
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)
	upstreamConf.CryptoKeyExchangeType = defaultKex
	upstreamConf.CryptoAllowAlgorithms = []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{defaultCipher}

	var upstream Node
	ret := upstream.Init(0x12345678, &upstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	upstreamAddr := reserveTCPListenAddress(t)
	ret = upstream.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstream.Reset() })

	// Downstream node with same crypto
	var dsConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf)
	dsConf.CryptoKeyExchangeType = defaultKex
	dsConf.CryptoAllowAlgorithms = []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{defaultCipher}
	dsConf.UpstreamAddress = upstreamAddr

	var downstream Node
	ret = downstream.Init(0x12346789, &dsConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	dsAddr := reserveTCPListenAddress(t)
	ret = downstream.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = downstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstream.Reset() })

	// Wait for connection
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return downstream.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(downstream.GetId())
	}, &upstream, &downstream)

	receivedCount := 0
	var lastReceivedData []byte
	handler := func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		receivedCount++
		lastReceivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	downstream.SetEventHandleOnForwardRequest(handler)
	upstream.SetEventHandleOnForwardRequest(handler)

	// Direction 1: Upstream to downstream
	{
		sendData := []byte("Encrypted upstream to downstream message!")
		countBefore := receivedCount
		ret = upstream.SendData(downstream.GetId(), 0, sendData)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

		waitForNodeCondition(t, 3*time.Second, func() bool {
			return receivedCount > countBefore
		}, &upstream, &downstream)

		assert.Equal(t, sendData, lastReceivedData)
		t.Log("[PASS] Upstream to downstream encrypted message")
	}

	// Direction 2: Downstream to upstream
	{
		sendData := []byte("Encrypted downstream to upstream message!")
		countBefore := receivedCount
		ret = downstream.SendData(upstream.GetId(), 0, sendData)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

		waitForNodeCondition(t, 3*time.Second, func() bool {
			return receivedCount > countBefore
		}, &upstream, &downstream)

		assert.Equal(t, sendData, lastReceivedData)
		t.Log("[PASS] Downstream to upstream encrypted message")
	}
}

// TestNodeMsgParity_CryptoConfigDisabled mirrors C++ atbus_node_msg::crypto_config_disabled.
// Tests plaintext message delivery with crypto explicitly disabled.
func TestNodeMsgParity_CryptoConfigDisabled(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.CryptoKeyExchangeType = protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE
	conf.CryptoAllowAlgorithms = nil

	var node1 Node
	ret := node1.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	n1Addr := reserveTCPListenAddress(t)
	ret = node1.Listen(n1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { node1.Reset() })

	var node2 Node
	ret = node2.Init(0x12356789, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	n2Addr := reserveTCPListenAddress(t)
	ret = node2.Listen(n2Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { node2.Reset() })

	node1.Connect(n2Addr)

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) &&
			node2.IsEndpointAvailable(node1.GetId())
	}, &node1, &node2)

	receivedCount := 0
	var receivedData []byte
	node2.SetEventHandleOnForwardRequest(func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		receivedCount++
		receivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	testMessage := []byte("Plain text message without encryption!")
	initialCount := receivedCount
	ret = node1.SendData(node2.GetId(), 0, testMessage)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 5*time.Second, func() bool {
		return receivedCount > initialCount && len(receivedData) > 0
	}, &node1, &node2)

	assert.Equal(t, testMessage, receivedData)
	t.Log("[PASS] No encryption test passed")
}

// TestNodeMsgParity_CryptoListAvailableAlgorithms mirrors C++ atbus_node_msg::crypto_list_available_algorithms.
// Verifies that Go has at least some crypto algorithms available.
func TestNodeMsgParity_CryptoListAvailableAlgorithms(t *testing.T) {
	t.Log("=== Available Crypto Algorithms ===")

	// Key exchange algorithms — Go's crypto/ecdh supports all of these
	kexAlgorithms := allKeyExchangeAlgorithms()
	t.Logf("Key Exchange Algorithms: %d", len(kexAlgorithms))
	for _, kex := range kexAlgorithms {
		curve := keyExchangeCurve(kex.enumVal)
		available := curve != nil
		t.Logf("  - %s: available=%v", kex.name, available)
	}

	// Cipher algorithms
	cipherAlgorithms := allCipherAlgorithms()
	t.Logf("Cipher Algorithms: %d", len(cipherAlgorithms))
	availableCiphers := 0
	for _, cipher := range cipherAlgorithms {
		// All Go cipher implementations are available (built-in crypto packages)
		keySize := cryptoAlgorithmKeySize(cipher.enumVal)
		available := keySize > 0
		if available {
			availableCiphers++
		}
		t.Logf("  - %s: available=%v (keySize=%d)", cipher.name, available, keySize)
	}

	t.Log("==================================")

	// Go supports all key exchange algorithms via crypto/ecdh
	assert.Greater(t, len(kexAlgorithms), 0, "At least some key exchange algorithms should be available")
	assert.Greater(t, availableCiphers, 0, "At least some cipher algorithms should be available")
}
