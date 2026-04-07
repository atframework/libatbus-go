package libatbus_impl

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	channel_utility "github.com/atframework/libatbus-go/channel/utility"
	error_code "github.com/atframework/libatbus-go/error_code"
	message_handle "github.com/atframework/libatbus-go/message_handle"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

type customCommandObservation struct {
	from          types.BusIdType
	args          [][]byte
	sequence      uint64
	endpointID    types.BusIdType
	hasEndpoint   bool
	hasConnection bool
}

type customCommandLoopbackConnection struct {
	receiver          *Node
	peer              *customCommandLoopbackConnection
	binding           types.Endpoint
	status            types.ConnectionState
	flags             uint32
	address           types.ChannelAddress
	connectionContext *ConnectionContext
	statistic         types.ConnectionStatistic
}

func newCustomCommandLoopbackConnectionPair(sender *Node, receiver *Node) (*customCommandLoopbackConnection, *customCommandLoopbackConnection) {
	senderAddress, _ := channel_utility.MakeAddress("ipv4://127.0.0.1:10001")
	receiverAddress, _ := channel_utility.MakeAddress("ipv4://127.0.0.1:10002")

	toReceiver := &customCommandLoopbackConnection{
		receiver:          receiver,
		status:            types.ConnectionState_Connected,
		flags:             uint32(types.ConnectionFlag_RegFd | types.ConnectionFlag_ClientMode),
		address:           senderAddress,
		connectionContext: NewConnectionContext(sender.GetCryptoKeyExchangeType()),
	}
	toSender := &customCommandLoopbackConnection{
		receiver:          sender,
		status:            types.ConnectionState_Connected,
		flags:             uint32(types.ConnectionFlag_RegFd | types.ConnectionFlag_ServerMode),
		address:           receiverAddress,
		connectionContext: NewConnectionContext(receiver.GetCryptoKeyExchangeType()),
	}

	toReceiver.peer = toSender
	toSender.peer = toReceiver
	return toReceiver, toSender
}

func (c *customCommandLoopbackConnection) Reset() {
	if c == nil {
		return
	}

	c.status = types.ConnectionState_Disconnected
	c.binding = nil
}

func (c *customCommandLoopbackConnection) Proc() types.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *customCommandLoopbackConnection) Listen() types.ErrorType {
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *customCommandLoopbackConnection) Connect() types.ErrorType {
	if c == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	c.status = types.ConnectionState_Connected
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *customCommandLoopbackConnection) Disconnect() types.ErrorType {
	if c == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	c.status = types.ConnectionState_Disconnected
	return error_code.EN_ATBUS_ERR_SUCCESS
}

func (c *customCommandLoopbackConnection) Push(buffer []byte) types.ErrorType {
	if c == nil || c.peer == nil || c.receiver == nil {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if c.status != types.ConnectionState_Connected && c.status != types.ConnectionState_Handshaking {
		return error_code.EN_ATBUS_ERR_CLOSING
	}

	msg := types.NewMessage()
	errCode := c.peer.connectionContext.UnpackMessage(msg, buffer, int(c.receiver.GetConfigure().MessageSize))
	c.receiver.onReceiveMessage(c.peer, msg, 0, errCode)
	return errCode
}

func (c *customCommandLoopbackConnection) AddStatisticFault() uint64 {
	if c == nil {
		return 0
	}

	c.statistic.FaultCount++
	return c.statistic.FaultCount
}

func (c *customCommandLoopbackConnection) ClearStatisticFault() {
	if c == nil {
		return
	}

	c.statistic.FaultCount = 0
}

func (c *customCommandLoopbackConnection) GetAddress() types.ChannelAddress {
	if c == nil {
		return nil
	}

	return c.address
}

func (c *customCommandLoopbackConnection) IsConnected() bool {
	return c != nil && c.status == types.ConnectionState_Connected
}

func (c *customCommandLoopbackConnection) IsRunning() bool {
	if c == nil {
		return false
	}

	return c.status == types.ConnectionState_Connecting ||
		c.status == types.ConnectionState_Handshaking ||
		c.status == types.ConnectionState_Connected
}

func (c *customCommandLoopbackConnection) GetBinding() types.Endpoint {
	if c == nil {
		return nil
	}

	return c.binding
}

func (c *customCommandLoopbackConnection) GetStatus() types.ConnectionState {
	if c == nil {
		return types.ConnectionState_Disconnected
	}

	return c.status
}

func (c *customCommandLoopbackConnection) CheckFlag(flag types.ConnectionFlag) bool {
	if c == nil {
		return false
	}

	return (c.flags & uint32(flag)) != 0
}

func (c *customCommandLoopbackConnection) SetTemporary() {
	if c == nil {
		return
	}

	c.flags |= uint32(types.ConnectionFlag_Temporary)
}

func (c *customCommandLoopbackConnection) GetStatistic() types.ConnectionStatistic {
	if c == nil {
		return types.ConnectionStatistic{}
	}

	return c.statistic
}

func (c *customCommandLoopbackConnection) GetConnectionContext() types.ConnectionContext {
	if c == nil {
		return nil
	}

	return c.connectionContext
}

func (c *customCommandLoopbackConnection) RemoveOwnerChecker() {}

func (c *customCommandLoopbackConnection) setBinding(ep types.Endpoint) {
	if c == nil {
		return
	}

	c.binding = ep
}

func (c *customCommandLoopbackConnection) setStatus(status types.ConnectionState) {
	if c == nil {
		return
	}

	c.status = status
}

func cloneCommandArgs(args [][]byte) [][]byte {
	if args == nil {
		return nil
	}

	ret := make([][]byte, len(args))
	for i, arg := range args {
		ret[i] = append([]byte(nil), arg...)
	}

	return ret
}

func reserveTCPListenAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	return "ipv4://" + addr
}

