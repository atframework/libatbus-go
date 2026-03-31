package libatbus_message_handle

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"time"

	lu "github.com/atframework/atframe-utils-go/lang_utility"

	buffer "github.com/atframework/libatbus-go/buffer"
	channel_utility "github.com/atframework/libatbus-go/channel/utility"
	error_code "github.com/atframework/libatbus-go/error_code"
	protocol "github.com/atframework/libatbus-go/protocol"
	types "github.com/atframework/libatbus-go/types"
)

// Golang暂不支持单工通道
func buildSupportedChannelSchemes() map[string]struct{} {
	ret := make(map[string]struct{})
	ret["dns"] = struct{}{}
	ret["ipv4"] = struct{}{}
	ret["ipv6"] = struct{}{}
	ret["atcp"] = struct{}{}
	ret["unix"] = struct{}{}
	ret["pipe"] = struct{}{}
	return ret
}

var _supportedChannelSchemes map[string]struct{} = nil

func getSupportedChannelSchemes() map[string]struct{} {
	if _supportedChannelSchemes == nil {
		_supportedChannelSchemes = buildSupportedChannelSchemes()
	}
	return _supportedChannelSchemes
}

func getConnectionBinding(conn types.Connection) types.Endpoint {
	if conn == nil {
		return nil
	}

	return getConnectionBinding(conn)
}

// This file is a Go port of the core helpers in atframework/libatbus
// `atbus_message_handler.cpp`.
//
// Scope note: only the following functions are fully implemented here:
//   - UnpackMessage
//   - PackMessage
//   - GetBodyName
//   - GenerateAccessData
//   - MakeAccessDataPlaintext
//   - CalculateAccessDataSignature

// ---- pack/unpack helpers ----

// UnpackMessage decodes binary data and populates a Message.
//
// This function calls ConnectionContext.UnpackMessage internally.
// maxBodySize follows the C++ signature: it bounds the decoded body size.
// If maxBodySize <= 0, the limit is not checked.
func UnpackMessage(connCtx types.ConnectionContext, data []byte, maxBodySize int) (*types.Message, error_code.ErrorType) {
	if connCtx == nil {
		return nil, error_code.EN_ATBUS_ERR_PARAMS
	}

	msg := types.NewMessage()
	err := connCtx.UnpackMessage(msg, data, maxBodySize)
	if err != error_code.EN_ATBUS_ERR_SUCCESS {
		return msg, err
	}

	return msg, error_code.EN_ATBUS_ERR_SUCCESS
}

// PackMessage packs a Message and returns the binary data in a StaticBufferBlock.
//
// maxBodySize bounds the body size before packing.
// If maxBodySize <= 0, the limit is not checked.
func PackMessage(connCtx types.ConnectionContext, msg *types.Message, protocolVersion int32, maxBodySize int) (*buffer.StaticBufferBlock, error_code.ErrorType) {
	if connCtx == nil {
		return nil, error_code.EN_ATBUS_ERR_PARAMS
	}
	if msg == nil {
		return nil, error_code.EN_ATBUS_ERR_PARAMS
	}

	return connCtx.PackMessage(msg, protocolVersion, maxBodySize)
}

// ---- message body name helpers ----

// messageBodyFullNames maps oneof case IDs to their protobuf full names.
// Initialized from protobuf reflection at package load time.
var messageBodyFullNames map[int]string

func init() {
	// Build the body names map from protobuf reflection to avoid hardcoding.
	messageBodyFullNames = buildMessageBodyFullNames()
}

// buildMessageBodyFullNames uses protobuf reflection to build the mapping
// from oneof case IDs to their full field names.
func buildMessageBodyFullNames() map[int]string {
	result := make(map[int]string)

	// Get the message descriptor for MessageBody
	msg := &protocol.MessageBody{}
	md := msg.ProtoReflect().Descriptor()

	// Find the "message_type" oneof
	oneofs := md.Oneofs()
	for i := 0; i < oneofs.Len(); i++ {
		oneof := oneofs.Get(i)
		if oneof.Name() == "message_type" {
			// Iterate over all fields in this oneof
			fields := oneof.Fields()
			for j := 0; j < fields.Len(); j++ {
				field := fields.Get(j)
				fieldNum := int(field.Number())
				fullName := string(field.FullName())
				result[fieldNum] = fullName
			}
			break
		}
	}

	return result
}

// GetBodyName returns the protobuf full name of the oneof field for message_body.
//
// This mirrors C++ `message_handler::get_body_name()` which uses the protobuf
// descriptor's `FieldDescriptor::full_name()`.
func GetBodyName(bodyCase int) string {
	if n, ok := messageBodyFullNames[bodyCase]; ok && n != "" {
		return n
	}
	return "Unknown"
}

// ---- access_data helpers ----

// GenerateAccessData fills protocol.AccessData and appends one signature entry per token.
//
// It mirrors the C++ overload that takes crypto_handshake_data.
func GenerateAccessData(ad *protocol.AccessData, busID types.BusIdType, nonce1, nonce2 uint64, accessTokens [][]byte, hd *protocol.CryptoHandshakeData) {
	GenerateAccessDataWithTimestamp(ad, busID, nonce1, nonce2, accessTokens, hd, time.Now().Unix())
}

// GenerateAccessDataWithTimestamp is like GenerateAccessData but allows specifying a fixed timestamp.
// This is primarily useful for cross-language compatibility testing.
func GenerateAccessDataWithTimestamp(ad *protocol.AccessData, busID types.BusIdType, nonce1, nonce2 uint64, accessTokens [][]byte, hd *protocol.CryptoHandshakeData, timestamp int64) {
	if ad == nil {
		return
	}
	ad.Algorithm = protocol.ATBUS_ACCESS_DATA_ALGORITHM_TYPE_ATBUS_ACCESS_DATA_ALGORITHM_HMAC_SHA256
	ad.Timestamp = timestamp
	ad.Nonce1 = nonce1
	ad.Nonce2 = nonce2

	plaintext := MakeAccessDataPlaintextFromHandshake(busID, ad, hd)
	ad.Signature = make([][]byte, 0, len(accessTokens))
	for _, token := range accessTokens {
		ad.Signature = append(ad.Signature, CalculateAccessDataSignature(ad, token, plaintext))
	}
}

// GenerateAccessDataForCustomCommand mirrors the C++ overload that takes custom_command_data.
func GenerateAccessDataForCustomCommand(ad *protocol.AccessData, busID types.BusIdType, nonce1, nonce2 uint64, accessTokens [][]byte, cs *protocol.CustomCommandData) {
	if ad == nil {
		return
	}
	ad.Algorithm = protocol.ATBUS_ACCESS_DATA_ALGORITHM_TYPE_ATBUS_ACCESS_DATA_ALGORITHM_HMAC_SHA256
	ad.Timestamp = time.Now().Unix()
	ad.Nonce1 = nonce1
	ad.Nonce2 = nonce2

	plaintext := MakeAccessDataPlaintextFromCustomCommand(busID, ad, cs)
	ad.Signature = make([][]byte, 0, len(accessTokens))
	for _, token := range accessTokens {
		ad.Signature = append(ad.Signature, CalculateAccessDataSignature(ad, token, plaintext))
	}
}

