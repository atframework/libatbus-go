package libatbus_impl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	error_code "github.com/atframework/libatbus-go/error_code"
	types "github.com/atframework/libatbus-go/types"
)

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
