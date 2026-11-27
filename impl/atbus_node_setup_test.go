// Package libatbus_impl provides internal implementation details for libatbus.
//
// This file contains Go equivalents of atbus_node_setup_test.cpp:
//   - override_listen_path
//   - crypto_algorithms
//   - compression_algorithms

package libatbus_impl

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

// TestNodeSetupParity_OverrideListenPath mirrors C++ atbus_node_setup::override_listen_path.
// On Unix it tests that overwrite_listen_path controls whether a second node
// can take over a Unix-socket listen path. On Windows, Unix sockets may not be
// fully supported so we skip.
func TestNodeSetupParity_OverrideListenPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix socket listen-path override test is not supported on Windows")
	}

	sockPath := "/tmp/atbus-go-unit-test-overwrite-unix.sock"
	t.Cleanup(func() {
		// Best-effort cleanup of leftover lock and socket files.
		// The node Reset/Close should handle this, but be defensive.
		os.Remove(sockPath)
		os.Remove(sockPath + ".lock")
	})

	// -- node1: overwrite_listen_path = false, listen on sockPath -- should succeed
	var conf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf1)
	conf1.OverwriteListenPath = false

	var node1 Node
	ret := node1.Init(0x12345678, &conf1)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node1.Listen("unix://" + sockPath)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"node1 should successfully listen on the socket path")

	// -- node2: overwrite_listen_path = false, same path -- should fail (path locked)
	var conf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf2)
	conf2.OverwriteListenPath = false

	var node2 Node
	ret = node2.Init(0x12356789, &conf2)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node2.Listen("unix://" + sockPath)
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"node2 should fail to listen on a locked path")

	// -- node3: overwrite_listen_path = true, same path -- should succeed
	var conf3 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf3)
	conf3.OverwriteListenPath = true

	var node3 Node
	ret = node3.Init(0x12367890, &conf3)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node3.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node3.Listen("unix://" + sockPath)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret,
		"node3 with overwrite_listen_path=true should take over the socket path")

	// Cleanup nodes
	node1.Reset()
	node2.Reset()
	node3.Reset()
}

// TestNodeSetupParity_CryptoAlgorithms mirrors C++ atbus_node_setup::crypto_algorithms.
// Besides metadata checks, it now also verifies the C++-style parse helper mapping.
func TestNodeSetupParity_CryptoAlgorithms(t *testing.T) {
	algorithms := []struct {
		name      string
		parseName string
		enumVal   protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		keySize   int
		ivSize    int
		isAEAD    bool
		tagSize   int
	}{
		{"XXTEA", "xxtea", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XXTEA, 16, 0, false, 0},
		{"CHACHA20", "chacha20", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20, 32, 16, false, 0},
		{"CHACHA20-POLY1305", "chacha20-poly1305-ietf", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF, 32, 12, true, 16},
		{"XCHACHA20-POLY1305", "xchacha20-poly1305-ietf", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_XCHACHA20_POLY1305_IETF, 32, 24, true, 16},
		{"AES-128-CBC", "aes-128-cbc", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_CBC, 16, 16, false, 0},
		{"AES-128-GCM", "aes-128-gcm", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_128_GCM, 16, 12, true, 16},
		{"AES-192-CBC", "aes-192-cbc", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_CBC, 24, 16, false, 0},
		{"AES-192-GCM", "aes-192-gcm", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_192_GCM, 24, 12, true, 16},
		{"AES-256-CBC", "aes-256-cbc", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_CBC, 32, 16, false, 0},
		{"AES-256-GCM", "aes-256-gcm", protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM, 32, 12, true, 16},
	}

	count := 0
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			// Verify C++-style helper parity.
			parsed := types.ParseCryptoAlgorithmName(algo.parseName)
			assert.Equal(t, algo.enumVal, parsed)

			// Verify string representation
			str := cryptoAlgorithmString(algo.enumVal)
			assert.NotEmpty(t, str)
			assert.NotEqual(t, "NONE", str, "Algorithm %s should have a non-NONE string representation", algo.name)

			// Verify key size
			assert.Equal(t, algo.keySize, cryptoAlgorithmKeySize(algo.enumVal),
				"Key size mismatch for %s", algo.name)

			// Verify IV size
			assert.Equal(t, algo.ivSize, cryptoAlgorithmIVSize(algo.enumVal),
				"IV size mismatch for %s", algo.name)

			// Verify AEAD property
			assert.Equal(t, algo.isAEAD, cryptoAlgorithmIsAEAD(algo.enumVal),
				"AEAD property mismatch for %s", algo.name)

			// Verify tag size
			assert.Equal(t, algo.tagSize, cryptoAlgorithmTagSize(algo.enumVal),
				"Tag size mismatch for %s", algo.name)
		})
		count++
	}

	assert.Greater(t, count, 0, "At least one crypto algorithm must be tested")
}