// MakeAccessDataPlaintextFromHandshake builds the signed plaintext string.
//
// C++ rules:
//   - If public_key is empty:
//     "<timestamp>:<nonce1>-<nonce2>:<bus_id>"
//   - Else:
//     "<timestamp>:<nonce1>-<nonce2>:<bus_id>:<type>:<sha256_hex(public_key)>"
func MakeAccessDataPlaintextFromHandshake(busID types.BusIdType, ad *protocol.AccessData, hd *protocol.CryptoHandshakeData) string {
	if hd == nil || len(hd.GetPublicKey()) == 0 {
		return fmt.Sprintf("%d:%d-%d:%d", ad.GetTimestamp(), ad.GetNonce1(), ad.GetNonce2(), busID)
	}

	h := sha256.Sum256(hd.GetPublicKey())
	tailHash := hex.EncodeToString(h[:])
	return fmt.Sprintf("%d:%d-%d:%d:%d:%s", ad.GetTimestamp(), ad.GetNonce1(), ad.GetNonce2(), busID, int32(hd.GetType()), tailHash)
}

// MakeAccessDataPlaintextFromCustomCommand builds the signed plaintext string.
//
// C++ rules:
//
//	"<timestamp>:<nonce1>-<nonce2>:<bus_id>:<sha256_hex(concat(commands.arg))>"
func MakeAccessDataPlaintextFromCustomCommand(busID types.BusIdType, ad *protocol.AccessData, cs *protocol.CustomCommandData) string {
	// Concatenate all command args.
	total := 0
	commands := cs.GetCommands()
	for _, item := range commands {
		total += len(item.GetArg())
	}

	buf := make([]byte, 0, total)
	for _, item := range commands {
		buf = append(buf, item.GetArg()...)
	}

	h := sha256.Sum256(buf)
	return fmt.Sprintf("%d:%d-%d:%d:%s", ad.GetTimestamp(), ad.GetNonce1(), ad.GetNonce2(), busID, hex.EncodeToString(h[:]))
}

// CalculateAccessDataSignature computes the signature for the given plaintext.
//
// It mirrors C++:
//
//	signature = HMAC-SHA256(plaintext, access_token)
//
// and truncates access_token length to 32868 bytes.
func CalculateAccessDataSignature(_ *protocol.AccessData, accessToken []byte, plaintext string) []byte {
	token := accessToken
	if len(token) > 32868 {
		token = token[:32868]
	}

	mac := hmac.New(sha256.New, token)
	_, _ = mac.Write([]byte(plaintext))
	return mac.Sum(nil)
}

// ---- Message wrapper ----

// Message is an alias to types.Message for convenience.
type Message = types.Message

// NewMessage creates a new empty Message.
func NewMessage() *Message {
	return types.NewMessage()
}

// ---- callback declarations (stubs) ----

// HandlerFn matches the C++ handler signature shape.
type HandlerFn func(n types.Node, conn types.Connection, msg *Message, status int, errcode error_code.ErrorType) error_code.ErrorType

// Receiver declares message receive handlers.
// Implementations can be provided by higher-level packages.
type DispatchHandleSet struct {
	fns [types.MessageBodyTypeNodeMax + 1]HandlerFn
}

var _defaultReceiver *DispatchHandleSet = nil

func getHandleSet() *DispatchHandleSet {
	return _defaultReceiver
}

func buildHandleSet() *DispatchHandleSet {
	ret := &DispatchHandleSet{
		fns: [types.MessageBodyTypeNodeMax + 1]HandlerFn{},
	}
	ret.fns[types.MessageBodyTypeDataTransformReq] = onRecvDataTransferReq
	ret.fns[types.MessageBodyTypeDataTransformRsp] = onRecvDataTransferRsp
	ret.fns[types.MessageBodyTypeCustomCommandReq] = onRecvCustomCommandReq
	ret.fns[types.MessageBodyTypeCustomCommandRsp] = onRecvCustomCommandRsp
	ret.fns[types.MessageBodyTypeNodeRegisterReq] = onRecvNodeRegisterReq
	ret.fns[types.MessageBodyTypeNodeRegisterRsp] = onRecvNodeRegisterRsp
	ret.fns[types.MessageBodyTypeNodePingReq] = onRecvNodePing
	ret.fns[types.MessageBodyTypeNodePongRsp] = onRecvNodePong
	ret.fns[types.MessageBodyTypeHandshakeConfirm] = onRecvHandshakeConfirm
	return ret
}

func DispatchMessage(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	receiver := getHandleSet()
	if receiver == nil {
		panic("Initialize handle set failed!")
	}

	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	head := m.GetHead()
	if head == nil {
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	bodyType := m.GetBodyType()
	if bodyType < 0 || bodyType > types.MessageBodyTypeNodeMax {
		return error_code.EN_ATBUS_ERR_ATNODE_INVALID_MSG
	}

	fn := receiver.fns[bodyType]
	if fn == nil {
		return error_code.EN_ATBUS_ERR_ATNODE_INVALID_MSG
	}

	n.AddStatisticDispatchTimes()
	return fn(n, conn, m, status, errcode)
}

func SendMessage(n types.Node, conn types.Connection, m *types.Message) error_code.ErrorType {
	if lu.IsNil(n) || lu.IsNil(conn) || lu.IsNil(m) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	head := m.MutableHead()

	buf, errCode := PackMessage(conn.GetConnectionContext(), m, n.GetProtocolVersion(), int(n.GetConfigure().MessageSize))
	if errCode != error_code.EN_ATBUS_ERR_SUCCESS {
		n.LogDebug(getConnectionBinding(conn), conn, m, "package message failed")
		n.LogError(getConnectionBinding(conn), conn, int(errCode), errCode, "package message failed")
		return errCode
	}

	if buf == nil {
		n.LogDebug(getConnectionBinding(conn), conn, m, "package message failed with unknown error")
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_INNER), error_code.EN_ATBUS_ERR_INNER, "package message failed with unknown error")
		return error_code.EN_ATBUS_ERR_INNER
	}

	usedSize := buf.Used()
	n.LogDebug(getConnectionBinding(conn), conn, m, fmt.Sprintf(
		"node send message(version=%v, command=%v, type=%v, sequence=%v, result_code=%v, length=%v)",
		head.GetVersion(), GetBodyName(int(m.GetBodyType())), head.GetType(), head.GetSequence(),
		head.GetResultCode(), usedSize,
	))

	return conn.Push(buf.UsedSpan())
}

func SendPing(n types.Node, conn types.Connection, messageSequence uint64) error_code.ErrorType {
	if lu.IsNil(n) || lu.IsNil(conn) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	message := types.NewMessage()
	head := message.MutableHead()
	body := message.MutableBody().MutableNodePingReq()

	head.Version = n.GetProtocolVersion()
	head.Sequence = messageSequence
	head.SourceBusId = uint64(n.GetId())

	body.TimePoint = n.GetTimerTick().UnixMicro()
	if conn.CheckFlag(types.ConnectionFlag_ClientMode) &&
		conn.GetConnectionContext().GetCryptoSelectAlgorithm() != protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE {
		refreshInterval := n.GetConfigure().CryptoKeyRefreshInterval
		if refreshInterval.Nanoseconds() > 0 &&
			// 考虑时间回退
			(n.GetTimerTick().After(conn.GetConnectionContext().GetHandshakeStartTime().Add(refreshInterval)) ||
				n.GetTimerTick().Add(refreshInterval).Before(conn.GetConnectionContext().GetHandshakeStartTime())) {
			resultCode := conn.GetConnectionContext().HandshakeGenerateSelfKey(body.GetCryptoHandshake().GetSequence())
			if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
				n.LogError(getConnectionBinding(conn), conn, int(resultCode), resultCode, "node send ping but handshake refresh secret failed")
			} else {
				resultCode = conn.GetConnectionContext().HandshakeWriteSelfPublicKey(
					body.MutableCryptoHandshake(),
					n.GetConfigure().CryptoAllowAlgorithms,
				)
				if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
					n.LogError(getConnectionBinding(conn), conn, int(resultCode), resultCode, "node send ping but write public key failed")
				}
			}
		}
	}

	return SendMessage(n, conn, message)
}