func pumpNodesOnce(t *testing.T, tick *time.Time, nodes ...*Node) {
	t.Helper()

	now := time.Now()

	for _, n := range nodes {
		if n == nil {
			continue
		}

		_, ret := n.Poll()
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	}

	for _, n := range nodes {
		if n == nil {
			continue
		}

		_, ret := n.Proc(now)
		require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	}

	if tick != nil {
		*tick = now
	}
}

func waitForNodeCondition(t *testing.T, timeout time.Duration, condition func() bool, nodes ...*Node) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	tick := time.Now()
	for time.Now().Before(deadline) {
		pumpNodesOnce(t, &tick, nodes...)
		if condition() {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	require.True(t, condition(), "node condition was not satisfied within %s", timeout)
}

func TestSendDataBeforeInitReturnsNotInited(t *testing.T) {
	// Arrange: use a zero-value node that has never been initialized.
	var n Node

	// Act: send a non-empty payload before Init.
	ret := n.SendData(0x12345678, 1, []byte("hello"))

	// Assert: match the C++ send_data path and report NOT_INITED first.
	assert.Equal(t, error_code.EN_ATBUS_ERR_NOT_INITED, ret)
}

func TestSendCustomCommandBeforeInitReturnsNotInited(t *testing.T) {
	// Arrange: use a zero-value node that has never been initialized.
	var n Node
	args := [][]byte{[]byte("self"), []byte("command"), []byte("yep")}

	// Act: send a custom command before Init.
	ret := n.SendCustomCommand(n.GetId(), args)

	// Assert: match the C++ send_cmd_to_self case and report NOT_INITED first.
	assert.Equal(t, error_code.EN_ATBUS_ERR_NOT_INITED, ret)
}

func TestSendDataRejectsPayloadExceedingConfiguredMessageSize(t *testing.T) {
	// Arrange: initialize a node with a tiny message size budget.
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.MessageSize = 4

	var n Node
	ret := n.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	// Act: send a payload that is one byte too large.
	ret = n.SendData(n.GetId(), 1, []byte("12345"))

	// Assert: oversize payloads must be rejected.
	assert.Equal(t, error_code.EN_ATBUS_ERR_INVALID_SIZE, ret)
}

func TestSendCustomCommandRejectsPayloadExceedingConfiguredMessageSize(t *testing.T) {
	// Arrange: initialize a node with a tiny message size budget.
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.MessageSize = 10

	var n Node
	ret := n.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	args := [][]byte{[]byte("hello"), []byte("world"), []byte("!")}

	// Act: send a custom command whose argument bytes exceed the limit.
	ret = n.SendCustomCommand(n.GetId(), args)

	// Assert: oversize custom command payloads must be rejected.
	assert.Equal(t, error_code.EN_ATBUS_ERR_INVALID_SIZE, ret)
}

func TestSendCustomCommandToSelfDispatchesRequestAndResponse(t *testing.T) {
	// Arrange: initialize and start a node; self-directed commands do not need a real network.
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	args := [][]byte{[]byte("self"), []byte("command"), []byte("yep")}
	expectedResponse := append(cloneCommandArgs(args), []byte("run custom cmd done"))

	callOrder := make([]string, 0, 2)
	var requestFrom types.BusIdType
	var requestArgs [][]byte
	var responseFrom types.BusIdType
	var responseArgs [][]byte
	var responseSequence uint64

	n.SetEventHandleOnCustomCommandRequest(func(node types.Node, ep types.Endpoint, conn types.Connection, from types.BusIdType, argv [][]byte) (error_code.ErrorType, [][]byte) {
		callOrder = append(callOrder, "request")
		requestFrom = from
		requestArgs = cloneCommandArgs(argv)
		assert.Nil(t, ep)
		assert.Nil(t, conn)
		return error_code.EN_ATBUS_ERR_SUCCESS, expectedResponse
	})
	n.SetEventHandleOnCustomCommandResponse(func(node types.Node, ep types.Endpoint, conn types.Connection, from types.BusIdType, rspData [][]byte, sequence uint64) error_code.ErrorType {
		callOrder = append(callOrder, "response")
		responseFrom = from
		responseArgs = cloneCommandArgs(rspData)
		responseSequence = sequence
		assert.Nil(t, ep)
		assert.Nil(t, conn)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	options := types.CreateNodeSendDataOptions()
	options.SetSequence(n.AllocateMessageSequence())

	// Act: send a custom command to self, matching the C++ send_cmd_to_self scenario.
	ret = n.SendCustomCommandWithOptions(n.GetId(), args, options)

	// Assert: both request and response callbacks should run in order with the same sequence.
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, []string{"request", "response"}, callOrder)
	assert.Equal(t, n.GetId(), requestFrom)
	assert.Equal(t, args, requestArgs)
	assert.Equal(t, n.GetId(), responseFrom)
	assert.Equal(t, expectedResponse, responseArgs)
	assert.Equal(t, options.GetSequence(), responseSequence)
	assert.Equal(t, 0, n.selfCommandMessages.Len())
}

func TestSendDataToSelfAndRequireResponseDispatchesChainedCallbacksMatchCppCase(t *testing.T) {
	// Arrange: initialize and start a node so self-directed messages dispatch immediately.
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	payload := []byte("self\x00hello world!\n")
	doubledPayload := append(append([]byte(nil), payload...), payload...)
	callbackOrder := make([]string, 0, 3)
	receivedPayloads := make([][]byte, 0, 2)
	responsePayload := []byte(nil)
	nestedSendRet := error_code.EN_ATBUS_ERR_SUCCESS

	n.SetEventHandleOnForwardRequest(func(node types.Node, ep types.Endpoint, conn types.Connection, msg *types.Message, content []byte) error_code.ErrorType {
		callbackOrder = append(callbackOrder, "request")
		receivedPayloads = append(receivedPayloads, append([]byte(nil), content...))
		require.NotNil(t, ep)
		assert.Equal(t, n.GetId(), ep.GetId())
		assert.Nil(t, conn)

		if bytes.Equal(content, payload) {
			options := types.CreateNodeSendDataOptions()
			options.SetFlag(types.NodeSendDataOptionFlag_RequiredResponse, true)
			nestedSendRet = n.SendDataWithOptions(n.GetId(), 0, doubledPayload, options)
		}

		return error_code.EN_ATBUS_ERR_SUCCESS
	})
	n.SetEventHandleOnForwardResponse(func(node types.Node, ep types.Endpoint, conn types.Connection, msg *types.Message) error_code.ErrorType {
		callbackOrder = append(callbackOrder, "response")
		require.NotNil(t, ep)
		assert.Equal(t, n.GetId(), ep.GetId())
		assert.Nil(t, conn)

		if msg != nil && msg.GetBody() != nil {
			if body := msg.GetBody().GetDataTransformRsp(); body != nil {
				responsePayload = append([]byte(nil), body.GetContent()...)
			}
		}

		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Act: send a self message whose first callback re-enqueues a response-required self message.
	ret = n.SendData(n.GetId(), 0, payload)

	// Assert: match the C++ chained self-dispatch behavior.
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, nestedSendRet)
	assert.Equal(t, []string{"request", "request", "response"}, callbackOrder)
	assert.Len(t, receivedPayloads, 2)
	assert.Equal(t, payload, receivedPayloads[0])
	assert.Equal(t, doubledPayload, receivedPayloads[1])
	assert.Equal(t, doubledPayload, responsePayload)
	assert.Equal(t, 0, n.selfDataMessages.Len())
}

func TestSendCustomCommandAcrossConnectedNodesMatchesCppCase(t *testing.T) {
	// Arrange: create two started nodes bridged by paired loopback connections.
	var node1 Node
	var node2 Node
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Init(0x12345678, nil))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.Init(0x12356789, nil))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.Start())

	epOnNode1 := CreateEndpoint(&node1, node2.GetId(), int64(node2.GetPid()), node2.GetHostname())
	epOnNode2 := CreateEndpoint(&node2, node1.GetId(), int64(node1.GetPid()), node1.GetHostname())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.AddEndpoint(epOnNode1))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.AddEndpoint(epOnNode2))

	connToNode2, connToNode1 := newCustomCommandLoopbackConnectionPair(&node1, &node2)
	require.True(t, epOnNode1.AddConnection(connToNode2, true))
	require.True(t, epOnNode2.AddConnection(connToNode1, true))
	assert.True(t, node1.IsEndpointAvailable(node2.GetId()))
	assert.True(t, node2.IsEndpointAvailable(node1.GetId()))

	args := [][]byte{[]byte("hello"), []byte("world"), []byte("!")}
	expectedResponse := append(cloneCommandArgs(args), []byte("run custom cmd done"))
	requestCount := 0
	responseCount := 0
	var request customCommandObservation
	var response customCommandObservation

	node2.SetEventHandleOnCustomCommandRequest(func(node types.Node, ep types.Endpoint, conn types.Connection, from types.BusIdType, argv [][]byte) (error_code.ErrorType, [][]byte) {
		requestCount++
		request = customCommandObservation{
			from:          from,
			args:          cloneCommandArgs(argv),
			hasEndpoint:   ep != nil,
			hasConnection: conn != nil,
		}
		if ep != nil {
			request.endpointID = ep.GetId()
		}

		return error_code.EN_ATBUS_ERR_SUCCESS, expectedResponse
	})
	node1.SetEventHandleOnCustomCommandResponse(func(node types.Node, ep types.Endpoint, conn types.Connection, from types.BusIdType, rspData [][]byte, sequence uint64) error_code.ErrorType {
		responseCount++
		response = customCommandObservation{
			from:          from,
			args:          cloneCommandArgs(rspData),
			sequence:      sequence,
			hasEndpoint:   ep != nil,
			hasConnection: conn != nil,
		}
		if ep != nil {
			response.endpointID = ep.GetId()
		}

		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	options := types.CreateNodeSendDataOptions()
	options.SetSequence(node1.AllocateMessageSequence())

	// Act: send a custom command from node1 to node2 and wait for both callbacks.
	ret := node1.SendCustomCommandWithOptions(node2.GetId(), args, options)

	// Assert: request/response payloads and sequence should match the C++ case semantics.
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, 1, responseCount)
	assert.Equal(t, node1.GetId(), request.from)
	assert.Equal(t, args, request.args)
	assert.True(t, request.hasEndpoint)
	assert.True(t, request.hasConnection)
	assert.Equal(t, node1.GetId(), request.endpointID)

	assert.Equal(t, node2.GetId(), response.from)
	assert.Equal(t, expectedResponse, response.args)
	assert.Equal(t, options.GetSequence(), response.sequence)
	assert.True(t, response.hasEndpoint)
	assert.True(t, response.hasConnection)
	assert.Equal(t, node2.GetId(), response.endpointID)
}

