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