func SendRegister(bodyType types.MessageBodyType, n types.Node, conn types.Connection, errcode error_code.ErrorType, messageSequence uint64) error_code.ErrorType {
	if bodyType != types.MessageBodyTypeNodeRegisterReq && bodyType != types.MessageBodyTypeNodeRegisterRsp {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if lu.IsNil(n) || lu.IsNil(conn) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	m := types.NewMessage()
	head := m.MutableHead()
	var body *protocol.RegisterData
	if bodyType == types.MessageBodyTypeNodeRegisterReq {
		body = m.MutableBody().MutableNodeRegisterReq()
	} else {
		body = m.MutableBody().MutableNodeRegisterRsp()
	}

	head.Version = n.GetProtocolVersion()
	head.Sequence = messageSequence
	head.SourceBusId = uint64(n.GetId())
	head.ResultCode = int32(errcode)

	if body.Channels == nil {
		body.Channels = make([]*protocol.ChannelData, 0, len(n.GetListenList()))
	}

	for _, addr := range n.GetListenList() {
		ch := body.AddChannels()
		if ch == nil {
			continue
		}
		ch.Address = addr.GetAddress()
	}

	body.BusId = uint64(n.GetId())
	body.Pid = int32(n.GetPid())
	body.Hostname = n.GetHostname()

	selfEp := n.GetSelfEndpoint()
	if selfEp == nil {
		n.LogError(selfEp, nil, int(error_code.EN_ATBUS_ERR_NOT_INITED), error_code.EN_ATBUS_ERR_NOT_INITED, "node not inited")
		return error_code.EN_ATBUS_ERR_NOT_INITED
	}

	body.Flags = uint32(selfEp.GetFlags())

	for scheme := range getSupportedChannelSchemes() {
		if body.SupportedChannelSchema == nil {
			body.SupportedChannelSchema = make([]string, 0, len(getSupportedChannelSchemes()))
		}
		body.SupportedChannelSchema = append(body.SupportedChannelSchema, scheme)
	}
	body.HashCode = selfEp.GetHashCode()

	// 打包本地支持的加密算法信息,需要配置允许+本地实现接入
	for _, algo := range n.GetConfigure().CompressionAllowAlgorithms {
		if !types.IsCompressionAlgorithmSupported(algo) {
			continue
		}
		if body.SupportedCompressionAlgorithm == nil {
			body.SupportedCompressionAlgorithm = make([]protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE, 0, len(n.GetConfigure().CompressionAllowAlgorithms))
		}
		body.SupportedCompressionAlgorithm = append(body.SupportedCompressionAlgorithm, algo)
	}

	// 客户端创建己方密钥对，发给对方协商
	if bodyType == types.MessageBodyTypeNodeRegisterReq &&
		conn.CheckFlag(types.ConnectionFlag_ClientMode) &&
		conn.GetConnectionContext().GetCryptoKeyExchangeAlgorithm() != protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE {
		resultCode := conn.GetConnectionContext().HandshakeGenerateSelfKey(body.GetCryptoHandshake().GetSequence())
		if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
			n.LogError(getConnectionBinding(conn), conn, int(resultCode), resultCode, "node send register but handshake generate self key failed")
			return resultCode
		}
	}

	// crypto handshake data
	if errcode == error_code.EN_ATBUS_ERR_SUCCESS &&
		conn.GetConnectionContext().GetCryptoKeyExchangeAlgorithm() != protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE {
		var allowedAlgorithms []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE
		if conn.CheckFlag(types.ConnectionFlag_ServerMode) {
			// 服务端发回协商结果即可
			allowedAlgorithms = []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{conn.GetConnectionContext().GetCryptoSelectAlgorithm()}
		} else {
			// 客户端要上报可用的算法列表
			allowedAlgorithms = n.GetConfigure().CryptoAllowAlgorithms
		}
		conn.GetConnectionContext().HandshakeWriteSelfPublicKey(
			body.MutableCryptoHandshake(),
			allowedAlgorithms,
		)
	}

	GenerateAccessData(body.MutableAccessKey(), n.GetId(), rand.Uint64(), rand.Uint64(), n.GetConfigure().AccessTokens, body.GetCryptoHandshake())

	return SendMessage(n, conn, m)
}

func SendTransferResponse(n types.Node, m *Message, errcode error_code.ErrorType) error_code.ErrorType {
	bodyType := m.GetBodyType()
	if bodyType != types.MessageBodyTypeNodeRegisterReq && bodyType != types.MessageBodyTypeNodeRegisterRsp {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	var fwdData *protocol.ForwardData
	if bodyType == types.MessageBodyTypeDataTransformReq {
		fwdData = m.MutableBody().MutableDataTransformReq()
		m.MutableBody().MessageType = &protocol.MessageBody_DataTransformRsp{
			DataTransformRsp: fwdData,
		}
	} else {
		fwdData = m.MutableBody().MutableDataTransformRsp()
	}

	selfId := uint64(n.GetId())
	originFrom := fwdData.GetFrom()
	originTo := fwdData.GetTo()

	head := m.MutableHead()
	head.ResultCode = int32(errcode)
	head.SourceBusId = uint64(n.GetId())

	fwdData.From = originTo
	fwdData.To = originFrom

	if len(fwdData.GetRouter()) == 0 || fwdData.GetRouter()[len(fwdData.GetRouter())-1] != selfId {
		// 去掉第一跳路由
		fwdData.AppendRouter(selfId)
	}

	ret, ep, conn := n.SendCtrlMessage(types.BusIdType(originFrom), m, nil)
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		n.LogError(ep, conn, int(ret), ret, fmt.Sprintf("send control message to %v failed", originFrom))
	}

	return ret
}

func SendCustomCommandResponse(n types.Node, conn types.Connection, rspData [][]byte, t int32,
	errcode error_code.ErrorType, messageSequence uint64, fromBusId types.BusIdType,
) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	m := types.NewMessage()

	head := m.MutableHead()
	body := m.MutableBody().MutableCustomCommandRsp()

	selfId := uint64(n.GetId())

	head.Version = n.GetProtocolVersion()
	head.Sequence = messageSequence
	head.SourceBusId = selfId
	head.ResultCode = int32(errcode)
	head.Type = t

	body.From = selfId
	if body.Commands == nil {
		body.Commands = make([]*protocol.CustomCommandArgv, 0, len(rspData))
	}

	for _, arg := range rspData {
		cmd := body.AddCommands()
		if cmd == nil {
			continue
		}
		cmd.Arg = arg
	}

	GenerateAccessDataForCustomCommand(body.MutableAccessKey(), n.GetId(), rand.Uint64(), rand.Uint64(), n.GetConfigure().AccessTokens, body)

	var ret error_code.ErrorType
	if lu.IsNil(conn) {
		ret, _, _ = n.SendCtrlMessage(fromBusId, m, nil)
	} else {
		ret = SendMessage(n, conn, m)
	}

	return ret
}