// TestNodeSetupParity_CompressionAlgorithms mirrors C++ atbus_node_setup::compression_algorithms.
// Besides metadata checks, it now also verifies the C++-style parse helper mapping.
func TestNodeSetupParity_CompressionAlgorithms(t *testing.T) {
	algorithms := []struct {
		name      string
		parseName string
		enumVal   protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE
	}{
		{"ZSTD", "zstd", protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZSTD},
		{"LZ4", "lz4", protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_LZ4},
		{"SNAPPY", "snappy", protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_SNAPPY},
		{"ZLIB", "zlib", protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_ZLIB},
	}

	count := 0
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			// Verify C++-style helper parity.
			parsed := types.ParseCompressionAlgorithmName(algo.parseName)
			assert.Equal(t, algo.enumVal, parsed)

			// Verify string representation
			str := compressionAlgorithmString(algo.enumVal)
			assert.NotEmpty(t, str)
			assert.NotEqual(t, "NONE", str, "Compression %s should have a non-NONE string representation", algo.name)

			// Verify that the compression algorithm is recognized by the support check delegate
			assert.True(t, types.IsCompressionAlgorithmSupported(algo.enumVal),
				"Compression algorithm %s should be supported", algo.name)
		})
		count++
	}

	assert.Greater(t, count, 0, "At least one compression algorithm must be tested")

	assert.Equal(t,
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
		types.ParseCompressionAlgorithmName("gzip"),
	)

	// Mirrors C++ connection_context::is_compression_algorithm_supported(NONE) == false.
	assert.False(t, types.IsCompressionAlgorithmSupported(
		protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE_ATBUS_COMPRESSION_ALGORITHM_NONE,
	))
}

// TestNodeSetupParity_KeyExchangeAlgorithms verifies the key exchange algorithm
// string mapping and curve availability. This extends the C++ crypto_algorithms
// test coverage to key exchange types.
func TestNodeSetupParity_KeyExchangeAlgorithms(t *testing.T) {
	algorithms := []struct {
		name    string
		enumVal protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE
	}{
		{"X25519", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519},
		{"SECP256R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1},
		{"SECP384R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1},
		{"SECP521R1", protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1},
	}

	count := 0
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			str := keyExchangeString(algo.enumVal)
			assert.NotEmpty(t, str)
			assert.NotEqual(t, "NONE", str)

			curve := keyExchangeCurve(algo.enumVal)
			assert.NotNil(t, curve, "Key exchange %s should map to a valid ECDH curve", algo.name)
		})
		count++
	}

	assert.Greater(t, count, 0)

	// NONE should not map to any curve
	assert.Nil(t, keyExchangeCurve(protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE))
}

// TestNodeSetupParity_ReloadCryptoAppliesConfig verifies ReloadCrypto correctly
// applies key exchange and allowed algorithm configuration.
func TestNodeSetupParity_ReloadCryptoAppliesConfig(t *testing.T) {
	var n Node
	ret := n.Init(0x1234, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Default should be NONE
	assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE,
		n.GetCryptoKeyExchangeType())

	// Reload with X25519
	algs := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_AES_256_GCM,
		protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_CHACHA20_POLY1305_IETF,
	}
	ret = n.ReloadCrypto(
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		0,
		algs,
	)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_X25519,
		n.GetCryptoKeyExchangeType())

	// Reload with invalid type should fall back to NONE
	ret = n.ReloadCrypto(
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE(9999),
		0,
		algs,
	)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE,
		n.GetCryptoKeyExchangeType())
}