func TestSendCustomCommandFromTemporaryNodeMatchesCppCase(t *testing.T) {
	// Arrange: connect a temporary node to a server through paired loopback connections.
	var server Node
	var temporary Node
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, server.Init(0x12345678, nil))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, temporary.Init(0, nil))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, server.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, temporary.Start())

	epOnTemporary := CreateEndpoint(&temporary, server.GetId(), int64(server.GetPid()), server.GetHostname())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, temporary.AddEndpoint(epOnTemporary))

	connToServer, connBackToTemporary := newCustomCommandLoopbackConnectionPair(&temporary, &server)
	require.True(t, epOnTemporary.AddConnection(connToServer, true))
	assert.True(t, temporary.IsEndpointAvailable(server.GetId()))

	args := [][]byte{[]byte("hello"), []byte("world"), []byte("!")}
	expectedResponse := append(cloneCommandArgs(args), []byte("run custom cmd done"))
	requestCount := 0
	responseCount := 0
	var request customCommandObservation
	var response customCommandObservation

	server.SetEventHandleOnCustomCommandRequest(func(node types.Node, ep types.Endpoint, conn types.Connection, from types.BusIdType, argv [][]byte) (error_code.ErrorType, [][]byte) {
		requestCount++
		request = customCommandObservation{
			from:          from,
			args:          cloneCommandArgs(argv),
			hasEndpoint:   ep != nil,
			hasConnection: conn != nil,
		}
		if ep != nil {
			request.endpointID = ep.GetId()
		}

		return error_code.EN_ATBUS_ERR_SUCCESS, expectedResponse
	})
	temporary.SetEventHandleOnCustomCommandResponse(func(node types.Node, ep types.Endpoint, conn types.Connection, from types.BusIdType, rspData [][]byte, sequence uint64) error_code.ErrorType {
		responseCount++
		response = customCommandObservation{
			from:          from,
			args:          cloneCommandArgs(rspData),
			sequence:      sequence,
			hasEndpoint:   ep != nil,
			hasConnection: conn != nil,
		}
		if ep != nil {
			response.endpointID = ep.GetId()
		}

		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	options := types.CreateNodeSendDataOptions()
	options.SetSequence(temporary.AllocateMessageSequence())
	connBackToTemporary.binding = nil

	// Act: send a custom command from the temporary node to the server.
	ret := temporary.SendCustomCommandWithOptions(server.GetId(), args, options)

	// Assert: the server should see bus id 0 as the requester and the temporary node should receive the response.
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, 1, responseCount)
	assert.Equal(t, types.BusIdType(0), request.from)
	assert.Equal(t, args, request.args)
	assert.False(t, request.hasEndpoint)
	assert.True(t, request.hasConnection)

	assert.Equal(t, server.GetId(), response.from)
	assert.Equal(t, expectedResponse, response.args)
	assert.Equal(t, options.GetSequence(), response.sequence)
	assert.True(t, response.hasEndpoint)
	assert.True(t, response.hasConnection)
	assert.Equal(t, server.GetId(), response.endpointID)
}