func forwardDataMessage(n types.Node, m *Message, fromServerId types.BusIdType, toServerId types.BusIdType) (error_code.ErrorType, types.Endpoint) {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS, nil
	}

	if m == nil || 0 == toServerId || n.GetId() == toServerId {
		n.LogError(nil, nil, int(error_code.EN_ATBUS_ERR_PARAMS), error_code.EN_ATBUS_ERR_PARAMS, "invalid parameters")
		return error_code.EN_ATBUS_ERR_PARAMS, nil
	}

	opts := types.CreateNodeGetPeerOptions()
	opts.SetBlacklist([]types.BusIdType{fromServerId})
	ret, targetEp, targetConn, _ := n.GetPeerChannel(toServerId, func(from types.Endpoint, to types.Endpoint) types.Connection {
		return from.GetDataConnection(to, true)
	}, opts)

	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		n.LogError(targetEp, targetConn, int(ret), ret, fmt.Sprintf("get peer channel to %v failed", toServerId))
		return ret, targetEp
	}

	if lu.IsNil(targetConn) || lu.IsNil(targetEp) {
		n.LogError(targetEp, targetConn, int(error_code.EN_ATBUS_ERR_CONNECTION_NOT_FOUND),
			error_code.EN_ATBUS_ERR_CONNECTION_NOT_FOUND, "no available target")
		return error_code.EN_ATBUS_ERR_CONNECTION_NOT_FOUND, targetEp
	}

	if fromServerId == 0 && targetEp.GetId() == fromServerId {
		n.LogError(targetEp, targetConn, int(error_code.EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME),
			error_code.EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME, "same source and target")
		return error_code.EN_ATBUS_ERR_ATNODE_SRC_DST_IS_SAME, targetEp
	}

	head := m.MutableHead()
	head.SourceBusId = uint64(n.GetId())

	return SendMessage(n, targetConn, m), targetEp
}

func onRecvDataTransferReq(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	bodyType := m.GetBodyType()
	if bodyType != types.MessageBodyTypeDataTransformReq &&
		bodyType != types.MessageBodyTypeDataTransformRsp {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "invalid body type")
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	var fwdData *protocol.ForwardData
	if bodyType == types.MessageBodyTypeDataTransformReq {
		fwdData = m.GetBody().GetDataTransformReq()
	} else {
		fwdData = m.GetBody().GetDataTransformRsp()
	}

	head := m.GetHead()
	connIsNil := lu.IsNil(conn)
	if head == nil || fwdData == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "no head or no custom_command_data")

		if !connIsNil {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	if head.GetVersion() < n.GetProtocolMinimalVersion() {
		if !connIsNil {
			conn.AddStatisticFault()
		}

		return SendTransferResponse(n, m, error_code.EN_ATBUS_ERR_UNSUPPORTED_VERSION)
	}

	if connIsNil && head.GetSourceBusId() != uint64(n.GetId()) {
		n.LogError(nil, nil, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "no connection")
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	if !connIsNil && conn.GetStatus() != types.ConnectionState_Connected {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_NOT_READY), error_code.EN_ATBUS_ERR_NOT_READY, "connection not ready")
		return error_code.EN_ATBUS_ERR_NOT_READY
	}

	// all transfer message must be send by a verified connection, there is no need to check access token again

	// dispatch message
	if fwdData.GetTo() == uint64(n.GetId()) {
		ep := getConnectionBinding(conn)
		n.LogDebug(ep, conn, m, "node receive data length = %v", len(fwdData.GetContent()))
		n.OnReceiveData(ep, conn, m, fwdData.GetContent())

		if fwdData.GetFlags()&uint32(protocol.ATBUS_FORWARD_DATA_FLAG_TYPE_FORWARD_DATA_FLAG_REQUIRE_RSP) > 0 {
			return SendTransferResponse(n, m, error_code.EN_ATBUS_ERR_SUCCESS)
		}

		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	// router message
	routerSize := len(fwdData.GetRouter())
	if routerSize >= int(n.GetConfigure().TTL) {
		return SendTransferResponse(n, m, error_code.EN_ATBUS_ERR_ATNODE_TTL)
	}

	directFromBusId := head.GetSourceBusId()
	fwdData.AppendRouter(uint64(n.GetId()))
	ret, toEp := forwardDataMessage(n, m, types.BusIdType(directFromBusId), types.BusIdType(fwdData.GetTo()))

	// 如果forward的流程失败，且尝试的目标endpoint是邻居/远端节点，
	// 可能是连接未完成，但是endpoint已建立，可以再尝试通过直接上游节点转发。
	ret = func(res error_code.ErrorType) error_code.ErrorType {
		if res == error_code.EN_ATBUS_ERR_SUCCESS {
			return res
		}

		if toEp != nil {
			releation, _ := n.GetTopologyRelation(toEp.GetId())
			if releation != types.TopologyRelationType_OtherUpstreamPeer &&
				releation != types.TopologyRelationType_SameUpstreamPeer {
				return res
			}
		}

		upstreamEp := n.GetUpstreamEndpoint()
		if upstreamEp == nil || (toEp != nil && upstreamEp.GetId() == toEp.GetId()) || directFromBusId == uint64(upstreamEp.GetId()) {
			return res
		}

		res, _ = forwardDataMessage(n, m, types.BusIdType(directFromBusId), upstreamEp.GetId())
		return res
	}(ret)

	// 只有失败或请求方要求回包，才下发通知，类似ICMP协议
	if ret != error_code.EN_ATBUS_ERR_SUCCESS || (fwdData.GetFlags()&uint32(protocol.ATBUS_FORWARD_DATA_FLAG_TYPE_FORWARD_DATA_FLAG_REQUIRE_RSP) > 0) {
		ret = SendTransferResponse(n, m, ret)
	}

	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		n.LogError(getConnectionBinding(conn), conn, int(ret), ret, "forward data message failed")
	}

	return ret
}

func onRecvDataTransferRsp(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	bodyType := m.GetBodyType()
	if bodyType != types.MessageBodyTypeDataTransformReq &&
		bodyType != types.MessageBodyTypeDataTransformRsp {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "invalid body type")
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	var fwdData *protocol.ForwardData
	if bodyType == types.MessageBodyTypeDataTransformReq {
		fwdData = m.GetBody().GetDataTransformReq()
	} else {
		fwdData = m.GetBody().GetDataTransformRsp()
	}

	head := m.GetHead()
	connIsNil := lu.IsNil(conn)
	if head == nil || fwdData == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "no head or no custom_command_data")

		if !connIsNil {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	if connIsNil && head.GetSourceBusId() != uint64(n.GetId()) {
		n.LogError(nil, nil, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "no connection")
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	if !connIsNil && conn.GetStatus() != types.ConnectionState_Connected {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_NOT_READY), error_code.EN_ATBUS_ERR_NOT_READY, "connection not ready")
		return error_code.EN_ATBUS_ERR_NOT_READY
	}

	// all transfer message must be send by a verified connect, there is no need to check access token again

	// dispatch message
	if fwdData.GetTo() == uint64(n.GetId()) {
		ep := getConnectionBinding(conn)
		if head.GetResultCode() < 0 {
			n.LogError(ep, conn, int(head.GetResultCode()), error_code.ErrorType(head.GetResultCode()), "receive data transfer response with error code")
		}

		n.OnReceiveForwardResponse(ep, conn, m)
		return error_code.EN_ATBUS_ERR_SUCCESS
	}

	// 检查如果发送目标不是来源，则转发失败消息
	ret, _ := forwardDataMessage(n, m, types.BusIdType(head.GetSourceBusId()), types.BusIdType(fwdData.GetTo()))
	return ret
}

