# libatbus-go 功能补全计划

> 基于 C++ libatbus 仓库 (`atsf4g-co/atframework/libatbus`) 与 Go 仓库 (`atsf4g-go/atframework/libatbus-go`) 的逐项对比。
> 正确性是第一优先级。Go 版本不需要内存通道 (`mem://`) 和共享内存通道 (`shm://`)。

---

## 一、整体对比概要

| 模块 | C++ | Go | 实现完成度 | 备注 |
|------|-----|-----|-----------|------|
| **atbus_node** | atbus_node.h/cpp | impl/atbus_node.go + types/atbus_node.go | ~95% | 少量缺失，见下 |
| **atbus_endpoint** | atbus_endpoint.h/cpp | impl/atbus_endpoint.go + types/atbus_endpoint.go | ~98% | 基本完整 |
| **atbus_connection** | atbus_connection.h/cpp | impl/atbus_connection.go + types/atbus_connection.go | ~95% | 缺少部分状态回调 |
| **atbus_connection_context** | atbus_connection_context.h/cpp | impl/atbus_connection_context.go + types/atbus_connection_context.go | ~95% | XXTEA 缺失(P1)，缺 IsCompressionAlgorithmSupported |
| **atbus_message_handler** | atbus_message_handler.h/cpp | message_handle/atbus_message_handler.go | ~97% | 功能一致，有 2 个 Bug 待修 |
| **atbus_topology** | atbus_topology.h/cpp | impl/atbus_topology.go + types/atbus_topology.go | ~100% | 接口完全对齐 |
| **channel_io_stream** | channel_io_stream.cpp | channel/io_stream/ | ~100% | 完整 |
| **channel_utility** | channel_utility.cpp | channel/utility/ | ~100% | 完整 |
| **buffer** | buffer.h/cpp | buffer/ | ~100% | 完整 |
| **error_code** | libatbus_error.h/cpp | error_code/ | ~100% | 完整 |
| **protocol** | libatbus_protocol.proto | protocol/ | ~100% | 完整，proto 一致 |
| **channel_mem** | channel_mem.cpp | N/A | 不需要 | Go 不实现 |
| **channel_shm** | channel_shm.cpp | N/A | 不需要 | Go 不实现 |

---

## 二、功能缺失详细分析

### 2.1 atbus_node 功能缺失

#### 2.1.1 `start_conf_t` / `StartWithConfigure` 对齐检查

- C++ `start()` 接受 `start_conf_t` 参数，包含 `timer_sec`/`timer_usec` 时间戳
- Go 已有 `StartWithConfigure(conf *StartConfigure)`，`StartConfigure` 包含 `TimerTimepoint`
- **状态**: ✅ 已实现

#### 2.1.2 `default_conf()` 默认值对齐

- C++ `node::default_conf()` 设置默认参数
- Go `SetDefaultNodeConfigure()` 存在
- **任务**: 逐项对比默认值是否一致
  - `loop_times`: C++ = 128 → Go 需确认
  - `ttl`: C++ = 16 → Go 需确认
  - `backlog`: C++ = 256 → Go 需确认
  - `first_idle_timeout`: C++ = 30s → Go 需确认
  - `ping_interval`: C++ = 60s → Go 需确认
  - `retry_interval`: C++ = 3s → Go 需确认
  - `fault_tolerant`: C++ = 3 → Go 需确认
  - `access_token_max_number`: C++ = 5 → Go 需确认
  - `crypto_key_refresh_interval`: C++ = 1h → Go 需确认
  - `message_size`: C++ = 262144 (256KB) → Go 需确认
  - `send_buffer_size`: C++ = 2MB → Go 需确认
  - `receive_buffer_size`: C++ = 8MB → Go 需确认

#### 2.1.3 `send_data` 返回值差异

- C++ `send_data()` 返回 `int` (error code)
- Go `SendData()` 返回 `ErrorType`
- **状态**: ✅ 语义等价

#### 2.1.4 `get_peer_channel()` 签名差异

- C++ 版本:
  ```cpp
  ATBUS_ERROR_TYPE get_peer_channel(bus_id_t tid, get_connection_fn_t fn,
      endpoint **ep_out, connection **conn_out,
      topology_peer::ptr_t *next_hop_peer, const get_peer_options_t &options)
  ```
- Go 版本:
  ```go
  GetPeerChannel(tid BusIdType, fn func(from, to Endpoint) Connection,
      options *NodeGetPeerOptions) (ErrorType, Endpoint, Connection, TopologyPeer)
  ```
- **状态**: ✅ 语义等价（Go 用多返回值代替出参）

#### 2.1.5 Shutdown 与 FatalShutdown 行为

- C++ `reset()` 直接重置节点
- Go 有 `Shutdown(reason)` 和 `FatalShutdown(ep, conn, code, err)`
- **任务**: 确认 Go 的 `Shutdown` 是否触发 `on_node_down` 回调、等待 `on_node_down` 返回非零时延迟 reset（与 C++ 行为一致）

#### 2.1.6 Poll 行为

- C++ `poll()` 调用底层 `uv_run(loop, UV_RUN_NOWAIT)`
- Go `Poll()` 基于 goroutine 模型，不需要显式 poll
- **状态**: ✅ by design。Go 的 goroutine 模型使 poll 无需与 C++ 完全对等，当前行为是设计预期

### 2.2 atbus_endpoint 功能缺失

#### 2.2.1 `watch()` 资源持有

- C++ `endpoint::watch()` 返回 `strong_rc_ptr` 防止被 GC
- Go 没有手动引用计数概念，依赖 GC
- **状态**: ✅ 不需要实现（Go GC 管理生命周期）

#### 2.2.2 统计数据方法对齐

C++ endpoint 统计方法与 Go 对比:

| C++ 方法 | Go 方法 | 状态 |
|----------|---------|------|
| `get_stat_push_start_times()` | `GetStatisticPushStartTimes()` | ✅ |
| `get_stat_push_start_size()` | `GetStatisticPushStartSize()` | ✅ |
| `get_stat_push_success_times()` | `GetStatisticPushSuccessTimes()` | ✅ |
| `get_stat_push_success_size()` | `GetStatisticPushSuccessSize()` | ✅ |
| `get_stat_push_failed_times()` | `GetStatisticPushFailedTimes()` | ✅ |
| `get_stat_push_failed_size()` | `GetStatisticPushFailedSize()` | ✅ |
| `get_stat_pull_times()` | `GetStatisticPullTimes()` | ✅ |
| `get_stat_pull_size()` | `GetStatisticPullSize()` | ✅ |
| `get_stat_created_time()` | `GetStatisticCreatedTime()` | ✅ |

### 2.3 atbus_connection 功能缺失

#### 2.3.1 `proc()` 方法对齐

- C++ `connection::proc(node, time_point)` 处理一帧
- Go `Connection.Proc()` 不接受时间参数
- **状态**: ✅ by design。Go 不支持 mem/shm 通道，无需通过 proc 传递时间戳驱动底层通道轮询

#### 2.3.2 `unpack()` 静态方法

- C++ `connection::unpack(conn, message, data)` 反序列化
- Go 通过 `ConnectionContext.UnpackMessage()` 实现
- **状态**: ✅ 等价

#### 2.3.3 内存/共享内存通道回调

- C++ 有 `mem_proc_fn`, `mem_free_fn`, `mem_push_fn`, `shm_proc_fn`, `shm_free_fn`, `shm_push_fn`
- **状态**: 不需要（Go 版本不支持 `mem://` / `shm://` 通道）

### 2.4 atbus_connection_context 功能缺失

#### 2.4.1 XXTEA 加密