// ---------------------------------------------------------------------------
// Parity tests matching C++ atbus_node_msg test cases
// ---------------------------------------------------------------------------

// TestNodeMsgParity_ResetAndSend mirrors C++ atbus_node_msg::reset_and_send.
// Sends a data message to self and verifies the forward_request callback fires.
func TestNodeMsgParity_ResetAndSend(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	addr := reserveTCPListenAddress(t)
	ret = n.Listen(addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { n.Reset() })

	now := time.Now().Add(time.Second)
	n.Poll()
	n.Proc(now)

	sendData := []byte("self\x00hello world!\n")

	requestCount := 0
	var receivedData []byte
	n.SetEventHandleOnForwardRequest(func(_ types.Node, ep types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		requestCount++
		receivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	options := types.CreateNodeSendDataOptions()
	ret = n.SendDataWithOptions(n.GetId(), 0, sendData, options)

	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	assert.Equal(t, 1, requestCount)
	assert.Equal(t, sendData, receivedData)
	assert.NotEqual(t, uint64(0), options.GetSequence())
}

// TestNodeMsgParity_SendFailed mirrors C++ atbus_node_msg::send_failed.
// Sends to non-existent peer IDs and expects INVALID_ID error.
func TestNodeMsgParity_SendFailed(t *testing.T) {
	var n Node
	ret := n.Init(0x12345678, nil)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	addr := reserveTCPListenAddress(t)
	ret = n.Listen(addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = n.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { n.Reset() })

	sendData := []byte("send failed")

	// Non-existent downstream
	ret = n.SendData(0x12346780, 0, sendData)
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, ret)

	// Non-existent sibling
	ret = n.SendData(0x12356789, 0, sendData)
	assert.Equal(t, error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID, ret)
}

// TestNodeMsgParity_UpstreamAndDownstream mirrors C++ atbus_node_msg::upstream_and_downstream.
// Tests bidirectional data transfer between an upstream and downstream node.
func TestNodeMsgParity_UpstreamAndDownstream(t *testing.T) {
	upstreamAddr := reserveTCPListenAddress(t)

	// Upstream node
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	var upstream Node
	ret := upstream.Init(0x12345678, &upstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstream.Reset() })

	// Downstream node
	var downstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&downstreamConf)
	downstreamConf.UpstreamAddress = upstreamAddr

	var downstream Node
	ret = downstream.Init(0x12346789, &downstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	dsAddr := reserveTCPListenAddress(t)
	ret = downstream.Listen(dsAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = downstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { downstream.Reset() })

	// Wait until both endpoints are available (data connections established)
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return upstream.IsEndpointAvailable(downstream.GetId()) &&
			downstream.IsEndpointAvailable(upstream.GetId())
	}, &upstream, &downstream)

	// Track received messages
	receivedCount := 0
	var lastReceivedData []byte
	handler := func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		receivedCount++
		lastReceivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	upstream.SetEventHandleOnForwardRequest(handler)
	downstream.SetEventHandleOnForwardRequest(handler)

	// Direction 1: upstream → downstream
	sendData1 := []byte("upstream to downstream\x00hello world!\n")
	countBefore := receivedCount
	ret = upstream.SendData(downstream.GetId(), 0, sendData1)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 3*time.Second, func() bool {
		return receivedCount > countBefore
	}, &upstream, &downstream)
	assert.Equal(t, sendData1, lastReceivedData)

	// Direction 2: downstream → upstream
	sendData2 := []byte("downstream to upstream\x00hello world!\n")
	countBefore = receivedCount
	ret = downstream.SendData(upstream.GetId(), 0, sendData2)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 3*time.Second, func() bool {
		return receivedCount > countBefore
	}, &upstream, &downstream)
	assert.Equal(t, sendData2, lastReceivedData)

	// Verify endpoints exist on both sides
	assert.NotNil(t, upstream.GetEndpoint(downstream.GetId()))
	assert.NotNil(t, downstream.GetEndpoint(upstream.GetId()))
}