// TestNodeSetupParity_ResetConnectionAfterLoopbackRedirect mirrors C++
// atbus_node_setup::reset_connection_after_loopback_redirect.
func TestNodeSetupParity_ResetConnectionAfterLoopbackRedirect(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Init(0x12345678, &conf))
	t.Cleanup(func() { node.Reset() })

	unusedAddr := strings.Replace(reserveTCPListenAddress(t), "ipv4://127.0.0.1:", "atcp://0.0.0.0:", 1)
	conn := CreateConnection(node, unusedAddr, false)
	require.NotNil(t, conn)
	assert.Equal(t, unusedAddr, conn.GetOriginAddress())
	assert.Equal(t, "127.0.0.1", conn.GetAddress().GetHost())
	assert.Equal(t, 1, node.GetConnectionTimerSize())

	_ = conn.Connect()
	conn.Reset()
	assert.Equal(t, 0, node.GetConnectionTimerSize())
}

// TestNodeSetupParity_ResetNodeWithPendingLoopbackRedirectConnection mirrors C++
// atbus_node_setup::reset_node_with_pending_loopback_redirect_connection.
func TestNodeSetupParity_ResetNodeWithPendingLoopbackRedirectConnection(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Init(0x12345678, &conf))

	unusedAddr := strings.Replace(reserveTCPListenAddress(t), "ipv4://127.0.0.1:", "atcp://0.0.0.0:", 1)
	conn := CreateConnection(node, unusedAddr, false)
	require.NotNil(t, conn)
	assert.Equal(t, 1, node.GetConnectionTimerSize())

	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Reset())
	assert.Equal(t, 0, node.GetConnectionTimerSize())
}

// TestNodeSetupParity_ReconnectAfterFailedLoopbackRedirectConnect mirrors C++
// atbus_node_setup::reconnect_after_failed_loopback_redirect_connect.
func TestNodeSetupParity_ReconnectAfterFailedLoopbackRedirectConnect(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Init(0x12345678, &conf))
	t.Cleanup(func() { node.Reset() })

	unusedAddr := strings.Replace(reserveTCPListenAddress(t), "ipv4://127.0.0.1:", "atcp://0.0.0.0:", 1)
	conn := CreateConnection(node, unusedAddr, false)
	require.NotNil(t, conn)

	// Go dials synchronously: connecting to an unlistened port fails immediately and cleans up the timer.
	ret := conn.Connect()
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, types.ConnectionState_Disconnected, conn.GetStatus())
	assert.Equal(t, 0, node.GetConnectionTimerSize())

	// Subsequent node.Connect must not be swallowed by a stale connecting_list_by_channel entry:
	// it must actually attempt a dial (returning connect error) and leave timer list empty.
	ret = node.Connect(unusedAddr)
	assert.NotEqual(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, 0, node.GetConnectionTimerSize())
}

// TestNodeSetupParity_CreateConnectionWithDuplicatedAddress mirrors C++
// atbus_node_setup::create_connection_with_duplicated_address.
func TestNodeSetupParity_CreateConnectionWithDuplicatedAddress(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Init(0x12345678, &conf))
	t.Cleanup(func() { node.Reset() })

	addr := reserveTCPListenAddress(t)
	wildcardAddr := strings.Replace(addr, "ipv4://127.0.0.1:", "ipv4://0.0.0.0:", 1)
	conn1 := CreateConnection(node, wildcardAddr, false)
	require.NotNil(t, conn1)
	assert.Equal(t, 1, node.GetConnectionTimerSize())

	conn2 := CreateConnection(node, wildcardAddr, false)
	require.NotNil(t, conn2)
	assert.Equal(t, 2, node.GetConnectionTimerSize())

	// 已有同地址未完成连接时, node.Connect 去重, 不再发起新连接
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Connect(wildcardAddr))
	assert.Equal(t, 2, node.GetConnectionTimerSize())

	// 两个连接必须能互不影响地移除
	conn1.Reset()
	assert.Equal(t, 1, node.GetConnectionTimerSize())

	conn2.Reset()
	assert.Equal(t, 0, node.GetConnectionTimerSize())
}