- C++ 支持 `ATBUS_CRYPTO_ALGORITHM_XXTEA`
- Go 未实现 XXTEA
- **优先级**: P1 — 跨语言互通必需
- **实现计划**:

  **Step 1: 在 `atframe-utils-go/algorithm/crypto/` 中实现 XXTEA**

  参考 C++ 实现 `atframe_utils/src/algorithm/xxtea.cpp` 和 `atframe_utils/include/algorithm/xxtea.h`。

  算法要点:
  - 128-bit 密钥 (`[4]uint32`)
  - 大端字节序 (big-endian) 读写 `uint32`
  - Delta = `0x9e3779b9`
  - 加密轮数: `rounds = 6 + 52/n` (n = block 数量, 最少 2)
  - 最小 8 字节块 (2 × uint32)
  - 输入自动 pad 到 4 字节对齐: `real_len = ((ilen - 1) | 0x03) + 1`
  - MX 混合函数: `(((z >> 5 ^ y << 2) + (y >> 3 ^ z << 4)) ^ ((sum ^ y) + (key[((p & 3) ^ e)] ^ z)))`

  API 设计 (对齐 C++):
  ```go
  // atframe-utils-go/algorithm/crypto/xxtea.go
  type XXTEAKey struct {
      Data [4]uint32
  }

  func XXTEASetup(key *XXTEAKey, keyBuf []byte) error     // 从 16 字节 buffer 初始化 key
  func XXTEAEncrypt(key *XXTEAKey, data []byte) ([]byte, error)  // 就地或返回新 buffer
  func XXTEADecrypt(key *XXTEAKey, data []byte) ([]byte, error)
  ```

  **Step 2: 单元测试**

  Reference C++ test vectors from `atframe_utils/test/case/xxtea_test.cpp`:

  | Key (hex) | Plaintext (hex) | Ciphertext (hex) |
  |-----------|-----------------|------------------|
  | 00010203 04050607 08090a0b 0c0d0e0f | 01234567 89abcdef | 96c3b4fa a72ea28c |
  | 00010203 04050607 08090a0b 0c0d0e0f | 00000000 00000000 | 946a4137 5b06e676 |
  | 00010203 04050607 08090a0b 0c0d0e0f | ffffffff ffffffff | 3e009e37 07669fc1 |
  | 00000000 00000000 00000000 00000000 | 01234567 89abcdef | 720e83a3 9d415508 |
  | 00000000 00000000 00000000 00000000 | 00000000 00000000 | e14ed316 1e89baa5 |
  | 00000000 00000000 00000000 00000000 | ffffffff ffffffff | 42d06235 0d2c6b05 |

  测试用例:
  - `TestXXTEABasic`: 使用上述 6 组向量做加密/解密往返验证
  - `TestXXTEAInputOutput`: 使用独立输入/输出 buffer 验证
  - `TestXXTEAEdgeCases`: 空输入、奇数长度输入 pad 验证

  **Step 3: 集成到 libatbus-go**

  在 `impl/atbus_connection_context.go` 的加密分支中添加 `ATBUS_CRYPTO_ALGORITHM_XXTEA` case，
  调用 `atframe-utils-go/algorithm/crypto/xxtea.go` 中的实现。

#### 2.4.2 `internal_padding_temporary_buffer_block()` 对齐

- C++ 有缓冲区对齐计算
- Go 通过 `StaticBufferBlock` 内部处理
- **状态**: ✅ 逻辑等价

#### 2.4.3 ChaCha20（非 AEAD）

- C++ 支持 `ATBUS_CRYPTO_ALGORITHM_CHACHA20`（=31，非 AEAD 纯流密码）
- Go 需确认是否支持纯 ChaCha20（不带 Poly1305）
- **任务**: 验证 Go 的 `golang.org/x/crypto` 是否支持纯 ChaCha20 流加密，并确认跨语言兼容性

#### 2.4.4 `is_compression_algorithm_supported()` 公开方法

- C++ 有静态查询方法 `is_compression_algorithm_supported(algo)`
- Go 缺少独立的公开查询方法
- **任务** (P1): 添加公开函数 `IsCompressionAlgorithmSupported(algo ATBUS_COMPRESSION_ALGORITHM_TYPE) bool`，
  放在 `impl/atbus_connection_context.go` 中，与 `UpdateCompressionAlgorithm()` 配套使用。
  实现逻辑：根据编译期可用的压缩库返回 true/false（Zstd, LZ4, Snappy, Zlib）

### 2.5 atbus_topology 功能对比

> 经重新核查 C++ `atbus_topology.h` 后更正。前一版本此节有多处接口签名错误。

#### 2.5.1 TopologyRegistry 接口完全对照

C++ `topology_registry` 公开方法与 Go `TopologyRegistry` 一一对应:

| C++ 方法 | Go 方法 | 状态 |
|----------|---------|------|
| `static ptr_t create()` | `CreateTopologyRegistry()` | ✅ |
| `topology_peer::ptr_t get_peer(bus_id_t) const` | `GetPeer(busId) TopologyPeer` | ✅ |
| `void remove_peer(bus_id_t)` | `RemovePeer(targetBusId)` | ✅ |
| `bool update_peer(bus_id_t targetId, bus_id_t upstreamId, topology_data::ptr_t data)` | `UpdatePeer(targetBusId, upstreamBusId, data) bool` | ✅ 签名一致 |
| `topology_relation_type get_relation(bus_id_t from, bus_id_t to, topology_peer::ptr_t *nextHop) const` | `GetRelation(from, to) (TopologyRelationType, TopologyPeer)` | ✅ Go 用多返回值代替出参 |
| `bool foreach_peer(fn) const` | `ForeachPeer(fn) bool` | ✅ |
| `static bool check_policy(const topology_policy_rule&, const topology_data&, const topology_data&)` | `CheckPolicy(rule, fromData, toData) bool` | ✅ C++ 是 static，Go 是 instance method，功能等价 |

**结论**: TopologyRegistry C++/Go 接口完全对齐，无缺失方法。
前一版本中提到的 `update_upstream(from, to)`、`clear_upstream(from)`、`check_cycle_safe(from, to)` **在 C++ 中不存在**（这些功能均已整合到 `update_peer` 内部）。

#### 2.5.2 TopologyPeer 接口对照

| C++ 方法 | Go 方法 | 状态 |
|----------|---------|------|
| `bus_id_t get_bus_id() const` | `GetBusId() BusIdType` | ✅ |
| `const topology_peer::ptr_t& get_upstream() const` | `GetUpstream() TopologyPeer` | ✅ |
| `const topology_data& get_topology_data() const` | `GetTopologyData() *TopologyData` | ✅ |
| `bool contains_downstream(bus_id_t) const` | `ContainsDownstream(busId) bool` | ✅ |
| `bool foreach_downstream(fn) const` | `ForeachDownstream(fn) bool` | ✅ |

**结论**: TopologyPeer C++/Go 接口完全对齐。

#### 2.5.3 Proactive vs Passive Peer 行为

- C++ 内部有 `set_proactively_added(bool)` / `get_proactively_added()` 控制孤立 peer 清理策略
- Go 内部也有 `setProactivelyAdded(v bool)` / `getProactivelyAdded() bool`
- **状态**: ✅ 内部行为已对齐

### 2.6 atbus_message_handler 功能对比

> 经重新核查，C++ `atbus_message_handler.h/cpp` 对应 Go `message_handle/atbus_message_handler.go`。
> 两者功能**基本一致**，但发现 2 个 Go 侧 Bug。

#### 2.6.0 Go 侧已发现 Bug

**Bug 1: `getConnectionBinding()` 无限递归**

```go
// 当前代码 (错误):
func getConnectionBinding(conn types.Connection) types.Endpoint {
    return getConnectionBinding(conn) // 无限递归!
}
// 应修正为:
func getConnectionBinding(conn types.Connection) types.Endpoint {
    if conn == nil {
        return nil
    }
    return conn.GetBinding()
}
```

**Bug 2: `SendTransferResponse()` 错误的 body type 检查**

```go
// 当前代码检查的是 kNodeRegisterReq/Rsp，应该检查 kDataTransformReq/Rsp
```

- **任务** (P0): 修复上述 2 个 Bug

#### 2.6.1 函数逐项对照

