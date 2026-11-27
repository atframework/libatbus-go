// Package libatbus_impl provides internal implementation details for libatbus.
//
// Traceability for C++ atbus_endpoint_test.cpp:
//   - connection_basic / endpoint_basic / get_connection: covered here.
//   - is_child and related topology assertions: covered in atbus_topology_test.go.
//   - address parsing/schema handling: covered in channel/utility/channel_utility_test.go.
package libatbus_impl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

func TestEndpointDataConnectionSkipsPendingConnection(t *testing.T) {
	var n Node
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, n.Init(1, nil))
	t.Cleanup(func() { n.Reset() })
	target := CreateEndpoint(&n, 2, 0, "peer")
	pending := CreateConnection(&n, "ipv4://127.0.0.1:10101", false)
	connected := CreateConnection(&n, "ipv4://127.0.0.1:10102", false)
	pending.setStatus(types.ConnectionState_Connecting)
	connected.setStatus(types.ConnectionState_Connected)
	require.True(t, target.AddConnection(pending, true))
	require.Nil(t, n.self.GetDataConnection(target, false))
	require.True(t, target.AddConnection(connected, true))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, n.AddEndpoint(target))
	require.Same(t, connected, n.self.GetDataConnection(target, false))
}

func TestNodeTopologyRelationReportsNextHop(t *testing.T) {
	var n Node
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, n.Init(0x12345678, nil))
	t.Cleanup(func() { n.Reset() })
	registry := n.GetTopologyRegistry()
	require.True(t, registry.UpdatePeer(n.GetId(), 0, makeTopologyData(1, "parent", nil)))
	for _, id := range []types.BusIdType{0x12340000, 0x1234ffff} {
		require.True(t, registry.UpdatePeer(id, n.GetId(), makeTopologyData(2, "child", nil)))
		rel, via := n.GetTopologyRelation(id)
		require.Equal(t, types.TopologyRelationType_ImmediateDownstream, rel)
		require.NotNil(t, via)
		require.Equal(t, id, via.GetBusId())
	}
	rel, via := n.GetTopologyRelation(n.GetId())
	require.Equal(t, types.TopologyRelationType_Self, rel)
	require.NotNil(t, via)
	require.Equal(t, n.GetId(), via.GetBusId())
	rel, via = n.GetTopologyRelation(0x12350000)
	require.Equal(t, types.TopologyRelationType_Invalid, rel)
	require.Nil(t, via)
}

func TestTopologyPassiveUpstreamPromotedBeforeChildRemoval(t *testing.T) {
	registry := CreateTopologyRegistry()
	require.True(t, registry.UpdatePeer(2, 1, makeTopologyData(2, "child", nil)))
	parent := registry.GetPeer(1)
	require.NotNil(t, parent)
	require.True(t, registry.UpdatePeer(1, 0, makeTopologyData(1, "parent", nil)))
	registry.RemovePeer(2)
	require.Nil(t, registry.GetPeer(2))
	require.Same(t, parent, registry.GetPeer(1))
	require.False(t, parent.ContainsDownstream(2))
}

func TestCreateEndpointInitializesOwnerAndMetadata(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1001, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act
	ep := CreateEndpoint(&n, 0x2001, 3456, "endpoint-host")

	// Assert
	require.NotNil(t, ep)
	assert.Equal(t, types.Node(&n), ep.GetOwner())
	assert.Equal(t, types.BusIdType(0x2001), ep.GetId())
	assert.Equal(t, int32(3456), ep.GetPid())
	assert.Equal(t, "endpoint-host", ep.GetHostname())
}

func TestEndpointAddConnectionBindsAsControlConnection(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1002, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	ep := CreateEndpoint(&n, 0x2002, 4567, "endpoint-host")
	require.NotNil(t, ep)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0", false)
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connected)

	// Act
	added := ep.AddConnection(conn, false)

	// Assert
	assert.True(t, added)
	assert.Equal(t, ep, conn.GetBinding())
	assert.Equal(t, conn, ep.ctrlConn)
}

func TestEndpointGetDataConnectionFallsBackToControlWhenEnabled(t *testing.T) {
	// Arrange
	var n Node
	ret := n.Init(0x1003, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	target := CreateEndpoint(&n, 0x2003, 5678, "endpoint-host")
	require.NotNil(t, target)

	conn := CreateConnection(&n, "ipv4://127.0.0.1:0", false)
	require.NotNil(t, conn)
	conn.setStatus(types.ConnectionState_Connected)
	require.True(t, target.AddConnection(conn, false))

	// Act
	result := n.self.GetDataConnection(target, true)

	// Assert
	assert.Equal(t, conn, result)
}

// TestEndpoint_ReloadScopeLabelsAndGateway mirrors C++ atbus_endpoint::reload_scope_labels_and_gateway.
func TestEndpoint_ReloadScopeLabelsAndGateway(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	var n Node
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, n.Init(0x12345678, &conf))
	t.Cleanup(func() { n.Reset() })

	ep := CreateEndpoint(&n, 0x12345679, int64(n.GetSelfPid()), n.GetSelfHostname())
	require.NotNil(t, ep)

	// 初始为空, 表示不限制
	assert.Empty(t, ep.GetScope())
	assert.Empty(t, ep.GetNamespace())
	assert.Empty(t, ep.GetLabels())
	assert.Empty(t, ep.GetGateway())

	// 本地配置源 reload
	labels := map[string]string{"zone": "a"}
	gw := types.Gateway{
		Address:    "ipv4://127.0.0.1:16450",
		MatchScope: "prod",
	}
	ep.Reload("prod", "game", labels, []types.Gateway{gw})
	assert.Equal(t, "prod", ep.GetScope())
	assert.Equal(t, "game", ep.GetNamespace())
	assert.Equal(t, map[string]string{"zone": "a"}, ep.GetLabels())
	require.Len(t, ep.GetGateway(), 1)
	assert.Equal(t, gw.Address, ep.GetGateway()[0].Address)
	assert.Equal(t, gw.MatchScope, ep.GetGateway()[0].MatchScope)

	// 注册包数据源 reload (线上协议字段)
	reg := &protocol.RegisterData{
		Scope:         "dev",
		NamespaceName: "lobby",
		Labels:        map[string]string{"zone": "b"},
		Channels: []*protocol.ChannelData{
			{
				Address:         "ipv4://127.0.0.1:16451",
				MatchScope:      "dev",
				MatchHosts:      []string{"host-a"},
				MatchNamespaces: []string{"lobby"},
				MatchLabels:     map[string]string{"zone": "b"},
			},
		},
	}
	ep.ReloadFromProtocol(reg.GetScope(), reg.GetNamespaceName(), reg.GetLabels(), reg.GetChannels())
	assert.Equal(t, "dev", ep.GetScope())
	assert.Equal(t, "lobby", ep.GetNamespace())
	assert.Equal(t, map[string]string{"zone": "b"}, ep.GetLabels())
	gateways := ep.GetGateway()
	require.Len(t, gateways, 1)
	assert.Equal(t, "ipv4://127.0.0.1:16451", gateways[0].Address)
	assert.Equal(t, "dev", gateways[0].MatchScope)
	_, hasHost := gateways[0].MatchHosts["host-a"]
	assert.True(t, hasHost)
	_, hasNS := gateways[0].MatchNamespaces["lobby"]
	assert.True(t, hasNS)
	assert.Equal(t, "b", gateways[0].MatchLabels["zone"])
}
