package libatbus_impl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

func TestNodeConfigureCopyAndNilDefaultHelpersMatchCppRelationshipCase(t *testing.T) {
	// Arrange: populate a non-trivial configuration so the copied value has visible state.
	var conf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf1)
	conf1.UpstreamAddress = "ipv4://127.0.0.1:12001"
	conf1.TopologyLabels = map[string]string{"zone": "test-a", "role": "lobby"}
	conf1.AccessTokens = [][]byte{[]byte("token-a"), []byte("token-b")}
	conf1.LoopTimes = 512

	var startConf types.StartConfigure
	startConf.TimerTimepoint = time.Now()

	// Act: copy the configuration by value, matching the C++ copy constructor coverage.
	conf2 := conf1

	// Assert: the copied value should preserve the original configuration fields.
	assert.Equal(t, conf1, conf2)

	// Act + Assert: nil inputs to default helpers must remain safe, matching the C++ smoke test.
	require.NotPanics(t, func() {
		types.SetDefaultNodeConfigure(nil)
	})
	require.NotPanics(t, func() {
		types.SetDefaultStartConfigure(nil)
	})

	// Act: apply the start defaults to a populated struct.
	types.SetDefaultStartConfigure(&startConf)

	// Assert: start defaults should reset the timer timepoint.
	assert.True(t, startConf.TimerTimepoint.Equal(time.Unix(0, 0)))
}

func TestChildEndpointOperationsMatchCppRelationshipCase(t *testing.T) {
	// Arrange: initialize a node as the owner of all child endpoints.
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act + Assert: insert the first endpoint.
	ep1 := CreateEndpoint(&n, 0x12345679, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep1)
	ret = n.AddEndpoint(ep1)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Len(t, n.GetImmediateEndpointSet(), 1)

	// Act + Assert: insert a second endpoint with a smaller bus id.
	ep2 := CreateEndpoint(&n, 0x12345589, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep2)
	ret = n.AddEndpoint(ep2)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Len(t, n.GetImmediateEndpointSet(), 2)

	// Act + Assert: inserting a duplicate id must not increase the endpoint count.
	beforeSize := len(n.GetImmediateEndpointSet())
	dup := CreateEndpoint(&n, 0x12345679, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, dup)
	ret = n.AddEndpoint(dup)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Len(t, n.GetImmediateEndpointSet(), beforeSize)

	// Act + Assert: insert one more distinct endpoint.
	ep3 := CreateEndpoint(&n, 0x12345680, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep3)
	ret = n.AddEndpoint(ep3)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Len(t, n.GetImmediateEndpointSet(), 3)

	// Act + Assert: removing a missing endpoint should report not found.
	ret = n.RemoveEndpointByID(0x12349999)
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_NOT_FOUND, ret)

	// Act + Assert: removing existing child endpoints should succeed.
	ret = n.RemoveEndpointByID(0x12345589)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.RemoveEndpointByID(0x12345680)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Assert: the immediate endpoint set should only keep the duplicated id entry.
	assert.Len(t, n.GetImmediateEndpointSet(), 1)
	assert.NotNil(t, n.GetEndpoint(0x12345679))
	assert.Nil(t, n.GetEndpoint(0x12345589))
	assert.Nil(t, n.GetEndpoint(0x12345680))
}

func TestImmediateEndpointEventsFireOnAddAndRemove(t *testing.T) {
	// Arrange: initialize a node and install endpoint lifecycle callbacks.
	var n Node
	ret := n.Init(0x22345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	addCalls := 0
	removeCalls := 0
	var addedID types.BusIdType
	var removedID types.BusIdType

	n.SetEventHandleOnAddEndpoint(func(node types.Node, ep types.Endpoint, status error_code.ErrorType) error_code.ErrorType {
		addCalls++
		addedID = ep.GetId()
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, status)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	n.SetEventHandleOnRemoveEndpoint(func(node types.Node, ep types.Endpoint, status error_code.ErrorType) error_code.ErrorType {
		removeCalls++
		removedID = ep.GetId()
		assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, status)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	ep := CreateEndpoint(&n, 0x22345679, int64(n.GetPid()), n.GetHostname())
	require.NotNil(t, ep)

	// Act: add and then remove the same endpoint.
	ret = n.AddEndpoint(ep)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.RemoveEndpointByID(ep.GetId())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Assert: both endpoint lifecycle callbacks should fire exactly once.
	assert.Equal(t, 1, addCalls)
	assert.Equal(t, 1, removeCalls)
	assert.Equal(t, ep.GetId(), addedID)
	assert.Equal(t, ep.GetId(), removedID)
}