| C++ 函数 | Go 函数 | 状态 |
|----------|---------|------|
| `unpack_message(connCtx, target, data, max_body_size)` | `UnpackMessage(connCtx, data, maxBodySize)` | ✅ |
| `pack_message(connCtx, m, version, random, max_size)` | `PackMessage(connCtx, msg, version, maxSize)` | ✅ Go 省略 random_engine |
| `get_body_name(body_case)` | `GetBodyName(bodyCase)` | ✅ |
| `generate_access_data(...)` | `GenerateAccessData(...)` / `GenerateAccessDataWithTimestamp(...)` | ✅ |
| `make_access_data_plaintext(...)` [crypto overload] | `MakeAccessDataPlaintextFromHandshake(...)` | ✅ |
| `make_access_data_plaintext(...)` [custom_cmd overload] | `MakeAccessDataPlaintextFromCustomCommand(...)` | ✅ |
| `calculate_access_data_signature(...)` | `CalculateAccessDataSignature(...)` | ✅ |
| `send_message(n, conn, msg)` | `SendMessage(n, conn, m)` | ✅ |
| `send_ping(n, conn, seq)` | `SendPing(n, conn, seq)` | ✅ |
| `send_handshake_confirm(n, conn, seq)` | `SendHandshakeConfirm(n, conn, seq)` | ✅ |
| `send_register(id, n, conn, ret, seq)` | `SendRegister(bodyType, n, conn, err, seq)` | ✅ |
| `send_transfer_response(n, m, ret)` | `SendTransferResponse(n, m, err)` | ⚠️ Bug 2 |
| `send_custom_command_response(...)` | `SendCustomCommandResponse(...)` | ✅ |
| `dispatch_message(n, conn, m, status, err)` | `DispatchMessage(n, conn, m, status, err)` | ✅ |
| `on_recv_data_transfer_req(...)` | `onRecvDataTransferReq(...)` | ✅ |
| `on_recv_data_transfer_rsp(...)` | `onRecvDataTransferRsp(...)` | ✅ |
| `on_recv_custom_command_req(...)` | `onRecvCustomCommandReq(...)` | ✅ |
| `on_recv_custom_command_rsp(...)` | `onRecvCustomCommandRsp(...)` | ✅ |
| `on_recv_node_register_req(...)` | `onRecvNodeRegisterReq(...)` | ✅ |
| `on_recv_node_register_rsp(...)` | `onRecvNodeRegisterRsp(...)` | ✅ |
| `on_recv_node_ping(...)` | `onRecvNodePing(...)` | ✅ |
| `on_recv_node_pong(...)` | `onRecvNodePong(...)` | ✅ |
| `on_recv_handshake_confirm(...)` | `onRecvHandshakeConfirm(...)` | ✅ |
| `accept_node_registration(...)` | `acceptNodeRegistration(...)` | ✅ |
| `calculate_channel_address_priority(...)` | `calculateChannelAddressPriority(...)` | ✅ |
| `_get_body_type_name(cmd)` | 无独立函数 | ✅ Go 通过 `GetBodyName()` + 反射实现等价功能 |

**结论**: message_handle 包与 C++ message_handler **功能一致**，除 2 个 Bug 外无缺失。

---

## 三、跨语言互通保障计划

### 3.1 协议兼容性

| 项目 | 状态 | 行动 |
|------|------|------|
| Protobuf .proto 文件一致 | ✅ 一致 | 持续同步 |
| Varint 编码 | ✅ 算法一致 | 已有跨语言测试 |
| 帧格式 `[CRC32(4B)][varint(payload_len)][payload]` | ✅ 一致 | 已有跨语言测试 |
| 消息格式 `[varint(head_len)][protobuf_head][body]` | ✅ 一致 | 已有跨语言测试 |
| Access Token HMAC-SHA256 签名 | ✅ 一致 | 已有跨语言测试 |
| Access Token Plaintext 格式 | ✅ 一致 | 已有跨语言测试 |

### 3.2 加密算法兼容性

| 算法 | C++ | Go | 跨语言互通 |
|------|-----|-----|-----------|
| AES-128-CBC | ✅ | ✅ | ✅ PKCS#7 padding |
| AES-192-CBC | ✅ | ✅ | ✅ |
| AES-256-CBC | ✅ | ✅ | ✅ |
| AES-128-GCM | ✅ | ✅ | ✅ AEAD |
| AES-192-GCM | ✅ | ✅ | ✅ |
| AES-256-GCM | ✅ | ✅ | ✅ |
| ChaCha20 | ✅ | ⚠️ 需验证 | ⚠️ 需验证纯 ChaCha20 |
| ChaCha20-Poly1305 IETF | ✅ | ✅ | ✅ AEAD |
| XChaCha20-Poly1305 IETF | ✅ | ✅ | ✅ AEAD |
| XXTEA | ✅ | ❌ 未实现 | ❌ 不互通 |

### 3.3 密钥交换兼容性

| 算法 | C++ | Go | 互通 |
|------|-----|-----|------|
| X25519 | ✅ | ✅ | ✅ |
| SECP256R1 (P-256) | ✅ | ✅ | ✅ |
| SECP384R1 (P-384) | ✅ | ✅ | ✅ |
| SECP521R1 (P-521) | ✅ | ✅ | ✅ |

### 3.4 压缩算法兼容性

| 算法 | C++ | Go | 互通 |
|------|-----|-----|------|
| Zstd | ✅ | ✅ | ✅ |
| LZ4 | ✅ | ✅ | ✅ |
| Snappy | ✅ | ✅ | ✅ |
| Zlib | ✅ | ✅ | ✅ |

### 3.5 KDF 兼容性

| 算法 | C++ | Go | 互通 |
|------|-----|-----|------|
| HKDF-SHA256 | ✅ | ✅ | ✅ 需验证 salt/info 参数一致 |

### 3.6 跨语言测试数据

- C++ 通过 `atbus_connection_context_crosslang_generator.cpp` 生成二进制测试向量
- C++ 通过 `atbus_access_data_crosslang_generator.cpp` 生成签名测试向量
- **任务**: Go 测试需加载这些测试向量并验证解析/校验结果一致

---

## 四、功能补全具体任务

### P0 (关键 - 影响正确性和跨语言互通)

| 编号 | 任务 | 说明 | 涉及文件 |
|------|------|------|---------|
| P0-1 | 默认配置值对齐 | 逐项对比 `default_conf()` 与 `SetDefaultNodeConfigure()` 所有默认值，修正差异 | `types/atbus_common_types.go` |
| P0-2 | HKDF 参数对齐验证 | 确认 HKDF-SHA256 的 salt, info 参数与 C++ 完全一致 | `impl/atbus_connection_context.go` |
| P0-3 | 消息路由 TTL 行为对齐 | 确认 TTL 递减、检查、错误返回与 C++ 一致 | `impl/atbus_node.go`, `message_handle/` |
| P0-4 | Access Token 时间容差 | 确认 ±300 秒容差与 C++ 一致 | `message_handle/atbus_message_handler.go` |
| P0-5 | 注册流程 access token 校验 | 确认注册请求/响应中 access token 的编码、校验逻辑完整 | `message_handle/atbus_message_handler.go` |
| P0-6 | Key Renegotiation 时序 | 确认 cipher staging / confirm 流程时序正确（server: stage until confirm; client: immediate） | `impl/atbus_connection_context.go` |
| P0-7 | 控制消息不加密/不压缩 | 确认 register, ping/pong, handshake_confirm 消息绕过加密和压缩 | `impl/atbus_connection_context.go`, `message_handle/` |
| P0-8 | 消息大小限制一致 | 检查 `message_size` 限制在 pack/unpack 中的执行方式与 C++ 一致 | `impl/atbus_connection_context.go` |
| P0-9 | 修复 getConnectionBinding 无限递归 | `getConnectionBinding()` 调用自身导致栈溢出，需改为 `conn.GetBinding()` | `message_handle/atbus_message_handler.go` |
| P0-10 | 修复 SendTransferResponse body type 检查 | 当前检查 `kNodeRegisterReq/Rsp`，应检查 `kDataTransformReq/Rsp` | `message_handle/atbus_message_handler.go` |

### P1 (重要 - 功能完整性)