func onRecvCustomCommandReq(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	bodyType := m.GetBodyType()
	if bodyType != types.MessageBodyTypeCustomCommandReq &&
		bodyType != types.MessageBodyTypeCustomCommandRsp {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "invalid body type")
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	var cmdData *protocol.CustomCommandData
	if bodyType == types.MessageBodyTypeCustomCommandReq {
		cmdData = m.GetBody().GetCustomCommandReq()
	} else {
		cmdData = m.GetBody().GetCustomCommandRsp()
	}

	head := m.GetHead()
	if head == nil || cmdData == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "no head or no custom_command_data")
		if !lu.IsNil(conn) {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	// check version
	if head.GetVersion() < n.GetProtocolMinimalVersion() {
		rspData := make([][]byte, 0, 1)
		rspData = append(rspData, []byte("Access Deny - Unsupported Version"))
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_UNSUPPORTED_VERSION), error_code.EN_ATBUS_ERR_UNSUPPORTED_VERSION,
			fmt.Sprintf("custom command version %v is lower than minimal supported %v", head.GetVersion(), n.GetProtocolMinimalVersion()),
		)

		if !lu.IsNil(conn) {
			conn.AddStatisticFault()
		}
		return SendCustomCommandResponse(n, conn, rspData, head.GetType(), error_code.EN_ATBUS_ERR_UNSUPPORTED_VERSION,
			head.GetSequence(), types.BusIdType(head.GetSourceBusId()),
		)
	}

	// message from self has no connection
	if lu.IsNil(conn) && cmdData.GetFrom() != uint64(n.GetId()) {
		n.LogError(nil, nil, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA,
			"message from self but from bus id not match",
		)
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	// Check access token
	if !n.CheckAccessHash(cmdData.GetAccessKey(), MakeAccessDataPlaintextFromCustomCommand(
		types.BusIdType(cmdData.GetFrom()),
		cmdData.GetAccessKey(),
		cmdData,
	), conn) {
		rspData := make([][]byte, 0, 1)
		rspData = append(rspData, []byte("Access Deny - Invalid Token"))
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_ACCESS_DENY), error_code.EN_ATBUS_ERR_ACCESS_DENY,
			fmt.Sprintf("access deny from %v, invalid token", cmdData.GetFrom()),
		)
		return SendCustomCommandResponse(n, conn, rspData, head.GetType(), error_code.EN_ATBUS_ERR_ACCESS_DENY,
			head.GetSequence(), types.BusIdType(cmdData.GetFrom()),
		)
	}

	cmdArgs := make([][]byte, 0, len(cmdData.GetCommands()))
	for _, cmd := range cmdData.GetCommands() {
		cmdArgs = append(cmdArgs, cmd.GetArg())
	}

	ret, rspData := n.OnCustomCommandRequest(getConnectionBinding(conn), conn, types.BusIdType(cmdData.GetFrom()), cmdArgs)
	if (conn != nil && conn.IsRunning() && conn.CheckFlag(types.ConnectionFlag_RegFd)) || n.GetId() == types.BusIdType(cmdData.GetFrom()) {
		ret = SendCustomCommandResponse(n, conn, rspData, head.GetType(), error_code.EN_ATBUS_ERR_SUCCESS,
			head.GetSequence(), types.BusIdType(cmdData.GetFrom()),
		)
	}

	return ret
}

func onRecvCustomCommandRsp(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	bodyType := m.GetBodyType()
	if bodyType != types.MessageBodyTypeCustomCommandReq &&
		bodyType != types.MessageBodyTypeCustomCommandRsp {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "invalid body type")
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	var cmdData *protocol.CustomCommandData
	if bodyType == types.MessageBodyTypeCustomCommandReq {
		cmdData = m.GetBody().GetCustomCommandReq()
	} else {
		cmdData = m.GetBody().GetCustomCommandRsp()
	}

	head := m.GetHead()
	if head == nil || cmdData == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "no head or no custom_command_data")
		if !lu.IsNil(conn) {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	// message from self has no connection
	if lu.IsNil(conn) && cmdData.GetFrom() != uint64(n.GetId()) {
		n.LogError(nil, nil, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA,
			"message from self but from bus id not match",
		)
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	cmdArgs := make([][]byte, 0, len(cmdData.GetCommands()))
	for _, cmd := range cmdData.GetCommands() {
		cmdArgs = append(cmdArgs, cmd.GetArg())
	}

	return n.OnCustomCommandResponse(getConnectionBinding(conn), conn, types.BusIdType(cmdData.GetFrom()), cmdArgs, head.GetSequence())
}

func acceptNodeRegistrationStepMakeEndpoint(n types.Node, conn types.Connection, m *Message, regData *protocol.RegisterData) (error_code.ErrorType, types.Endpoint) {
	if conn.IsConnected() {
		ep := conn.GetBinding()
		if ep == nil || ep.GetId() != types.BusIdType(regData.GetBusId()) {
			n.LogError(ep, conn, int(error_code.EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH), error_code.EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH, "bus id not match")
			conn.Reset()
			return error_code.EN_ATBUS_ERR_ATNODE_BUS_ID_NOT_MATCH, ep
		}

		ep.UpdateHashCode(regData.GetHashCode())
		n.LogInfo(ep, conn, "connection already connected receive register again")
		return error_code.EN_ATBUS_ERR_SUCCESS, ep
	}

	// 临时连接不需要绑定endpoint，握手成功即可发送指令消息
	if 0 == regData.GetBusId() {
		conn.SetTemporary()
		n.LogInfo(nil, conn, "connection set temporary")
		return error_code.EN_ATBUS_ERR_SUCCESS, nil
	}

	// 老端点新增连接不需要创建新连接
	hostName := regData.GetHostname()
	ep := n.GetEndpoint(types.BusIdType(regData.GetBusId()))

	// 已有节点，增加连接
	if !lu.IsNil(ep) {
		// 检测机器名和进程号必须一致,自己是临时节点则不需要检查
		if 0 != n.GetId() && (ep.GetPid() != regData.GetPid() || ep.GetHostname() != hostName) {
			n.LogError(ep, conn, int(error_code.EN_ATBUS_ERR_ATNODE_ID_CONFLICT), error_code.EN_ATBUS_ERR_ATNODE_ID_CONFLICT,
				fmt.Sprintf("bus id %v already exists with different hostname or pid (old: %v/%v, new: %v/%v)",
					regData.GetBusId(), ep.GetHostname(), ep.GetPid(), hostName, regData.GetPid(),
				),
			)
			conn.Reset()
			return error_code.EN_ATBUS_ERR_ATNODE_ID_CONFLICT, ep
		} else if !ep.AddConnection(conn, conn.CheckFlag(types.ConnectionFlag_AccessShareHost)) {
			// 有共享物理机限制的连接只能加为数据节点（一般就是内存通道或者共享内存通道）
			n.LogError(ep, conn, int(error_code.EN_ATBUS_ERR_ATNODE_NO_CONNECTION),
				error_code.EN_ATBUS_ERR_ATNODE_NO_CONNECTION, "add connection to endpoint failed")
			conn.Reset()
			return error_code.EN_ATBUS_ERR_ATNODE_NO_CONNECTION, ep
		}

		ep.UpdateHashCode(regData.GetHashCode())
		n.LogDebug(ep, conn, m, "connection added to existed endpoint")
		return error_code.EN_ATBUS_ERR_SUCCESS, ep
	}

	// 创建新端点
	ep = n.CreateEndpoint(types.BusIdType(regData.GetBusId()), hostName, int(regData.GetPid()))
	if lu.IsNil(ep) {
		n.LogDebug(nil, conn, m, "malloc endpoint failed")
		n.LogError(nil, conn, int(error_code.EN_ATBUS_ERR_MALLOC),
			error_code.EN_ATBUS_ERR_MALLOC, "malloc endpoint failed")
		conn.Reset()
		return error_code.EN_ATBUS_ERR_MALLOC, nil
	}
	ep.UpdateHashCode(regData.GetHashCode())

	ret := n.AddEndpoint(ep)
	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		n.LogError(ep, conn, int(ret), ret, "add endpoint to node failed")
		conn.Reset()
		return ret, ep
	}

	n.LogDebug(ep, conn, nil, "node add a new endpoint success")
	// 新的endpoint第一个连接为控制连接，后续的为数据连接
	ep.AddConnection(conn, false)
	return ret, ep
}