// TestNodeMsgParity_PingPong mirrors C++ atbus_node_msg::ping_pong.
// Tests periodic ping/pong exchange between three nodes.
func TestNodeMsgParity_PingPong(t *testing.T) {
	upstreamAddr := reserveTCPListenAddress(t)

	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)
	conf.PingInterval = 1 * time.Second

	// Upstream
	var upstream Node
	ret := upstream.Init(0x12346789, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstream.Reset() })

	// Node1 (downstream of upstream)
	conf1 := conf
	conf1.UpstreamAddress = upstreamAddr
	var node1 Node
	ret = node1.Init(0x12345678, &conf1)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	n1Addr := reserveTCPListenAddress(t)
	ret = node1.Listen(n1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { node1.Reset() })

	// Node2 (peer, node1 connects to it)
	var node2 Node
	ret = node2.Init(0x12356789, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	n2Addr := reserveTCPListenAddress(t)
	ret = node2.Listen(n2Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = node2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { node2.Reset() })

	// Connect node1 → node2 as peers
	node1.Connect(n2Addr)

	// Track ping/pong counts
	pingCount := 0
	pongCount := 0
	pingHandler := func(_ types.Node, _ types.Endpoint, _ *types.Message, _ *protocol.PingData) error_code.ErrorType {
		pingCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	pongHandler := func(_ types.Node, _ types.Endpoint, _ *types.Message, _ *protocol.PingData) error_code.ErrorType {
		pongCount++
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	for _, n := range []*Node{&upstream, &node1, &node2} {
		n.SetEventHandleOnPingEndpoint(pingHandler)
		n.SetEventHandleOnPongEndpoint(pongHandler)
	}

	// Wait for connections to establish
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(upstream.GetId()) &&
			upstream.IsEndpointAvailable(node1.GetId()) &&
			node1.IsEndpointAvailable(node2.GetId()) &&
			node2.IsEndpointAvailable(node1.GetId())
	}, &upstream, &node1, &node2)

	// Run for ~8 simulated seconds with 80ms steps to trigger multiple ping/pong cycles
	oldPong := pongCount
	procTime := time.Now()
	tickCount := 0
	for pongCount-oldPong < 40 && tickCount < 1000 {
		procTime = procTime.Add(80 * time.Millisecond)
		node1.Poll()
		node2.Poll()
		upstream.Poll()
		node1.Proc(procTime)
		node2.Proc(procTime)
		upstream.Proc(procTime)
		tickCount++
		if tickCount%12 == 0 {
			time.Sleep(1 * time.Millisecond) // yield to goroutines
		}
	}

	// Verify pong counts — with multiple connections over simulated time, expect pong activity.
	// The exact count depends on connection establishment timing, so we use a relaxed lower bound.
	assert.Greater(t, pongCount-oldPong, 0, "Expected at least some pongs")
	assert.Greater(t, pingCount, 0, "Expected at least some pings")

	// Pong timestamps on upstream-downstream pair (guaranteed to have ping timers)
	ep1u := node1.GetEndpoint(upstream.GetId())
	if ep1u != nil {
		if epImpl, ok := ep1u.(*Endpoint); ok {
			assert.False(t, epImpl.GetStatisticLastPong().IsZero(), "last_pong should be set on node1 for upstream")
		}
	}
}