// TestNodeSetupParity_UpstreamTopologyWithWildcardUpstreamAddress mirrors C++
// atbus_node_setup::upstream_topology_with_wildcard_upstream_address.
func TestNodeSetupParity_UpstreamTopologyWithWildcardUpstreamAddress(t *testing.T) {
	listenAddr := reserveTCPListenAddress(t)
	wildcardAddr := strings.Replace(listenAddr, "ipv4://127.0.0.1:", "ipv4://0.0.0.0:", 1)

	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	node1 := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Init(0x12345678, &conf))
	t.Cleanup(func() { node1.Reset() })

	var confUpstream types.NodeConfigure
	types.SetDefaultNodeConfigure(&confUpstream)
	confUpstream.UpstreamAddress = wildcardAddr
	node2 := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.Init(0x12356789, &confUpstream))
	t.Cleanup(func() { node2.Reset() })

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Listen(listenAddr))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.Start())

	waitForNodeCondition(t, 8*time.Second, func() bool {
		upstream := node2.GetUpstreamEndpoint()
		return upstream != nil && upstream.GetId() == node1.GetId() &&
			node2.GetSelfEndpoint().GetCtrlConnection(upstream) != nil
	}, node1, node2)

	upstream := node2.GetUpstreamEndpoint()
	require.NotNil(t, upstream)
	conn := node2.GetSelfEndpoint().GetCtrlConnection(upstream)
	require.NotNil(t, conn)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.Reset())
	assert.Equal(t, types.ConnectionState_Disconnected, conn.GetStatus())
	assert.Nil(t, conn.GetBinding())
}

// TestNodeSetupParity_GatewayConfCopyAndDefault mirrors C++
// atbus_node_setup::gateway_conf_copy_and_default.
func TestNodeSetupParity_GatewayConfCopyAndDefault(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	assert.Equal(t, 60*time.Second, conf.MaxRetryInterval)

	gw := types.Gateway{
		Address:         "ipv4://127.0.0.1:16450",
		MatchScope:      "prod",
		MatchHosts:      map[string]struct{}{"host-a": {}},
		MatchNamespaces: map[string]struct{}{"game": {}},
		MatchLabels:     map[string]string{"zone": "a"},
	}
	conf.Scope = "prod"
	conf.NamespaceName = "game"
	conf.NodeLabels = map[string]string{"zone": "a"}
	conf.Gateway = []types.Gateway{gw}

	node := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Init(0x12345678, &conf))
	t.Cleanup(func() { node.Reset() })

	// Mutating source conf must not affect initialized node copy
	conf.Gateway[0].Address = "ipv4://127.0.0.1:19999"
	conf.Gateway[0].MatchLabels["zone"] = "mutated"
	conf.NodeLabels["zone"] = "mutated"

	nodeGateways := node.GetGatewayAddress()
	require.Len(t, nodeGateways, 1)
	assert.Equal(t, "ipv4://127.0.0.1:16450", nodeGateways[0].Address)
	assert.Equal(t, "prod", nodeGateways[0].MatchScope)
	assert.Equal(t, map[string]struct{}{"host-a": {}}, nodeGateways[0].MatchHosts)
	assert.Equal(t, map[string]struct{}{"game": {}}, nodeGateways[0].MatchNamespaces)
	assert.Equal(t, map[string]string{"zone": "a"}, nodeGateways[0].MatchLabels)
	assert.Equal(t, "a", node.GetConfigure().NodeLabels["zone"])

	// 复用 NodeConfigure 时 SetDefaultNodeConfigure 必须清掉旧的隔离配置
	types.SetDefaultNodeConfigure(&conf)
	assert.Empty(t, conf.Scope)
	assert.Empty(t, conf.NamespaceName)
	assert.Empty(t, conf.NodeLabels)
	assert.Empty(t, conf.Gateway)
	assert.Equal(t, 60*time.Second, conf.MaxRetryInterval)
}