| 编号 | 任务 | 说明 | 涉及文件 |
|------|------|------|---------|
| P1-1 | Shutdown 回调行为 | 确认 `Shutdown(reason)` 调用 `on_node_down` 回调，并在回调返回非零时延迟 reset | `impl/atbus_node.go` |
| P1-2 | 连接超时行为 | 确认 `first_idle_timeout` 超时后触发 `on_invalid_connection` 回调并断开 | `impl/atbus_node.go` |
| P1-3 | Fault Tolerant 行为 | 确认错误次数超过 `fault_tolerant` 阈值后自动断开连接/端点 | `impl/atbus_node.go`, `impl/atbus_endpoint.go` |
| P1-4 | 上游掉线与重连 | 确认上游断开后状态变为 `LostUpstream`/`ConnectingUpstream`，自动重试连接 | `impl/atbus_node.go` |
| P1-5 | Topology on_topology_update_upstream | 确认上游拓扑变更回调触发正确 | `impl/atbus_node.go` |
| P1-6 | NodeGetPeerOptions.blacklist | 确认黑名单过滤在路由中正确生效 | `impl/atbus_node.go` |
| P1-7 | send_data RequireResponse 标志 | 确认 `FORWARD_DATA_FLAG_REQUIRE_RSP` 在发送失败或接收方处理后自动响应 | `message_handle/` |
| P1-8 | Custom Command 的 access token 校验 | 确认 custom command 消息也做 access token 校验（与 register 一致） | `message_handle/` |
| P1-9 | Protocol Version 校验 | 确认注册时检查对端协议版本，低于 `protocol_minimal_version` 时拒绝 | `message_handle/` |
| P1-10 | 连接冲突检测 | 确认 ID 冲突时的处理行为与 C++ 一致（拒绝注册 or 踢出旧连接） | `message_handle/`, `impl/atbus_node.go` |
| P1-11 | XXTEA 加密实现 | 在 `atframe-utils-go/algorithm/crypto/` 实现 XXTEA（参考 C++ xxtea.cpp），然后集成到 libatbus-go connection_context 加密分支 | `atframe-utils-go/algorithm/crypto/xxtea.go`, `impl/atbus_connection_context.go` |
| P1-12 | IsCompressionAlgorithmSupported | 添加公开函数查询压缩算法支持能力 | `impl/atbus_connection_context.go` |

### P2 (改进 - 功能增强)

| 编号 | 任务 | 说明 | 涉及文件 |
|------|------|------|---------|
| P2-1 | ChaCha20 纯流密码 | 验证/实现纯 ChaCha20（非 AEAD）以保证与 C++ 互通 | `impl/atbus_connection_context.go` |
| P2-2 | 支持 overwrite_listen_path | 确认 Unix socket / named pipe 监听时是否支持覆盖已存在文件 | `channel/io_stream/` |

### P3 (低优先级 - 可选)

| 编号 | 任务 | 说明 | 涉及文件 |
|------|------|------|---------|
| P3-1 | Replay Attack 防护 | 实现 nonce 追踪防重放攻击 | `impl/atbus_node.go` |

---

## 五、单元测试补全计划

> 本章基于 C++ 测试用例 (118+ cases) 逐项映射 Go 测试，确保覆盖所有场景。
> 排除 `mem://` 和 `shm://` 相关用例。
> 不破坏测试目的和核心时序。

### 5.1 已有 Go 测试覆盖情况

| C++ 测试文件 | 测试数量 | Go 等价测试 | 覆盖状态 |
|-------------|---------|------------|---------|
| `libatbus_error_test.cpp` (6) | 6 | `error_code/libatbus_error_test.go` (2) | ⚠️ 部分覆盖 |
| `buffer_test.cpp` (11) | 11 | `buffer/*_test.go` (~40) | ✅ 超额覆盖 |
| `channel_io_stream_tcp_test.cpp` (8) | 8 | `channel/io_stream/*_test.go` (~20) | ✅ 覆盖 |
| `channel_io_stream_unix_test.cpp` (5) | 5 | `channel/io_stream/*_test.go` | ✅ 覆盖 |
| `channel_mem_test.cpp` (6) | 6 | N/A | ❌ 不需要 |
| `channel_shm_test.cpp` (6) | 6 | N/A | ❌ 不需要 |
| `atbus_topology_test.cpp` (9) | 9 | `impl/atbus_topology_test.go` (9) | ✅ 覆盖 |
| `atbus_endpoint_test.cpp` (5) | 5 | 无对应文件 | ❌ 需新增 |
| `atbus_connection_context_test.cpp` (37) | 37 | `impl/atbus_connection_context_test.go` (~26) | ⚠️ 部分覆盖 |
| `atbus_message_handler_test.cpp` (16) | 16 | `message_handle/atbus_message_handler_test.go` (17) | ✅ 覆盖 |
| `atbus_node_setup_test.cpp` (3) | 3 | 无对应文件 | ❌ 需新增 |
| `atbus_node_reg_test.cpp` (22) | 22 | 无对应文件 | ❌ 需新增 |
| `atbus_node_msg_test.cpp` (24) | 24 | 无对应文件 | ❌ 需新增 |
| `atbus_node_relationship_test.cpp` (3) | 3 | 无对应文件 | ❌ 需新增 |
| `atbus_connection_context_crosslang_generator.cpp` (10) | 10 | 无对应文件 | ❌ 需新增 |
| `atbus_access_data_crosslang_generator.cpp` (8) | 8 | 无对应文件 | ⚠️ 部分覆盖 |
| (C++ `xxtea_test.cpp`) | 2 | 无对应文件 | ❌ 需新增 (atframe-utils-go) |

### 5.2 需新增的 Go 测试文件

#### 5.2.0 `atframe-utils-go/algorithm/crypto/xxtea_test.go` (新文件)

> XXTEA 算法独立测试，放在 atframe-utils-go 仓库中。

| 序号 | 测试名 | 对应 C++ | 说明 |
|------|--------|---------|------|
| 1 | `TestXXTEABasic` | `xxtea_test::basic` | 使用 6 组 C++ 测试向量，就地加密 → 验证密文 → 就地解密 → 验证回到明文 |
| 2 | `TestXXTEAInputOutput` | `xxtea_test::input_output` | 使用独立 input/output buffer，验证加密 output 大小 ≥ input 大小且 4 字节对齐，解密还原 |
| 3 | `TestXXTEAEdgeCases` | 无 | 测试空输入错误、奇数长度 pad 验证、<8 字节输入处理 |

#### 5.2.1 `impl/atbus_endpoint_test.go` (新文件)

| 序号 | 测试名 | 对应 C++ | 说明 |
|------|--------|---------|------|
| 1 | `TestEndpointConnectionBasic` | `atbus_endpoint::connection_basic` | 测试 nil node 创建 connection 返回空 |
| 2 | `TestEndpointBasic` | `atbus_endpoint::endpoint_basic` | 测试 nil node 创建 endpoint 返回空 |
| 3 | `TestEndpointIsChild` | `atbus_endpoint::is_child` | 测试拓扑关系查询: Self/ImmediateDownstream/Invalid；创建 node，手动添加 topology peer，验证 `GetTopologyRelation()` 返回正确关系和 next_hop |
| 4 | `TestEndpointGetConnection` | `atbus_endpoint::get_connection` | 测试优先级选择: 创建 endpoint 添加不同类型 connection（本地 vs 远程），验证 `GetDataConnection()` 返回优先级最高的连接 |
| 5 | `TestChannelAddress` | `atbus_channel::address` | 测试地址解析: 空字符串/duplex/simplex/local_host/local_process 判断（已在 `channel/utility/` 中覆盖，可跳过或移入） |

#### 5.2.2 `impl/atbus_node_setup_test.go` (新文件)

| 序号 | 测试名 | 对应 C++ | 说明 |
|------|--------|---------|------|
| 1 | `TestNodeSetupOverrideListenPath` | `atbus_node_setup::override_listen_path` | 仅限 Unix/macOS: 三个 node 监听相同 unix socket 路径，验证 overwrite_listen_path=true/false 行为 |
| 2 | `TestNodeSetupCryptoAlgorithms` | `atbus_node_setup::crypto_algorithms` | 遍历所有 ATBUS_CRYPTO_ALGORITHM_TYPE，验证可用算法列表 |
| 3 | `TestNodeSetupCompressionAlgorithms` | `atbus_node_setup::compression_algorithms` | 遍历所有 ATBUS_COMPRESSION_ALGORITHM_TYPE，验证可用算法列表 |