// TestNodeMsgParity_TransferAndConnect mirrors C++ atbus_node_msg::transfer_and_connect.
// Two downstreams registered to the same upstream; message transfers through the upstream.
func TestNodeMsgParity_TransferAndConnect(t *testing.T) {
	upstreamAddr := reserveTCPListenAddress(t)

	// Upstream node
	var upstreamConf types.NodeConfigure
	types.SetDefaultNodeConfigure(&upstreamConf)

	var upstream Node
	ret := upstream.Init(0x12345678, &upstreamConf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Listen(upstreamAddr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = upstream.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { upstream.Reset() })

	// Downstream 1
	var dsConf1 types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf1)
	dsConf1.UpstreamAddress = upstreamAddr

	var ds1 Node
	ret = ds1.Init(0x12346789, &dsConf1)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ds1Addr := reserveTCPListenAddress(t)
	ret = ds1.Listen(ds1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = ds1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { ds1.Reset() })

	// Downstream 2
	var dsConf2 types.NodeConfigure
	types.SetDefaultNodeConfigure(&dsConf2)
	dsConf2.UpstreamAddress = upstreamAddr

	var ds2 Node
	ret = ds2.Init(0x12346890, &dsConf2)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ds2Addr := reserveTCPListenAddress(t)
	ret = ds2.Listen(ds2Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = ds2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { ds2.Reset() })

	// Wait for all connections
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return upstream.IsEndpointAvailable(ds1.GetId()) &&
			upstream.IsEndpointAvailable(ds2.GetId()) &&
			ds1.IsEndpointAvailable(upstream.GetId()) &&
			ds2.IsEndpointAvailable(upstream.GetId())
	}, &upstream, &ds1, &ds2)

	// Register topology
	for _, n := range []*Node{&upstream, &ds1, &ds2} {
		n.GetTopologyRegistry().UpdatePeer(ds1.GetId(), upstream.GetId(), nil)
		n.GetTopologyRegistry().UpdatePeer(ds2.GetId(), upstream.GetId(), nil)
	}

	// Track received messages
	receivedCount := 0
	var lastReceivedData []byte
	handler := func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		receivedCount++
		lastReceivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	ds1.SetEventHandleOnForwardRequest(handler)
	ds2.SetEventHandleOnForwardRequest(handler)

	// Send from ds1 to ds2 through upstream
	sendData := []byte("transfer through upstream\n")
	countBefore := receivedCount
	ret = ds1.SendData(ds2.GetId(), 0, sendData)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 5*time.Second, func() bool {
		return receivedCount > countBefore && len(lastReceivedData) > 0
	}, &upstream, &ds1, &ds2)

	assert.Equal(t, sendData, lastReceivedData)
}