func acceptNodeRegistrationStepUpdateEndpoint(n types.Node, ep types.Endpoint, conn types.Connection, regData *protocol.RegisterData) {
	supportedSchemes := make([]string, 0, len(regData.GetSupportedChannelSchema()))
	for _, scheme := range regData.GetSupportedChannelSchema() {
		if _, ok := getSupportedChannelSchemes()[scheme]; ok {
			supportedSchemes = append(supportedSchemes, strings.ToLower(scheme))
		}
	}

	ep.UpdateSupportSchemes(supportedSchemes)

	// update listen addresses
	ep.ClearListenAddress()
	for _, ch := range regData.GetChannels() {
		if ch == nil || ch.GetAddress() == "" {
			continue
		}
		ep.AddListenAddress(ch.GetAddress())
	}
}

func calculateChannelAddressPriority(addr string, isSameHost bool, isSameProcess bool) int {
	ret := 0
	addrLower := strings.ToLower(addr)
	if isSameProcess && channel_utility.IsLocalProcessAddress(addr) {
		ret += 0x20
	}

	if isSameHost && channel_utility.IsLocalHostAddress(addr) {
		ret += 0x10

		if strings.HasPrefix(addrLower, "mem:") || strings.HasPrefix(addrLower, "shm:") {
			ret += 0x08
		}
	}

	if channel_utility.IsDuplexAddress(addr) {
		ret += 0x02

		if strings.HasPrefix(addrLower, "unix:") || strings.HasPrefix(addrLower, "pipe:") {
			ret += 0x04
		}
	}

	if addr != "" {
		ret += 0x01
	}

	return ret
}

func acceptNodeRegistrationStepDataChannel(n types.Node, ep types.Endpoint, conn types.Connection, regData *protocol.RegisterData) error_code.ErrorType {
	// 如果双方一边有IOS通道，另一边没有，则没有的连接有的
	// 如果双方都有IOS通道，则CLIENT端连接SERVER端
	isSameHost := ep.GetHostname() == n.GetHostname()
	isSameProcess := isSameHost && int(ep.GetPid()) == n.GetPid()
	hasDataConnectionSuccess := false

	// 如果SERVER端判定出对方可能会通过双工通道再连接自己一次，就不用反向发起数据连接。
	if conn.CheckFlag(types.ConnectionFlag_ServerMode) {
		endpointSelectPriority := 0
		endpointSelectAddress := ""
		for _, addr := range n.GetListenList() {
			if !ep.IsSchemeSupported(addr.GetScheme()) {
				continue
			}

			_, exists := getSupportedChannelSchemes()[addr.GetScheme()]
			if !exists {
				continue
			}

			checkPriority := calculateChannelAddressPriority(addr.GetAddress(), isSameHost, isSameProcess)
			if checkPriority > endpointSelectPriority {
				endpointSelectPriority = checkPriority
				endpointSelectAddress = addr.GetAddress()
			}
		}

		if endpointSelectAddress != "" && channel_utility.IsDuplexAddress(endpointSelectAddress) {
			return error_code.EN_ATBUS_ERR_SUCCESS
		}
	}

	// io_stream channel only need one connection
	// 按优先级尝试连接对方的地址列表，建立数据连接
	addressPriorityList := make([]struct {
		priority int
		address  string
	}, 0, len(regData.GetChannels()))
	for _, ch := range regData.GetChannels() {
		if ch == nil || ch.GetAddress() == "" {
			continue
		}

		if !isSameProcess && channel_utility.IsLocalProcessAddress(ch.GetAddress()) {
			continue
		}

		if !isSameHost && channel_utility.IsLocalHostAddress(ch.GetAddress()) {
			continue
		}

		addr, _ := channel_utility.MakeAddress(ch.GetAddress())
		_, exists := getSupportedChannelSchemes()[addr.GetScheme()]
		if !exists {
			continue
		}

		addressPriorityList = append(addressPriorityList, struct {
			priority int
			address  string
		}{
			priority: calculateChannelAddressPriority(ch.GetAddress(), isSameHost, isSameProcess),
			address:  ch.GetAddress(),
		})
	}

	slices.SortFunc(addressPriorityList, func(a, b struct {
		priority int
		address  string
	},
	) int {
		if a.priority == b.priority {
			if a.address < b.address {
				return -1
			} else if a.address > b.address {
				return 1
			}
			return 0
		}

		return b.priority - a.priority
	})

	ret := error_code.EN_ATBUS_ERR_SUCCESS
	for _, addrInfo := range addressPriorityList {
		// if n is not a temporary node, connect to other nodes
		if hasDataConnectionSuccess {
			break
		}

		res := n.ConnectWithEndpoint(addrInfo.address, ep)
		if res != error_code.EN_ATBUS_ERR_SUCCESS {
			n.LogError(ep, conn, int(res), res, fmt.Sprintf("connect to address %s failed", addrInfo.address))
			ret = res
			continue
		}

		hasDataConnectionSuccess = true
	}

	// 如果新创建的endpoint没有成功进行的数据连接，加入检测列表，下一帧释放
	if !hasDataConnectionSuccess {
		// 如果不能被对方连接，进入GC检测列表
		n.AddEndpointGcList(ep)
	} else {
		ret = error_code.EN_ATBUS_ERR_SUCCESS
	}

	return ret
}

func acceptNodeRegistration(n types.Node, conn types.Connection, m *Message, regData *protocol.RegisterData) (error_code.ErrorType, types.Endpoint) {
	if lu.IsNil(n) || lu.IsNil(conn) || m == nil || regData == nil {
		return error_code.EN_ATBUS_ERR_PARAMS, nil
	}

	ret, ep := acceptNodeRegistrationStepMakeEndpoint(n, conn, m, regData)

	// 临时连接不需要创建数据通道
	if ret != error_code.EN_ATBUS_ERR_SUCCESS || conn.CheckFlag(types.ConnectionFlag_Temporary) || lu.IsNil(ep) {
		return ret, ep
	}

	if n.GetId() == 0 || ep.GetId() == 0 {
		return ret, ep
	}

	acceptNodeRegistrationStepUpdateEndpoint(n, ep, conn, regData)

	// 如果已经有数据通道了，就不需要再创建了
	if ep.GetDataConnectionCount(false) > 0 {
		return ret, ep
	}

	ret = acceptNodeRegistrationStepDataChannel(n, ep, conn, regData)
	return ret, ep
}