// TestNodeSetupParity_GatewayChannelDataRoundtrip mirrors C++
// atbus_node_setup::gateway_channel_data_roundtrip.
func TestNodeSetupParity_GatewayChannelDataRoundtrip(t *testing.T) {
	gw := types.Gateway{
		Address:         "ipv4://127.0.0.1:16450",
		MatchScope:      "prod",
		MatchHosts:      map[string]struct{}{"host-a": {}, "host-b": {}},
		MatchNamespaces: map[string]struct{}{"game": {}},
		MatchLabels:     map[string]string{"zone": "a", "app": "x"},
	}

	chanData := &protocol.ChannelData{}
	types.DumpGatewayToChannelData(gw, chanData)
	assert.Equal(t, gw.Address, chanData.GetAddress())
	assert.Equal(t, gw.MatchScope, chanData.GetMatchScope())
	assert.Len(t, chanData.GetMatchHosts(), 2)
	assert.Len(t, chanData.GetMatchNamespaces(), 1)
	assert.Len(t, chanData.GetMatchLabels(), 2)

	restored := types.BuildGatewayFromChannelData(chanData)
	assert.Equal(t, gw.Address, restored.Address)
	assert.Equal(t, gw.MatchScope, restored.MatchScope)
	assert.Equal(t, gw.MatchHosts, restored.MatchHosts)
	assert.Equal(t, gw.MatchNamespaces, restored.MatchNamespaces)
	assert.Equal(t, gw.MatchLabels, restored.MatchLabels)

	// 未配置任何匹配规则的地址对所有对端可达
	wildcardChan := &protocol.ChannelData{Address: "ipv4://127.0.0.1:16451"}
	wildcardGw := types.BuildGatewayFromChannelData(wildcardChan)
	assert.Equal(t, "ipv4://127.0.0.1:16451", wildcardGw.Address)
	assert.Empty(t, wildcardGw.MatchScope)
	assert.Empty(t, wildcardGw.MatchHosts)
	assert.Empty(t, wildcardGw.MatchNamespaces)
	assert.Empty(t, wildcardGw.MatchLabels)
}

// TestNodeSetupParity_DumpListenToChannelData mirrors C++
// atbus_node_setup::dump_listen_to_channel_data.
func TestNodeSetupParity_DumpListenToChannelData(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.Scope = "prod"
	conf.NamespaceName = "game"

	node := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Init(0x12345678, &conf))
	t.Cleanup(func() { node.Reset() })

	chanData := &protocol.ChannelData{}
	node.DumpListenToChannelData("ipv4://127.0.0.1:16450", chanData)
	assert.Equal(t, "ipv4://127.0.0.1:16450", chanData.GetAddress())
	assert.Equal(t, "prod", chanData.GetMatchScope())
	assert.Equal(t, []string{"game"}, chanData.GetMatchNamespaces())
	assert.Empty(t, chanData.GetMatchHosts())
	assert.Empty(t, chanData.GetMatchLabels())

	var wildcardConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&wildcardConf)
	wildcardNode := NewNode()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, wildcardNode.Init(0x12345679, &wildcardConf))
	t.Cleanup(func() { wildcardNode.Reset() })

	wildcardChan := &protocol.ChannelData{}
	wildcardNode.DumpListenToChannelData("ipv4://127.0.0.1:16450", wildcardChan)
	assert.Equal(t, "ipv4://127.0.0.1:16450", wildcardChan.GetAddress())
	assert.Empty(t, wildcardChan.GetMatchScope())
	assert.Empty(t, wildcardChan.GetMatchNamespaces())
}

// TestNodeSetupParity_MessageSequenceAllocatorAndSelfIdentity verifies
// C++ node constructor message_sequence_allocator_ offset and per-node self hostname/pid overrides.
func TestNodeSetupParity_MessageSequenceAllocatorAndSelfIdentity(t *testing.T) {
	node := NewNode()
	seq1 := node.AllocateMessageSequence()
	seq2 := node.AllocateMessageSequence()
	assert.Greater(t, seq1, uint64(0))
	assert.Equal(t, seq1+1, seq2)

	// Per-node identity falls back to process global defaults until overridden
	assert.Equal(t, node.GetHostname(), node.GetSelfHostname())
	assert.Equal(t, int32(node.GetPid()), node.GetSelfPid())

	node.SetSelfHostname("custom-node-host")
	node.SetSelfPid(42424)
	assert.Equal(t, "custom-node-host", node.GetSelfHostname())
	assert.Equal(t, int32(42424), node.GetSelfPid())

	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node.Init(0x12345678, nil))
	t.Cleanup(func() { node.Reset() })
	assert.Equal(t, "custom-node-host", node.GetSelfEndpoint().GetHostname())
	assert.Equal(t, int32(42424), node.GetSelfEndpoint().GetPid())
}