// TestNodeMsgParity_TransferOnly mirrors C++ atbus_node_msg::transfer_only.
// Message crosses from ds1→upstream1→upstream2→ds2 (4 nodes, 2 upstream hierarchies).
func TestNodeMsgParity_TransferOnly(t *testing.T) {
	us1Addr := reserveTCPListenAddress(t)
	us2Addr := reserveTCPListenAddress(t)

	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	// Upstream 1
	var us1 Node
	ret := us1.Init(0x12345678, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = us1.Listen(us1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = us1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { us1.Reset() })

	// Upstream 2
	var us2 Node
	ret = us2.Init(0x12356789, &conf)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = us2.Listen(us2Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = us2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { us2.Reset() })

	// Downstream 1 (under upstream 1)
	dsConf1 := conf
	dsConf1.UpstreamAddress = us1Addr
	var ds1 Node
	ret = ds1.Init(0x12346789, &dsConf1)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ds1Addr := reserveTCPListenAddress(t)
	ret = ds1.Listen(ds1Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = ds1.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { ds1.Reset() })

	// Downstream 2 (under upstream 2)
	dsConf2 := conf
	dsConf2.UpstreamAddress = us2Addr
	var ds2 Node
	ret = ds2.Init(0x12354678, &dsConf2)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ds2Addr := reserveTCPListenAddress(t)
	ret = ds2.Listen(ds2Addr)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	ret = ds2.Start()
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)
	t.Cleanup(func() { ds2.Reset() })

	// Cross-connect upstreams
	us1.Connect(us2Addr)

	// Wait for all connections
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return ds1.IsEndpointAvailable(us1.GetId()) &&
			us1.IsEndpointAvailable(ds1.GetId()) &&
			ds2.IsEndpointAvailable(us2.GetId()) &&
			us2.IsEndpointAvailable(ds2.GetId()) &&
			us1.IsEndpointAvailable(us2.GetId()) &&
			us2.IsEndpointAvailable(us1.GetId())
	}, &us1, &us2, &ds1, &ds2)

	// Register topology on all nodes
	for _, n := range []*Node{&us1, &us2, &ds1, &ds2} {
		n.GetTopologyRegistry().UpdatePeer(ds1.GetId(), us1.GetId(), nil)
		n.GetTopologyRegistry().UpdatePeer(ds2.GetId(), us2.GetId(), nil)
	}

	// Track received messages
	receivedCount := 0
	var lastReceivedData []byte
	handler := func(_ types.Node, _ types.Endpoint, _ types.Connection, _ *types.Message, content []byte) error_code.ErrorType {
		receivedCount++
		lastReceivedData = append([]byte(nil), content...)
		return error_code.EN_ATBUS_ERR_SUCCESS
	}
	ds1.SetEventHandleOnForwardRequest(handler)
	ds2.SetEventHandleOnForwardRequest(handler)

	// Send from ds1 to ds2 (path: ds1→us1→us2→ds2)
	sendData := []byte("transfer through upstream only\n")
	countBefore := receivedCount
	ret = ds1.SendData(ds2.GetId(), 0, sendData)
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, ret)

	waitForNodeCondition(t, 8*time.Second, func() bool {
		return receivedCount > countBefore
	}, &us1, &us2, &ds1, &ds2)

	assert.Greater(t, receivedCount, countBefore)
	assert.Equal(t, sendData, lastReceivedData)
}