func onRecvNodeRegisterReq(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	bodyType := m.GetBodyType()
	if bodyType != types.MessageBodyTypeNodeRegisterReq &&
		bodyType != types.MessageBodyTypeNodeRegisterRsp {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "invalid body type")
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	var regData *protocol.RegisterData
	if bodyType == types.MessageBodyTypeNodeRegisterReq {
		regData = m.GetBody().GetNodeRegisterReq()
	} else {
		regData = m.GetBody().GetNodeRegisterRsp()
	}

	head := m.GetHead()
	connIsNil := lu.IsNil(conn)
	if connIsNil || head == nil || regData == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "no head or no node_register_data")
		if !connIsNil {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	// check version
	if head.GetVersion() < n.GetProtocolMinimalVersion() {
		conn.AddStatisticFault()
		ret := SendRegister(types.MessageBodyTypeNodeRegisterRsp, n, conn, error_code.EN_ATBUS_ERR_UNSUPPORTED_VERSION,
			head.GetSequence())
		if ret != error_code.EN_ATBUS_ERR_SUCCESS {
			n.LogError(getConnectionBinding(conn), conn, int(ret), ret, fmt.Sprintf("send unsupported version %d register response failed", head.GetVersion()))
			if !connIsNil {
				conn.Reset()
			}
			return ret
		}
	}

	responseCode, ep := func() (error_code.ErrorType, types.Endpoint) {
		// Check access token
		checkAccessToken := n.CheckAccessHash(regData.GetAccessKey(), MakeAccessDataPlaintextFromHandshake(
			types.BusIdType(regData.GetBusId()),
			regData.GetAccessKey(),
			regData.GetCryptoHandshake(),
		), conn)
		if !checkAccessToken {
			return error_code.EN_ATBUS_ERR_ACCESS_DENY, nil
		}

		// 处理握手协商数据
		if conn.CheckFlag(types.ConnectionFlag_ServerMode) && regData.GetCryptoHandshake().GetSequence() != 0 &&
			regData.GetCryptoHandshake().GetType() != protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE {
			// 服务端读取对方公钥，创建己方密钥对，发给对方协商。自己可以完成协商过程
			resultCode := conn.GetConnectionContext().HandshakeGenerateSelfKey(regData.GetCryptoHandshake().GetSequence())
			if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
				return resultCode, nil
			}

			resultCode = conn.GetConnectionContext().HandshakeReadPeerKey(regData.GetCryptoHandshake(),
				n.GetConfigure().CryptoAllowAlgorithms, true)
			if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
				return resultCode, nil
			}
		}

		// 更新对端加密算法支持
		resultCode := conn.GetConnectionContext().UpdateCompressionAlgorithm(regData.GetSupportedCompressionAlgorithm())
		if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
			return resultCode, nil
		}

		return acceptNodeRegistration(n, conn, m, regData)
	}()

	if responseCode != error_code.EN_ATBUS_ERR_SUCCESS {
		n.LogError(getConnectionBinding(conn), conn, int(responseCode), responseCode, fmt.Sprintf("access deny from %v, invalid token", regData.GetBusId()))
		ret := SendRegister(types.MessageBodyTypeNodeRegisterRsp, n, conn, responseCode, head.GetSequence())
		if ret != error_code.EN_ATBUS_ERR_SUCCESS {
			n.LogError(getConnectionBinding(conn), conn, int(ret), ret, fmt.Sprintf("send register response to %d failed", regData.GetBusId()))
			conn.Reset()
			return ret
		}
	}

	// 仅fd连接发回注册回包，否则忽略（内存和共享内存通道为单工通道）
	if conn.CheckFlag(types.ConnectionFlag_RegFd) {
		ret := SendRegister(types.MessageBodyTypeNodeRegisterRsp, n, conn, responseCode, head.GetSequence())
		if ret != error_code.EN_ATBUS_ERR_SUCCESS {
			n.LogError(getConnectionBinding(conn), conn, int(ret), ret, fmt.Sprintf("send register response to %d failed", regData.GetBusId()))
			conn.Reset()
		} else {
			// 注册事件触发
			n.OnRegister(ep, conn, error_code.EN_ATBUS_ERR_SUCCESS)
		}

		return ret
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func onRecvNodeRegisterRsp(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	bodyType := m.GetBodyType()
	if bodyType != types.MessageBodyTypeNodeRegisterReq &&
		bodyType != types.MessageBodyTypeNodeRegisterRsp {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "invalid body type")
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	var regData *protocol.RegisterData
	if bodyType == types.MessageBodyTypeNodeRegisterReq {
		regData = m.GetBody().GetNodeRegisterReq()
	} else {
		regData = m.GetBody().GetNodeRegisterRsp()
	}

	head := m.GetHead()
	connIsNil := lu.IsNil(conn)
	if connIsNil || head == nil || regData == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA, "no head or no node_register_data")
		if !connIsNil {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	ret := error_code.ErrorType(head.GetResultCode())
	ret, ep := func(resultCode error_code.ErrorType) (error_code.ErrorType, types.Endpoint) {
		// Check access token
		checkAccessToken := n.CheckAccessHash(regData.GetAccessKey(), MakeAccessDataPlaintextFromHandshake(
			types.BusIdType(regData.GetBusId()),
			regData.GetAccessKey(),
			regData.GetCryptoHandshake(),
		), conn)
		if !checkAccessToken {
			return error_code.EN_ATBUS_ERR_ACCESS_DENY, nil
		}

		if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
			return resultCode, nil
		}

		// 处理握手协商数据
		if conn.CheckFlag(types.ConnectionFlag_ClientMode) && regData.GetCryptoHandshake().GetSequence() != 0 &&
			regData.GetCryptoHandshake().GetType() != protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE {
			resultCode = conn.GetConnectionContext().HandshakeReadPeerKey(regData.GetCryptoHandshake(),
				n.GetConfigure().CryptoAllowAlgorithms, false)
			if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
				return resultCode, nil
			}

			// 发送 handshake_confirm 通知对端完成密钥切换
			SendHandshakeConfirm(n, conn, regData.GetCryptoHandshake().GetSequence())
		}

		resultCode = conn.GetConnectionContext().UpdateCompressionAlgorithm(regData.GetSupportedCompressionAlgorithm())
		if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
			return resultCode, nil
		}

		// 先刷新拓扑关系
		if n.GetId() != 0 && regData.GetBusId() != 0 && conn.GetAddress().GetAddress() == n.GetConfigure().UpstreamAddress {
			n.SetTopologyUpstream(types.BusIdType(regData.GetBusId()))
		}
		return acceptNodeRegistration(n, conn, m, regData)
	}(ret)

	if ret != error_code.EN_ATBUS_ERR_SUCCESS {
		if lu.IsNil(ep) {
			ep = conn.GetBinding()
		}
		if !lu.IsNil(ep) {
			n.AddEndpointGcList(ep)
		}

		// 如果是父节点回的错误注册包，且未被激活过，则要关闭进程
		if conn.GetAddress().GetAddress() == n.GetConfigure().UpstreamAddress && !n.CheckFlag(types.NodeFlag_Actived) {
			n.LogDebug(ep, conn, m, "node register to parent node failed, shutdown")
			n.FatalShutdown(ep, conn, ret, fmt.Errorf("node register to parent node failed, shutdown"))
		} else {
			n.LogError(ep, conn, int(ret), ret, "node register failed")
		}

		n.OnRegister(ep, conn, ret)
		conn.Reset()
		return ret
	}

	// 注册事件触发
	n.OnRegister(ep, conn, ret)

	if types.NodeState_ConnectingUpstream == n.GetState() {
		// 父节点返回的rsp成功则可以上线
		// 这时候父节点的endpoint不一定初始化完毕
		upstreamEp := n.GetUpstreamEndpoint()
		if upstreamEp != nil && upstreamEp.GetId() == types.BusIdType(regData.GetBusId()) {
			// 父节点先注册完成
			n.OnUpstreamRegisterDone()
			n.OnActived()
		}
	}

	return ret
}