#### 5.2.3 `impl/atbus_node_reg_test.go` (新文件 - 核心)

> 所有测试使用 TCP (ipv4/ipv6) 通道。每个测试需创建真实 node、真实网络连接。
> 使用辅助函数 `waitUntil(condition, timeout)` 等待异步条件。

| 序号 | 测试名 | 对应 C++ | 关键时序和断言 |
|------|--------|---------|--------------|
| 1 | `TestNodeRegResetAndSendTcp` | `atbus_node_reg::reset_and_send_tcp` | 1) 创建 2 个对等节点，配置 access_token；2) 双方 Init/Listen/Start；3) 设置 add/remove endpoint 回调；4) node1 Connect → node2；5) 等待 `IsEndpointAvailable` 双向；6) 发送数据验证接收；7) `Shutdown()` node1；8) 等待 endpoint 变为 nil；9) 断言: add_count ≥ 2, remove_count ≥ 2 |
| 2 | `TestNodeRegTimeout` | `atbus_node_reg::timeout` | 1) 2 节点，设置 `on_new_connection` & `on_invalid_connection` 回调；2) Connect；3) 等 new_conn_count ≥ 1；4) 跳过时间超过 `first_idle_timeout`，调 `Proc(futureTime)`；5) 等 invalid_conn_count ≥ 1；6) 断言: 超时状态 = EN_ATBUS_ERR_NODE_TIMEOUT |
| 3 | `TestNodeRegMessageSizeLimit` | `atbus_node_reg::message_size_limit` | 1) 配置 `MessageSize = 4096`；2) 发送恰好 4096 字节的数据 → 成功；3) 发送 4097 字节 → EN_ATBUS_ERR_INVALID_SIZE |
| 4 | `TestNodeRegFailedMismatchAccessToken` | `atbus_node_reg::reg_failed_with_mismatch_access_token` | 1) 节点 1 用 token "A"，节点 2 用 token "B"；2) Connect；3) 等待短时间；4) 断言: register_failed ≥ 2, status = ACCESS_DENY |
| 5 | `TestNodeRegFailedMissingAccessToken` | `atbus_node_reg::reg_failed_with_missing_access_token` | 同上，但一方无 token |
| 6 | `TestNodeRegFailedUnsupportedVersion` | `atbus_node_reg::reg_failed_with_unsupported` | 1) node1 的 ProtocolVersion < MinimalVersion；2) Connect；3) 断言: UNSUPPORTED_VERSION |
| 7 | `TestNodeRegDestruct` | `atbus_node_reg::destruct` | 1) 2 节点连接；2) 将 node1 设为 nil（Go GC 或 Reset）；3) 等 node2 检测到端点丢失 |
| 8 | `TestNodeRegPcSuccess` | `atbus_node_reg::reg_pc_success` | 1) 上下游节点，配置 UpstreamAddress；2) 设置 on_register/on_available 回调；3) 依序 Init/Listen/Start；4) 等双方 `IsEndpointAvailable`；5) 断言: register_count ≥ 2, add_endpoint ≥ 2, available ≥ 2；6) Disconnect → SUCCESS, 再次 Disconnect → NOT_FOUND |
| 9 | `TestNodeRegPcSuccessCrossSubnet` | `atbus_node_reg::reg_pc_success_cross_subnet` | 同 test 8，但 bus_id 首字节不同，验证跨子网仍可连接 |
| 10 | `TestNodeRegBrotherSuccess` | `atbus_node_reg::reg_bro_success` | 对等节点注册，验证双向 endpoint 可用 |
| 11 | `TestNodeRegConflict` | `atbus_node_reg::conflict` | 1) 上游 + 2 下游，下游 ID 子网冲突；2) 一个注册成功，另一个失败 |
| 12 | `TestNodeRegReconnectUpstreamFailed` | `atbus_node_reg::reconnect_upstream_failed` | 1) 上下游连接成功；2) Reset 上游；3) 下游变为 LostUpstream/ConnectingUpstream；4) 创建新上游；5) 下游重新连接到 Running |
| 13 | `TestNodeRegSetHostname` | `atbus_node_reg::set_hostname` | 设置/获取/恢复 hostname |
| 14 | `TestNodeRegOnCloseConnectionNormal` | `atbus_node_reg::on_close_connection_normal` | 1) 2 节点连接；2) 设 on_close_connection 回调；3) node1 Shutdown；4) 断言: close_count 增加 |
| 15 | `TestNodeRegOnCloseConnectionByPeer` | `atbus_node_reg::on_close_connection_by_peer` | 同上，但 node1 直接 Reset（非优雅关闭），验证对端检测到断开 |
| 16 | `TestNodeRegOnTopologyUpstreamSet` | `atbus_node_reg::on_topology_upstream_set` | 1) 设置 on_topology_update_upstream 回调；2) 上下游连接；3) 断言: 回调触发，新 upstream ID 正确 |
| 17 | `TestNodeRegOnTopologyUpstreamClear` | `atbus_node_reg::on_topology_upstream_clear` | 1) 上下游连接成功后 Reset 上游；2) 断言: 回调触发或下游检测到 upstream 丢失 |
| 18 | `TestNodeRegOnTopologyUpstreamChangeId` | `atbus_node_reg::on_topology_upstream_change_id` | 1) 上下游连接成功；2) Reset 旧上游；3) 创建新上游(不同 ID)；4) 下游重新连接；5) 断言: 回调触发，old_id ≠ new_id |

