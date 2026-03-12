package libatbus_types

import (
	protocol "github.com/atframework/libatbus-go/protocol"
)

// MessageBodyType is the Go equivalent of C++ message_body_type (the oneof case).
// It aliases protocol.MessageBody_EnMessageTypeID for convenience.
type MessageBodyType = protocol.MessageBody_EnMessageTypeID

// MessageBodyType constants using generated enum values.
const (
	MessageBodyTypeUnknown          = protocol.MessageBody_EnMessageTypeID_NONE
	MessageBodyTypeCustomCommandReq = protocol.MessageBody_EnMessageTypeID_CustomCommandReq
	MessageBodyTypeCustomCommandRsp = protocol.MessageBody_EnMessageTypeID_CustomCommandRsp
	MessageBodyTypeDataTransformReq = protocol.MessageBody_EnMessageTypeID_DataTransformReq
	MessageBodyTypeDataTransformRsp = protocol.MessageBody_EnMessageTypeID_DataTransformRsp
	MessageBodyTypeNodeRegisterReq  = protocol.MessageBody_EnMessageTypeID_NodeRegisterReq
	MessageBodyTypeNodeRegisterRsp  = protocol.MessageBody_EnMessageTypeID_NodeRegisterRsp
	MessageBodyTypeNodePingReq         = protocol.MessageBody_EnMessageTypeID_NodePingReq
	MessageBodyTypeNodePongRsp         = protocol.MessageBody_EnMessageTypeID_NodePongRsp
	MessageBodyTypeHandshakeConfirm    = protocol.MessageBody_EnMessageTypeID_HandshakeConfirm
	MessageBodyTypeNodeMax             = MessageBodyTypeHandshakeConfirm
)

// Message is the Go equivalent of C++ atbus::message.
//
// It wraps the protobuf-generated protocol.MessageHead and protocol.MessageBody,
// and provides accessor methods similar to the C++ class. The Go version does not
// use protobuf Arena or inplace caching; it holds the structs directly.
type Message struct {
	head *protocol.MessageHead
	body *protocol.MessageBody

	// unpackError stores any error encountered during unpacking.
	unpackError string
}

// NewMessage creates a new empty Message.
func NewMessage() *Message {
	return &Message{
		head: &protocol.MessageHead{},
		body: &protocol.MessageBody{},
	}
}

// MutableHead returns a mutable reference to the message head.
func (m *Message) MutableHead() *protocol.MessageHead {
	if m.head == nil {
		m.head = &protocol.MessageHead{}
	}
	return m.head
}

// MutableBody returns a mutable reference to the message body.
func (m *Message) MutableBody() *protocol.MessageBody {
	if m.body == nil {
		m.body = &protocol.MessageBody{}
	}
	return m.body
}

// GetHead returns the message head (may be nil).
func (m *Message) GetHead() *protocol.MessageHead {
	if m == nil {
		return nil
	}
	return m.head
}

// GetBody returns the message body (may be nil).
func (m *Message) GetBody() *protocol.MessageBody {
	if m == nil {
		return nil
	}
	return m.body
}

// Head returns the message head, creating an empty one if nil.
func (m *Message) Head() *protocol.MessageHead {
	return m.MutableHead()
}

// Body returns the message body, creating an empty one if nil.
func (m *Message) Body() *protocol.MessageBody {
	return m.MutableBody()
}

// GetBodyType returns the body oneof case based on the set message type.
// It uses the generated GetMessageTypeOneofCase() method.
func (m *Message) GetBodyType() MessageBodyType {
	if m == nil || m.body == nil {
		return MessageBodyTypeUnknown
	}
	return m.body.GetMessageTypeOneofCase()
}

// GetUnpackErrorMessage returns any unpack error message stored in the message.
func (m *Message) GetUnpackErrorMessage() string {
	if m == nil {
		return ""
	}
	return m.unpackError
}

// SetUnpackError sets the unpack error message.
func (m *Message) SetUnpackError(err string) {
	if m != nil {
		m.unpackError = err
	}
}