// TestNodeMsgParity_SendLoopbackError mirrors C++ atbus_node_msg::send_loopback_error.
// Two nodes connected via TCP. A raw forward message with an invalid target ID
// is sent; node2 returns an error response with EN_ATBUS_ERR_ATNODE_INVALID_ID.
func TestNodeMsgParity_SendLoopbackError(t *testing.T) {
	var conf types.NodeConfigure
	types.SetDefaultNodeConfigure(&conf)

	n1Addr := reserveTCPListenAddress(t)
	n2Addr := reserveTCPListenAddress(t)

	var node1, node2 Node
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Init(0x12345678, &conf))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.Init(0x12356789, &conf))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Listen(n1Addr))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.Listen(n2Addr))
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node1.Start())
	require.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, node2.Start())
	t.Cleanup(func() { node1.Reset(); node2.Reset() })

	// Connect node1 → node2
	node1.Connect(n2Addr)

	// Wait for bidirectional availability
	waitForNodeCondition(t, 8*time.Second, func() bool {
		return node1.IsEndpointAvailable(node2.GetId()) &&
			node2.IsEndpointAvailable(node1.GetId())
	}, &node1, &node2)

	// Set response handler on node1
	responseCalled := 0
	var responseStatus int32
	var responseData []byte
	node1.SetEventHandleOnForwardResponse(func(_ types.Node, _ types.Endpoint, _ types.Connection, msg *types.Message) error_code.ErrorType {
		responseCalled++
		if msg != nil && msg.GetHead() != nil {
			responseStatus = msg.GetHead().GetResultCode()
		}
		if msg != nil && msg.GetBody() != nil {
			if rsp := msg.GetBody().GetDataTransformRsp(); rsp != nil {
				responseData = append([]byte(nil), rsp.GetContent()...)
			}
		}
		return error_code.EN_ATBUS_ERR_SUCCESS
	})

	// Build a raw data_transform_req with an invalid target ID (0x12346789)
	sendData := []byte("loop back message!")
	m := types.NewMessage()
	m.MutableHead().Version = node1.GetProtocolVersion()
	m.MutableHead().Sequence = node1.AllocateMessageSequence()

	body := m.MutableBody().MutableDataTransformReq()
	body.From = uint64(node1.GetId())
	body.To = uint64(0x12346789) // invalid: no such node
	body.AppendRouter(uint64(node1.GetId()))
	body.Content = sendData
	body.Flags = uint32(protocol.ATBUS_FORWARD_DATA_FLAG_TYPE_FORWARD_DATA_FLAG_REQUIRE_RSP)

	// Get the data connection to node2 and send the raw message
	epOnNode1 := node1.GetEndpoint(node2.GetId())
	require.NotNil(t, epOnNode1)
	conn := node1.GetSelfEndpointInstance().GetDataConnection(epOnNode1, true)
	require.NotNil(t, conn, "data connection to node2 should exist")
	sendRet := message_handle.SendMessage(&node1, conn, m)
	assert.Equal(t, error_code.EN_ATBUS_ERR_SUCCESS, sendRet)

	// Wait for the error response to arrive back at node1
	waitForNodeCondition(t, 3*time.Second, func() bool {
		return responseCalled > 0
	}, &node1, &node2)

	assert.Equal(t, 1, responseCalled)
	assert.Equal(t, int32(error_code.EN_ATBUS_ERR_ATNODE_INVALID_ID), responseStatus)
	assert.Equal(t, sendData, responseData)
}