> **注**: 跳过 `mem_and_send`(#15) 和 `shm_and_send`(#16)：Go 不支持 mem/shm 通道。

#### 5.2.4 `impl/atbus_node_msg_test.go` (新文件 - 核心)

> 消息收发、路由转发、加密测试。

| 序号 | 测试名 | 对应 C++ | 关键时序和断言 |
|------|--------|---------|--------------|
| 1 | `TestNodeMsgPingPong` | `atbus_node_msg::ping_pong` | 1) 3 节点: node1, node2, upstream；PingInterval=1s；2) 连接等待；3) 等待 ≥4 秒的 pong 计数；4) 断言: pong_count ≈ elapsed_seconds × expected_rate (±tolerance)；5) 断言: 双方 endpoint 的 LastPong 时间 > 0 |
| 2 | `TestNodeMsgCustomCmd` | `atbus_node_msg::custom_cmd` | 1) 2 对等节点连接；2) 设置 custom_command_request/response 回调；3) 分配 sequence；4) 发送 3 段命令 ["hello", "world", "!"]；5) 等待 response；6) 断言: 接收数据 = 发送数据，sequence 正确 |
| 3 | `TestNodeMsgCustomCmdByTempNode` | `atbus_node_msg::custom_cmd_by_temp_node` | 1) node1(固定ID) + node2(临时 ID=0)；2) node2 不 Listen，直接连接 node1；3) node2 发 custom command 给 node1；4) 断言: 正常收到响应 |
| 4 | `TestNodeMsgSendCmdToSelf` | `atbus_node_msg::send_cmd_to_self` | 1) 单节点；2) 未初始化时发送 → NOT_INITED；3) 初始化后发送自定义命令给自己；4) 断言: request + response 回调各触发一次 (count+=2) |
| 5 | `TestNodeMsgResetAndSend` | `atbus_node_msg::reset_and_send` | 1) 单节点发送数据给自己；2) 断言: 接收数据正确 |
| 6 | `TestNodeMsgSendLoopbackError` | `atbus_node_msg::send_loopback_error` | 1) 2 对等节点；2) 构造指向不存在节点的消息，通过已有连接发送；3) 断言: 对端返回 INVALID_ID 错误 |
| 7 | `TestNodeMsgSendToSelfAndNeedRsp` | `atbus_node_msg::send_msg_to_self_and_need_rsp` | 1) 单节点，REQUIRE_RSP 标志；2) request 回调中再次发送数据；3) 断言: 总计 3 次回调 (初始 req + echo req + echo rsp) |
| 8 | `TestNodeMsgUpstreamAndDownstream` | `atbus_node_msg::upstream_and_downstream` | 1) 上下游 2 节点；2) 双向发送数据；3) 断言: 双向均成功接收，数据匹配 |
| 9 | `TestNodeMsgTransferAndConnect` | `atbus_node_msg::transfer_and_connect` | 1) 1 上游 + 2 下游（兄弟），手动注册拓扑；2) 下游1 → 下游2 发送数据 (经上游转发)；3) 断言: 下游2 收到数据，router 路径包含上游 |
| 10 | `TestNodeMsgTransferOnly` | `atbus_node_msg::transfer_only` | 1) 4 节点 (2 上游 + 2 下游)，手动注册拓扑；2) 跨上游发送 (down1→up1→up2→down2)；3) 断言: 多跳转发成功 |
| 11 | `TestNodeMsgTopologyMultiLevelRoute` | `atbus_node_msg::topology_registry_multi_level_route` | 1) 3 节点链 (上游→中间→下游)；2) 注册拓扑前: `GetTopologyRelation(下游)` = Invalid；3) 注册拓扑后: `GetTopologyRelation(下游)` = TransitiveDownstream, next_hop = 中间节点；4) 发送数据 + RequireResponse；5) 断开中间→下游连接后重试 → 收到错误响应 |
| 12 | `TestNodeMsgTopologyMultiLevelRouteReverse` | `atbus_node_msg::topology_registry_multi_level_route_reverse` | 同 test 11 但反向（下游→上游） |
| 13 | `TestNodeMsgSendFailed` | `atbus_node_msg::send_failed` | 1) 单上游节点；2) 发送到不存在的 ID → INVALID_ID |
| 14 | `TestNodeMsgTransferFailed` | `atbus_node_msg::transfer_failed` | 1) 上下游 + fake peer 拓扑；2) 下游发送到 fake peer via 上游 → 上游转发失败；3) 断言: response 回调收到错误 |
| 15 | `TestNodeMsgTransferFailedCrossUpstreams` | `atbus_node_msg::transfer_failed_cross_upstreams` | 1) 复杂拓扑（2 上游 + 下游）；2) 发送到不存在的目标 → 通过上游链转发后失败；3) 断言: 错误响应，但本地连接不断 |
| 16 | `TestNodeMsgHandlerGetBodyName` | `atbus_node_msg::msg_handler_get_body_name` | 1) GetBodyName(0) = "Unknown"；2) GetBodyName(大数) = "Unknown"；3) GetBodyName(DataTransformReq) = 有效名 |
| 17 | `TestNodeMsgCryptoKeyExchangeAlgorithms` | `atbus_node_msg::crypto_config_key_exchange_algorithms` | 遍历所有 KeyExchangeType (X25519, P256, P384, P521)，每个配置 AES-256-GCM，2 对等节点加密通信 |
| 18 | `TestNodeMsgCryptoCipherAlgorithms` | `atbus_node_msg::crypto_config_cipher_algorithms` | 遍历所有 CryptoAlgorithm (AES-CBC/GCM, ChaCha20-Poly1305 等)，每个配置 X25519 交换，验证通信 |
| 19 | `TestNodeMsgCryptoComprehensiveMatrix` | `atbus_node_msg::crypto_config_comprehensive_matrix` | KeyExchange × CipherAlgorithm 全组合测试 |
| 20 | `TestNodeMsgCryptoMultipleAlgorithms` | `atbus_node_msg::crypto_config_multiple_algorithms` | 配置允许多个算法，验证协商成功 |
| 21 | `TestNodeMsgCryptoUpstreamDownstream` | `atbus_node_msg::crypto_config_upstream_downstream` | 上下游节点加密通信测试 |
| 22 | `TestNodeMsgCryptoDisabled` | `atbus_node_msg::crypto_config_disabled` | CryptoKeyExchangeType = NONE，验证明文通信正常 |
| 23 | `TestNodeMsgCryptoListAvailableAlgorithms` | `atbus_node_msg::crypto_list_available_algorithms` | 列出所有可用算法，确认不为空 |

#### 5.2.5 `impl/atbus_node_relationship_test.go` (新文件)

| 序号 | 测试名 | 对应 C++ | 说明 |
|------|--------|---------|------|
| 1 | `TestNodeRelaCopyConf` | `atbus_node_rela::copy_conf` | 创建 NodeConfigure 并复制，验证字段一致 |
| 2 | `TestNodeRelaChildEndpointOpr` | `atbus_node_rela::child_endpoint_opr` | 1) 创建 node；2) 添加 3 个 endpoint（其中一个重复）；3) 删除不存在的 → NOT_FOUND；4) 删除已有的 → SUCCESS |

#### 5.2.6 `impl/atbus_connection_context_crosslang_test.go` (新文件)

> 跨语言兼容性测试：加载 C++ 生成的测试向量，验证 Go 实现能正确解析。

| 序号 | 测试名 | 对应 C++ | 说明 |
|------|--------|---------|------|
| 1 | `TestCrossLangNoEncryptionUnpack` | `crosslang_generator::generate_no_encryption_test_files` | 加载 C++ 生成的无加密消息二进制文件，用 Go unpack 验证 |
| 2 | `TestCrossLangCompressedUnpack` | `crosslang_generator::generate_compressed_test_files` | 加载 C++ 生成的压缩消息，验证 Go 能正确解压 |
| 3 | `TestCrossLangEncryptedUnpack` | `crosslang_generator::generate_encrypted_test_files` | 加载 C++ 生成的加密消息，用相同密钥解密验证 |
| 4 | `TestCrossLangCompressedEncryptedUnpack` | `crosslang_generator::generate_compressed_encrypted_test_files` | 加载 C++ 生成的压缩+加密消息验证 |

#### 5.2.7 `impl/atbus_access_data_crosslang_test.go` (新文件)

| 序号 | 测试名 | 对应 C++ | 说明 |
|------|--------|---------|------|
| 1 | `TestCrossLangAccessDataPlaintext` | `access_data_crosslang::generate_plaintext_test_files` | 加载 C++ 生成的 plaintext 测试向量，验证 Go `MakeAccessDataPlaintext*` 输出一致 |
| 2 | `TestCrossLangAccessDataSignature` | `access_data_crosslang::generate_signature_test_files` | 加载 C++ 生成的签名测试向量，验证 Go `CalculateAccessDataSignature` 输出一致 |
| 3 | `TestCrossLangAccessDataFull` | `access_data_crosslang::generate_full_access_data_test_files` | 加载 C++ 生成的完整 access_data，验证 Go `GenerateAccessData` 输出一致 |

#### 5.2.8 补充已有测试文件

##### `error_code/libatbus_error_test.go` 补充

| 序号 | 测试名 | 对应 C++ | 说明 |
|------|--------|---------|------|
| 1 | `TestLibatbusStrerrorKnownSamples` | `libatbus_error::strerror_known_samples` | 验证多个已知错误码的字符串映射 |
| 2 | `TestLibatbusStrerrorUnknownThreadLocalCache` | `libatbus_error::strerror_unknown_thread_local_cache` | 未知错误码的缓存行为 |
| 3 | `TestLibatbusWstrerror` | `libatbus_error::wstrerror_known_and_unknown` | 宽字符版本（Go 中可用 UTF-8 等价） |

##### `impl/atbus_connection_context_test.go` 补充