func onRecvNodePing(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	body := m.GetBody().GetNodePingReq()
	if body == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA,
			"no head or no node_ping_req",
		)
		if !lu.IsNil(conn) {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	head := m.GetHead()

	retCode := error_code.EN_ATBUS_ERR_SUCCESS
	if head.GetVersion() < n.GetProtocolMinimalVersion() {
		retCode = error_code.EN_ATBUS_ERR_UNSUPPORTED_VERSION
	}

	if !lu.IsNil(conn) {
		ep := getConnectionBinding(conn)

		// 处理握手协商数据
		withHandshake := conn.CheckFlag(types.ConnectionFlag_ServerMode) && !lu.IsNil(ep) &&
			body.GetCryptoHandshake().GetSequence() != 0 &&
			body.GetCryptoHandshake().GetType() != protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE
		if withHandshake {
			// 服务端读取对方公钥，创建己方密钥对，发给对方协商。自己可以完成协商过程
			retCode = conn.GetConnectionContext().HandshakeGenerateSelfKey(body.GetCryptoHandshake().GetSequence())
			if retCode == error_code.EN_ATBUS_ERR_SUCCESS {
				retCode = conn.GetConnectionContext().HandshakeReadPeerKey(body.GetCryptoHandshake(),
					n.GetConfigure().CryptoAllowAlgorithms, true)
			}
			if retCode != error_code.EN_ATBUS_ERR_SUCCESS {
				n.LogError(ep, conn, int(retCode), retCode, "ping handshake refresh secret failed")
				withHandshake = false
			}
		}

		n.OnPing(ep, m, body)

		// 下发协商换密钥数据
		{
			responseM := types.NewMessage()
			head := responseM.MutableHead()
			pongBody := responseM.MutableBody().MutableNodePongRsp()

			head.Version = n.GetProtocolVersion()
			head.Sequence = head.GetSequence()
			head.SourceBusId = uint64(n.GetId())
			head.Type = head.GetType()
			head.ResultCode = int32(retCode)

			pongBody.TimePoint = body.GetTimePoint()

			if withHandshake {
				allowedAlgorithms := []protocol.ATBUS_CRYPTO_ALGORITHM_TYPE{conn.GetConnectionContext().GetCryptoSelectAlgorithm()}
				conn.GetConnectionContext().HandshakeWriteSelfPublicKey(
					pongBody.MutableCryptoHandshake(),
					allowedAlgorithms,
				)
			}

			return SendMessage(n, conn, responseM)
		}
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func onRecvNodePong(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	body := m.GetBody().GetNodePongRsp()
	if body == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA,
			fmt.Sprintf("node recv node_ping from %v but without node_pong_rsp", m.GetHead().GetSourceBusId()),
		)
		if !lu.IsNil(conn) {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	if !lu.IsNil(conn) {
		// 处理握手协商数据
		if conn.CheckFlag(types.ConnectionFlag_ClientMode) &&
			conn.GetConnectionContext().GetCryptoSelectAlgorithm() != protocol.ATBUS_CRYPTO_ALGORITHM_TYPE_ATBUS_CRYPTO_ALGORITHM_NONE &&
			body.GetCryptoHandshake().GetSequence() != 0 &&
			body.GetCryptoHandshake().GetType() != protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_NONE {
			resultCode := conn.GetConnectionContext().HandshakeReadPeerKey(body.GetCryptoHandshake(),
				n.GetConfigure().CryptoAllowAlgorithms, false)

			if resultCode != error_code.EN_ATBUS_ERR_SUCCESS {
				n.LogError(getConnectionBinding(conn), conn, int(resultCode), resultCode,
					fmt.Sprintf("node recv node_pong from %v handshake refresh secret failed", m.GetHead().GetSourceBusId()),
				)
			} else {
				// 发送 handshake_confirm 通知对端完成密钥切换
				SendHandshakeConfirm(n, conn, body.GetCryptoHandshake().GetSequence())
			}
		}

		ep := getConnectionBinding(conn)
		n.OnPong(ep, m, body)
		if !lu.IsNil(ep) && m.GetHead().GetSequence() == ep.GetStatisticUnfinishedPing() {
			ep.SetStatisticUnfinishedPing(0)

			offset := n.GetTimerTick().UnixMicro() - body.GetTimePoint()
			ep.SetStatisticPingDelay(time.Microsecond*time.Duration(offset), n.GetTimerTick())
		}
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

func onRecvHandshakeConfirm(n types.Node, conn types.Connection, m *Message, status int, errcode error_code.ErrorType) error_code.ErrorType {
	if lu.IsNil(n) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	body := m.GetBody().GetHandshakeConfirm()
	if body == nil {
		n.LogError(getConnectionBinding(conn), conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA,
			fmt.Sprintf("node recv handshake_confirm from %v but without handshake_confirm data", m.GetHead().GetSourceBusId()),
		)
		if !lu.IsNil(conn) {
			conn.AddStatisticFault()
		}
		return error_code.EN_ATBUS_ERR_BAD_DATA
	}

	// 必须已经注册完成的 connection 才能处理 handshake_confirm
	if !lu.IsNil(conn) {
		if !conn.CheckFlag(types.ConnectionFlag_Temporary) && lu.IsNil(conn.GetBinding()) {
			n.LogError(nil, conn, int(error_code.EN_ATBUS_ERR_BAD_DATA), error_code.EN_ATBUS_ERR_BAD_DATA,
				fmt.Sprintf("node recv handshake_confirm from %v but connection has no endpoint", m.GetHead().GetSourceBusId()),
			)
			conn.Reset()
			return error_code.EN_ATBUS_ERR_BAD_DATA
		}

		conn.GetConnectionContext().ConfirmHandshake(body.GetSequence())
	}

	return error_code.EN_ATBUS_ERR_SUCCESS
}

// SendHandshakeConfirm sends a handshake_confirm message to notify the peer
// that the cipher renegotiation is complete on this end.
func SendHandshakeConfirm(n types.Node, conn types.Connection, sequence uint64) error_code.ErrorType {
	if lu.IsNil(n) || lu.IsNil(conn) {
		return error_code.EN_ATBUS_ERR_PARAMS
	}

	responseM := types.NewMessage()
	head := responseM.MutableHead()
	confirmBody := responseM.MutableBody().MutableHandshakeConfirm()

	head.Version = n.GetProtocolVersion()
	head.Sequence = conn.GetConnectionContext().GetNextSequence()
	head.SourceBusId = uint64(n.GetId())
	confirmBody.Sequence = sequence

	return SendMessage(n, conn, responseM)
}

func init() {
	_defaultReceiver = buildHandleSet()
}