| 序号 | 测试名 | 对应 C++ | 说明 |
|------|--------|---------|------|
| 1 | `TestPaddingTemporaryBufferBlock` | `padding_*` (9 tests) | 测试 0/8/16/64/4096/1MB 等大小的对齐计算 |
| 2 | `TestHandshakeCompleteFlow` | `handshake_complete_flow` | 完整 client-server 握手: client 生成 key → server 读取 → server 生成 key → client 读取 → 验证双方共享密钥一致 |
| 3 | `TestHandshakeSequenceMismatch` | `handshake_sequence_mismatch` | 使用错误的 sequence 调用 confirm → 失败 |
| 4 | `TestHandshakeNoCommonAlgorithm` | `handshake_no_common_algorithm` | 双方无共同加密算法时的处理 |
| 5 | `TestPackUnpackWithEncryption` | `pack_unpack_encrypted_message` | AES-256-GCM 加密消息的打包/解包往返 |
| 6 | `TestPackUnpackWithCompression` | `pack_unpack_compressed_message` | 各压缩算法的打包/解包往返 |
| 7 | `TestPackUnpackSizeLimit` | `pack_unpack_size_limit` | 超过 max_body_size 的消息 → 错误 |
| 8 | `TestPackUnpackInvalidData` | `pack_unpack_invalid_data` | 损坏的数据 → 错误 |
| 9 | `TestBidirectionalEncryptedCommunication` | `bidirectional_encrypted_communication` | 双方使用各自密钥加解密通信 |
| 10 | `TestAllKeyExchangeWithAES256GCM` | `all_key_exchange_algorithms_with_aes256_gcm` | X25519/P256/P384/P521 × AES-256-GCM |
| 11 | `TestAllCryptoWithSECP256R1` | `all_crypto_algorithms_with_secp256r1` | P-256 × 所有密码算法 |
| 12 | `TestAllCryptoWithX25519` | `all_crypto_algorithms_with_x25519` | X25519 × 所有密码算法 |
| 13 | `TestComprehensiveCryptoMatrix` | `comprehensive_crypto_matrix` | 所有 KeyExchange × 所有 Cipher 全组合 |
| 14 | `TestAEADCiphersVerification` | `aead_ciphers_verification` | 验证 AEAD 密码的 tag_size 和行为 |
| 15 | `TestNonAEADCiphersVerification` | `non_aead_ciphers_verification` | 验证非 AEAD 密码的 PKCS#7 padding |
| 16 | `TestHigherSecurityKeyExchange` | `higher_security_key_exchange` | P-384, P-521 高安全级别密钥交换 |
| 17 | `TestKeyRenegotiationFlow` | `key_renegotiation_flow` | 完整密钥重协商: 初始握手 → 通信 → 重新生成密钥 → stage → confirm → 新密钥通信 |

### 5.3 测试辅助工具

在 Go 测试中需创建以下辅助:

```go
// impl/atbus_test_utils_test.go

// waitUntil 循环执行 node.Proc() 直到条件满足或超时
func waitUntil(t *testing.T, condition func() bool, timeout time.Duration,
    tickInterval time.Duration, nodes ...types.Node) bool

// createTestNode 创建、初始化并启动测试节点
func createTestNode(t *testing.T, busId types.BusIdType,
    listenAddr string, opts ...TestNodeOption) types.Node

// TestNodeOption 用于自定义节点配置
type TestNodeOption func(*types.NodeConfigure)

// withAccessToken 配置 access token
func withAccessToken(token string) TestNodeOption

// withCrypto 配置加密算法
func withCrypto(keyExchange protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE,
    algorithms ...protocol.ATBUS_CRYPTO_ALGORITHM_TYPE) TestNodeOption

// withCompression 配置压缩算法
func withCompression(algorithms ...protocol.ATBUS_COMPRESSION_ALGORITHM_TYPE) TestNodeOption

// withUpstream 配置上游地址
func withUpstream(address string) TestNodeOption

// withPingInterval 配置 ping 间隔
func withPingInterval(interval time.Duration) TestNodeOption

// withMessageSize 配置最大消息大小
func withMessageSize(size uint64) TestNodeOption

// RecvMsgHistory 记录接收到的消息历史
type RecvMsgHistory struct {
    mu        sync.Mutex
    Count     int
    Data      []byte
    Status    error_code.ErrorType
    PingCount int
    PongCount int
    // ... 更多跟踪字段
}
```

### 5.4 测试执行规范

1. **网络端口**: 每个测试函数使用唯一端口范围，避免并发冲突
2. **超时**: 所有 `waitUntil` 使用合理超时（建议 8-15 秒），ticker 间隔 50-100ms
3. **清理**: 每个测试结束时 `defer node.Reset()` 确保资源释放
4. **并行**: `node_reg` 和 `node_msg` 测试不建议并行执行（端口冲突）
5. **跳过条件**: Unix socket 测试在 Windows 上 `t.Skip()`
6. **时序要求**:
   - 连接建立后再发数据
   - Proc() 调用需在主 goroutine 或受控 goroutine 中
   - ping/pong 测试需精确控制时间推进
7. **测试命名**: 遵循 Go 惯例 `TestXxx`，用 `t.Run()` 做子测试

### 5.5 测试优先级

| 优先级 | 测试文件 | 原因 |
|--------|---------|------|
| P0 | `atbus_node_reg_test.go` | 连接/注册是核心功能 |
| P0 | `atbus_node_msg_test.go` | 消息收发是核心功能 |
| P0 | `atbus_connection_context_test.go` 补充 | 加密正确性关键 |
| P1 | `atbus_node_relationship_test.go` | 拓扑操作验证 |
| P1 | `atbus_endpoint_test.go` | 端点管理验证 |
| P1 | `atbus_connection_context_crosslang_test.go` | 跨语言互通验证 |
| P1 | `atbus_access_data_crosslang_test.go` | 跨语言签名验证 |
| P2 | `atbus_node_setup_test.go` | 配置和算法验证 |
| P2 | `error_code` 补充 | 错误码完整性 |

---

## 六、实施路线

### 阶段 1: Bug 修复与验证 (P0)

1. 修复 `getConnectionBinding()` 无限递归 Bug (P0-9)
2. 修复 `SendTransferResponse()` body type 检查 Bug (P0-10)
3. 逐项对比 Go/C++ 默认配置值，修正差异
4. 验证 HKDF、access token 格式、TTL 行为
5. 验证控制消息绕过加密/压缩
6. 验证消息大小限制
7. 验证 key renegotiation 完整时序

### 阶段 2: 核心测试 (P0)

8. 创建测试辅助工具 (`atbus_test_utils_test.go`)
9. 实现 `atbus_node_reg_test.go` 全部用例
10. 实现 `atbus_node_msg_test.go` 全部用例
11. 补充 `atbus_connection_context_test.go` 缺失用例

### 阶段 3: XXTEA 实现 (P1)

12. 在 `atframe-utils-go/algorithm/crypto/` 实现 XXTEA 算法
13. 实现 XXTEA 单元测试 (使用 C++ 测试向量)
14. 集成 XXTEA 到 libatbus-go connection_context 加密分支
15. 添加 `IsCompressionAlgorithmSupported()` 公开函数

### 阶段 4: 完善测试 (P1)

16. 实现 `atbus_endpoint_test.go`
17. 实现 `atbus_node_relationship_test.go`
18. 实现跨语言兼容性测试
19. 补充 error_code 测试

### 阶段 5: 功能修复 (P1-P2)

20. 修复在测试中发现的行为差异
21. ChaCha20 纯流密码兼容性
22. overwrite_listen_path 支持

### 阶段 6: 可选增强 (P3)

23. Replay attack 防护

---

## 七、C++ 与 Go 接口对照表

### Node 接口

| C++ 方法 | Go 方法 | 状态 |
|----------|---------|------|
| `create()` | `impl.CreateNode()` | ✅ |
| `init(id, conf)` | `Init(id, conf)` | ✅ |
| `start()` / `start(conf)` | `Start()` / `StartWithConfigure(conf)` | ✅ |
| `reset()` | `Reset()` | ✅ |
| `proc(now)` | `Proc(now)` | ✅ |
| `poll()` | `Poll()` | ✅ (可能为 no-op) |
| `listen(addr)` | `Listen(addr)` | ✅ |
| `connect(addr)` | `Connect(addr)` | ✅ |
| `connect(addr, ep)` | `ConnectWithEndpoint(addr, ep)` | ✅ |
| `disconnect(id)` | `Disconnect(id)` | ✅ |
| `send_data(tid, type, data)` | `SendData(tid, t, data)` | ✅ |
| `send_data(tid, type, data, opts)` | `SendDataWithOptions(tid, t, data, opts)` | ✅ |
| `send_custom_command(tid, args)` | `SendCustomCommand(tid, args)` | ✅ |
| `send_custom_command(tid, args, opts)` | `SendCustomCommandWithOptions(tid, args, opts)` | ✅ |
| `get_peer_channel(...)` | `GetPeerChannel(...)` | ✅ |
| `set_topology_upstream(tid)` | `SetTopologyUpstream(tid)` | ✅ |
| `get_endpoint(tid)` | `GetEndpoint(tid)` | ✅ |
| `add_endpoint(ep)` | `AddEndpoint(ep)` | ✅ |
| `remove_endpoint(tid)` | `RemoveEndpoint(ep)` | ✅ by design，Go 用 Endpoint 对象 |
| `is_endpoint_available(tid)` | `IsEndpointAvailable(tid)` | ✅ |
| `check_access_hash(...)` | `CheckAccessHash(...)` | ✅ |
| `reload_crypto(...)` | `ReloadCrypto(...)` | ✅ |
| `reload_compression(...)` | `ReloadCompression(...)` | ✅ |
| `get_crypto_key_exchange_type()` | `GetCryptoKeyExchangeType()` | ✅ |
| `get_hash_code()` | `GetHashCode()` | ✅ |
| `get_iostream_channel()` | `GetIoStreamChannel()` | ✅ |
| `get_self_endpoint()` | `GetSelfEndpoint()` | ✅ |
| `get_upstream_endpoint()` | `GetUpstreamEndpoint()` | ✅ |
| `get_immediate_endpoint_set()` | `GetImmediateEndpointSet()` | ✅ |
| `default_conf(conf)` | `SetDefaultNodeConfigure(conf)` | ✅ |
| N/A (C++ 用 uv_loop) | `GetContext()` | ✅ Go 用 context |
| N/A | `Shutdown(reason)` | ✅ Go 新增 |
| N/A | `FatalShutdown(...)` | ✅ Go 新增 |

### Endpoint 接口

| C++ 方法 | Go 方法 | 状态 |
|----------|---------|------|
| `create(owner, id, pid, hostname)` | `CreateEndpoint(id, hostname, pid)` via Node | ✅ |
| `reset()` | `Reset()` | ✅ |
| `get_id()` | `GetId()` | ✅ |
| `get_pid()` | `GetPid()` | ✅ |
| `get_hostname()` | `GetHostname()` | ✅ |
| `get_hash_code()` | `GetHashCode()` | ✅ |
| `update_hash_code(code)` | `UpdateHashCode(code)` | ✅ |
| `add_connection(conn, force)` | `AddConnection(conn, force)` | ✅ |
| `remove_connection(conn)` | `RemoveConnection(conn)` | ✅ |
| `is_available()` | `IsAvailable()` | ✅ |
| `get_flag(f)` / `set_flag(f,v)` | `GetFlag(f)` / `SetFlag(f,v)` | ✅ |
| `get_listen()` | `GetListenAddress()` | ✅ |
| `add_listen(addr)` | `AddListenAddress(addr)` | ✅ |
| `clear_listen()` | `ClearListenAddress()` | ✅ |
| `update_supported_schemas(set)` | `UpdateSupportSchemes(list)` | ✅ |
| `is_schema_supported(s)` | `IsSchemeSupported(s)` | ✅ |
| `add_ping_timer()` | `AddPingTimer()` | ✅ |
| `clear_ping_timer()` | `ClearPingTimer()` | ✅ |
| `get_ctrl_connection(ep)` | `GetCtrlConnection(ep)` | ✅ |
| `get_data_connection(ep)` | `GetDataConnection(ep, fallback)` | ✅ |
| `get_data_connection_count(fallback)` | `GetDataConnectionCount(fallback)` | ✅ |
| 所有统计方法 | 全部对应 | ✅ |

### Connection 接口

| C++ 方法 | Go 方法 | 状态 |
|----------|---------|------|
| `create(owner, addr)` | 通过 Node 内部创建 | ✅ |
| `reset()` | `Reset()` | ✅ |
| `proc(node, now)` | `Proc()` | ✅ by design，Go 无 mem/shm 通道无需时间参数 |
| `listen()` | `Listen()` | ✅ |
| `connect()` | `Connect()` | ✅ |
| `disconnect()` | `Disconnect()` | ✅ |
| `push(buffer)` | `Push(buffer)` | ✅ |
| `get_address()` | `GetAddress()` | ✅ |
| `is_connected()` | `IsConnected()` | ✅ |
| `is_running()` | `IsRunning()` | ✅ |
| `get_binding()` | `GetBinding()` | ✅ |
| `get_status()` | `GetStatus()` | ✅ |
| `check_flag(f)` | `CheckFlag(f)` | ✅ |
| `set_temporary()` | `SetTemporary()` | ✅ |
| `get_statistic()` | `GetStatistic()` | ✅ |
| `get_connection_context()` | `GetConnectionContext()` | ✅ |
| `unpack(conn, msg, data)` | via `ConnectionContext.UnpackMessage()` | ✅ |

### ConnectionContext 接口

| C++ 方法 | Go 方法 | 状态 |
|----------|---------|------|
| `create(algo, dh_ctx)` | 内部创建 | ✅ |
| `pack_message(...)` | `PackMessage(...)` | ✅ |
| `unpack_message(...)` | `UnpackMessage(...)` | ✅ |
| `handshake_generate_self_key(seq)` | `HandshakeGenerateSelfKey(seq)` | ✅ |
| `handshake_read_peer_key(...)` | `HandshakeReadPeerKey(...)` | ✅ |
| `confirm_handshake(seq)` | `ConfirmHandshake(seq)` | ✅ |
| `handshake_write_self_public_key(...)` | `HandshakeWriteSelfPublicKey(...)` | ✅ |
| `get_handshake_start_time()` | `GetHandshakeStartTime()` | ✅ |
| `get_crypto_key_exchange_algorithm()` | `GetCryptoKeyExchangeAlgorithm()` | ✅ |
| `get_crypto_select_kdf_type()` | `GetCryptoSelectKdfType()` | ✅ |
| `get_crypto_select_algorithm()` | `GetCryptoSelectAlgorithm()` | ✅ |
| `get_compression_select_algorithm()` | `GetCompressSelectAlgorithm()` | ✅ |
| `update_compression_algorithm(algos)` | `UpdateCompressionAlgorithm(algos)` | ✅ |
| `setup_crypto_with_key(...)` | `SetupCryptoWithKey(...)` | ✅ |
| `is_compression_algorithm_supported(algo)` | 无独立方法 | ⚠️ 需添加 (P1-12) |

### TopologyRegistry 接口

| C++ 方法 | Go 方法 | 状态 |
|----------|---------|------|
| `create()` | `CreateTopologyRegistry()` | ✅ |
| `update_peer(targetId, upstreamId, data)` | `UpdatePeer(targetId, upstreamId, data)` | ✅ 签名一致 |
| `get_peer(id)` | `GetPeer(id)` | ✅ |
| `foreach_peer(fn)` | `ForeachPeer(fn)` | ✅ |
| `remove_peer(id)` | `RemovePeer(id)` | ✅ |
| `get_relation(from, to, &nextHop)` | `GetRelation(from, to) (relation, nextHop)` | ✅ Go 多返回值 |
| `static check_policy(rule, fromData, toData)` | `CheckPolicy(rule, fromData, toData)` | ✅ C++ static / Go instance |

---

## 八、风险与注意事项

1. **端口冲突**: Go 测试需使用不同端口范围，避免与 C++ 测试冲突
2. **平台差异**: Unix socket 在 Windows 上不可用，需 `t.Skip()` 或用 TCP 替代
3. **时间精度**: Go 的 `time.Time` 精度为纳秒，与 C++ `std::chrono::microseconds` 对齐
4. **GC 影响**: Go GC 可能影响连接超时和 ping/pong 测试的时序
5. **goroutine 泄漏**: 每个测试结束需确保所有 goroutine 正确关闭
6. **竞态检测**: 建议所有测试在 `-race` 标志下运行
7. **跨语言测试数据**: 需要 C++ 侧先生成测试向量文件，Go 侧加载验证
