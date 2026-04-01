# libatbus-go 功能补全计划（修订版）

> 本文档基于对 `atsf4g-co/atframework/libatbus` 与
> `atsf4g-go/atframework/libatbus-go` 的源码逐项核查。
>
> 判定原则：
>
> - 以 C++ `libatbus` 当前实现为真值；
> - Go 版本明确不做 `mem://` 与 `shm://`；
> - 正确性、跨语言互通、主要对外接口一致性优先；
> - 已存在的能力不再误列为“缺失项”。

单独的单元测试执行计划见 `UnitTestPlan.md`。

---

## 一、结论先行

- `atbus_node` **不能再按“95% 完成”评估**。当前存在多处已确认的运行时逻辑错误，
  且缺少专门回归测试。
- `atbus_endpoint` 与 `atbus_connection` 的主体结构基本在，但被 `Node` 集成问题、
  根包公开 API 暴露不足、缺少专项测试拖住了。
- `atbus_connection_context`、`protocol`、`topology`、`buffer`、`io_stream`
  比旧计划写得更完整；旧计划把一部分已实现能力错写成了缺失。
- 此前跨语言互通的主要缺口集中在 **XXTEA** 与 **纯 ChaCha20**；
  当前两者的实现已经补齐。
  AES/CBC、AES/GCM、ChaCha20-Poly1305、XChaCha20-Poly1305、压缩、
  AccessData/HMAC、握手确认流程已经有实现基础，并且现有 Go 测试已覆盖相当一部分。

---

## 二、模块状态总表

| 模块 | 当前状态 | 已核实结论 | 主要缺口 / 风险 |
| ------ | ---------- | ------------ | ----------------- |
| `atbus_node` | ❌ 关键路径待修复 | 主体接口存在 | `Listen` / `GetPeerChannel` / `AddEndpoint` / `RemoveEndpoint` 相关逻辑存在确认缺陷，且缺少专项测试 |
| `atbus_endpoint` | ⚠️ 基础能力在 | 统计、连接选择、listen 地址管理主体已在 | 受 `Node` 集成问题影响；缺少 dedicated parity tests |
| `atbus_connection` | ⚠️ 基础能力在 | 生命周期主体存在；不支持 `mem://` / `shm://` 属设计范围 | 根包未给出清晰公开创建入口；缺少专项回归测试 |
| `atbus_connection_context` | ⚠️ 大部分已对齐 | 握手、HKDF、压缩、主流 cipher、XXTEA、纯 ChaCha20 已在 | 继续补 pure ChaCha20 的跨语言向量与更多回归 |
| `atbus_message_handler` | ⚠️ 基本对齐 | 功能面与 C++ 接近 | 已确认 2 个 P0 Bug |
| `atbus_topology` | ✅ 已对齐 | 接口和核心行为已基本对齐 | 保持回归测试即可 |
| `channel/io_stream` | ✅ 已对齐 | TCP / Unix / pipe 风格 I/O 流能力基本齐 | 无需新增 mem/shm 能力 |
| `channel/utility` | ✅ 已对齐 | 地址解析、优先级相关能力已在 | 保持回归测试即可 |
| `buffer` | ✅ 已对齐 | 覆盖较充分 | 无 |
| `error_code` | ✅ 主体已在 | 错误码映射主体已在 | 补足剩余字符串映射用例 |
| `protocol` | ✅ 已对齐 | `.proto` 一致 | 无 |
| `channel_mem` | N/A | 不在 Go 范围内 | 明确不做 |
| `channel_shm` | N/A | 不在 Go 范围内 | 明确不做 |

---

## 三、上一版 `Plan.md` 的误判修正

### 3.1 默认配置的真实对齐结果

下表为本次重新核对后的真实结果。旧版计划中关于默认值的多项数字不正确。

| 配置项 | C++ 当前值 | Go 当前值 | 结论 |
| -------- | ------------ | ----------- | ------ |
| `LoopTimes` | `256` | `256` | ✅ 一致 |
| `TTL` | `16` | `16` | ✅ 一致 |
| `FirstIdleTimeout` | `30s` | `30s` | ✅ 一致 |
| `PingInterval` | `8s` | `8s` | ✅ 一致 |
| `RetryInterval` | `3s` | **未初始化** | ❌ 需修复 |
| `FaultTolerant` | `2` | `2` | ✅ 一致 |
| `BackLog` | `256` | `256` | ✅ 一致 |
| `AccessTokenMaxNumber` | `5` | `5` | ✅ 一致 |
| `CryptoKeyRefreshInterval` | `3h` | `3h` | ✅ 一致 |
| `MessageSize` | `2 MiB` | `2 MiB` | ✅ 一致 |
| `RecvBufferSize` | `256 MiB` | `256 MiB` | ✅ 一致 |
| `SendBufferSize` | `8 MiB` | `8 MiB` | ✅ 一致 |

旧版计划中写到的 `128`、`60s`、`3`、`1h`、`256KB`、`2MB`、`8MB`
等默认值判断均不准确，应以本表为准。

### 3.2 已经存在、无需再列为缺失项的能力

- `types.IsCompressionAlgorithmSupported(...)` 已存在；
  `ConnectionContext.IsCompressionAlgorithmSupported(...)` 也已实现。
- `impl/atbus_connection_context_test.go` 已经覆盖大量跨语言 handshake /
  cipher / compression / confirm 流程，不是“无跨语言测试”。
- `message_handle/atbus_message_handler_test.go` 已经覆盖 AccessData
  plaintext / signature 的跨语言向量验证，不是“无 access_data 跨语言测试”。
- `Shutdown` / `on_node_down` 当前 C++ 实现并不会根据回调返回值延迟 `reset()`；
  Go **不需要**为了“对齐 C++”去实现旧计划里提到的这条行为。
- `Poll()` 与 `connection.Proc()` 在 Go 里的差异属于设计差异，
  不应按 `mem://` / `shm://` 模型强行追平。

### 3.3 明确不做的能力

- `mem://` 通道；
- `shm://` 通道；
- C++ `endpoint::watch()` 这类手动引用计数语义；
- C++ `ref_object()` / `unref_object()` — Go 使用 GC，无需手动引用计数；
- C++ `get_evloop()` — Go 使用原生 goroutine 而非 libuv 事件循环；
- C++ `get_crypto_key_exchange_context()` — Go 的 `crypto/ecdh` 标准库是无状态的，
  不需要共享 DH 上下文对象；
- 为 `mem://` / `shm://` 保留的连接回调与时间推进接口。

---

## 四、确认存在的缺失与错误

### 4.1 P0：必须优先修复的已确认问题

| 编号 | 问题 | 影响 | 涉及文件 |
| ------ | ------ | ------ | ---------- |
| P0-1 | `SetDefaultNodeConfigure()` 未初始化 `RetryInterval` | 影响上游重连与超时调度，默认行为与 C++ 不一致 | `types/atbus_common_types.go` |
| P0-2 | `ReloadCompression()` 成功写入配置后却返回 `EN_ATBUS_ERR_PARAMS` | 调用方会误判为失败 | `impl/atbus_node.go` |
| P0-3 | `Listen()` 使用了反向的 `self` 判定，已初始化节点反而直接返回 `NOT_INITED` | 正常监听路径被拒绝 | `impl/atbus_node.go` |
| P0-4 | `GetPeerChannel()` 的状态门槛写反，非 `Created` 状态直接返回 `NOT_INITED` | 正常路由查询不可用 | `impl/atbus_node.go` |
| P0-5 | `AddEndpoint()` 拒绝 `owner != nil` 的 endpoint；而 `CreateEndpoint()` 恰恰总会设置 owner。C++ 中 `add_endpoint()` 检查的是 `this == ep->get_owner()`（要求 owner 是当前 node），Go 写反为 `owner == nil` | 正常 endpoint 挂载路径被无条件拒绝，与 C++ 行为完全相反 | `impl/atbus_node.go`, `impl/atbus_endpoint.go` |
| P0-6 | `removeChild()` 删除成功后固定返回 `false` | `RemoveEndpoint()` 可能在删除成功后仍回报 `NOT_FOUND` | `impl/atbus_node.go` |
| P0-7 | `getConnectionBinding()` 无限递归 | 运行时会栈溢出 | `message_handle/atbus_message_handler.go` |
| P0-8 | `SendTransferResponse()` 检查了错误的 body type | 数据转发响应流程可能走错分支 | `message_handle/atbus_message_handler.go` |
| P0-9 | `Proc()` 正常路径未调用 `dispatchAllSelfMessages()` | 自发消息（send_data 到自身）在非 shutdown 路径永远不会被投递，与 C++ 每帧 dispatch 的行为严重不一致 | `impl/atbus_node.go` |

### 4.2 P1：功能完整性与公开接口对齐问题

| 编号 | 问题 | 目标 | 涉及文件 |
| ------ | ------ | ------ | ---------- |
| P1-1 | ✅ 根包已新增清晰的 `CreateNode` / `NewNode` 公开入口 | 继续通过根包测试锁定公开用法 | `atbus_node.go`, `atbus_node_test.go` |
| P1-2 | ✅ `Endpoint` / `Connection` 已提供根包公开 helper | 继续通过根包测试锁定创建路径 | `atbus_endpoint.go`, `atbus_connection.go`, `atbus_*_test.go` |
| P1-3 | ✅ 已新增 `RemoveEndpointByID` | 对齐 C++ 的按 `bus_id_t` 删除入口 | `types/atbus_node.go`, `impl/atbus_node.go`, `atbus_node_test.go` |
| P1-4 | ✅ XXTEA 已实现 | 已补齐实现与跨语言回归 | `atframe-utils-go/algorithm/crypto`, `impl/atbus_connection_context.go`, `impl/atbus_connection_context_test.go` |
| P1-5 | ⚠️ 纯 ChaCha20 已实现 | 已补齐 cipher 列表一致性与 pack/unpack；仍建议补跨语言向量 | `impl/atbus_connection_context.go`, `impl/atbus_connection_context_test.go` |
| P1-6 | ⚠️ 已新增 `Endpoint` / `Connection` 专项测试，并继续补充 `NodeRelationship` / `NodeReg` / `NodeMsg` 的 parity tests；当前已覆盖 `copy_conf`、`set_hostname`、`custom_cmd`、`custom_cmd_by_temp_node`、`send_cmd_to_self`、`send_msg_to_self_and_need_rsp` 等 case 的无网络 / loopback 子集，更完整的注册与网络矩阵仍需继续补齐 | 建立能直接兜住上述 P0/P1 问题的回归网 | `impl/atbus_endpoint_test.go`, `impl/atbus_connection_test.go`, `impl/atbus_node_relationship_test.go`, `impl/atbus_node_reg_test.go`, `impl/atbus_node_msg_test.go`, `impl/*_test.go` |

### 4.3 待用专项测试确认的行为边界

以下项目目前**不作为已确认缺失**，但必须通过专项测试再判定是否需要修正：

- `processUpstreamOperations()` 与 C++ 在上游 ping / reconnect 调度时序上的细节差异；
- `fault_tolerant` 计数累积、`OnInvalidConnection` 触发阈值；
- `NodeGetPeerOptions.blacklist`、冲突路由、fallback 到上游等边界条件；
- ID 冲突、上游切换、跨子网注册的完整行为矩阵。

---

## 五、跨语言互通现状

| 项目 | 当前状态 | 说明 |
| ------ | ---------- | ------ |
| Protobuf / 帧格式 / 消息格式 | ✅ | `.proto` 一致，pack / unpack 主体已在 |
| AccessData plaintext / HMAC-SHA256 | ✅ | 已有实现，现有 Go 测试已覆盖一部分跨语言向量 |
| HKDF-SHA256 | ✅ | 当前实现与 C++ helper 形态一致，保留回归测试即可 |
| Key exchange: `X25519` / `P-256` / `P-384` / `P-521` | ✅ | 已实现 |
| AES-CBC / AES-GCM | ✅ | 已实现 |
| ChaCha20-Poly1305 / XChaCha20-Poly1305 | ✅ | 已实现 |
| Zstd / LZ4 / Snappy / Zlib | ✅ | 已实现 |
| Key renegotiation + handshake confirm | ✅ | server stage / client confirm 流程已在 |
| 纯 ChaCha20 | ⚠️ | 已补齐实现、协商与 pack/unpack；建议继续补 C++ 跨语言向量 |
| XXTEA | ✅ | 已实现并有跨语言测试 |

结论：

- 如果只考虑当前已实现算法集合，Go 与 C++ 的跨语言互通基础已经具备；
- 如果目标是 **“算法接口列表也一致”**，XXTEA 与纯 ChaCha20 已补齐；
- 如果目标还包括 **“纯 ChaCha20 的跨语言向量全覆盖”**，则仍需继续补测试；
- `mem://` / `shm://` 不应重新纳入范围。

---

## 六、单元测试计划摘要

详细执行计划见 `UnitTestPlan.md`。这里仅保留最重要的结论：

- 现有 Go 测试已经覆盖 `buffer`、`channel/io_stream`、`channel/utility`、
  `error_code`、`topology`、`message_handle`、`connection_context` 等模块；
- **真正的大空洞** 在 `Node`、`Endpoint`、`Connection`，而不是低层编解码；
- 旧版计划中的关键计数需要修正：
  - `atbus_message_handler_test.cpp` 是 **19** 个 case，不是 16；
  - `atbus_node_msg_test.cpp` 是 **23** 个 case；
  - `atbus_node_reg_test.cpp` 是 **21** 个 case，其中 `mem_and_send`、`shm_and_send`
    不在 Go 范围内，因此要求覆盖 **19** 个 case；
  - `atbus_node_relationship_test.cpp` 包含 `basic_test`，旧版遗漏了它；
  - `atbus_node_reg_test.cpp` 里还有一个旧版遗漏的 case：
    `reg_pc_failed_with_subnet_mismatch`。

建议新增 / 补强的测试文件：

- `impl/atbus_node_regression_test.go`：先兜住本次已确认的 P0 Bug；
- `impl/atbus_connection_test.go`：✅ 已新增，后续继续补强 `atbus_connection` 的直接回归；
- `impl/atbus_endpoint_test.go`：✅ 已新增，后续继续补强；
- `impl/atbus_node_setup_test.go`；
- `impl/atbus_node_relationship_test.go`：✅ 已新增，当前覆盖 `copy_conf`、`child_endpoint_opr`，以及端点增删生命周期回调回归；
- `impl/atbus_node_reg_test.go`：✅ 已新增，当前覆盖 `set_hostname`；其余注册 / 拓扑 / close-connection 矩阵继续补齐；
- `impl/atbus_node_msg_test.go`：✅ 已新增，当前覆盖 `send_cmd_to_self`、`custom_cmd`、`custom_cmd_by_temp_node`、`send_msg_to_self_and_need_rsp`、未初始化发送返回码、`message_size_limit` 的无网络 / loopback 子集；
- `error_code/libatbus_error_test.go` 与现有 `connection_context` /
  `message_handle` 测试做定向补强，而不是重写。

---

## 七、建议实施顺序

1. 先修复 P0 代码问题：默认值、`Node` 关键路径、`message_handle` 两个 Bug；
2. 立刻补 `impl/atbus_node_regression_test.go`，让每个 P0 Bug 都有直接回归；
3. 补 `Endpoint` / `Connection` / `NodeRelationship` / `NodeSetup` 的低成本对齐测试；
4. 再补 `NodeReg` 与 `NodeMsg` 两大网络核心测试集；
5. 明确根包公开 API 的构造入口，并补 `RemoveEndpointByID` 这类长期对齐项；
6. 最后补 XXTEA、纯 ChaCha20，并把跨语言矩阵扩到完整算法集合；
7. 全过程保持 `mem://` / `shm://` 排除，不要把范围偷偷长回去。

---

## 八、最终判定标准

满足以下条件后，才可认为 `libatbus-go` 与 C++ 版本进入“主要接口与行为已对齐”状态：

1. 本文列出的全部 P0 问题被修复并有对应回归测试；
2. `Node` / `Endpoint` / `Connection` 拥有独立测试文件，而不再只靠低层测试“侧面覆盖”；
3. `UnitTestPlan.md` 中列出的全部 **非 `mem://` / `shm://`** C++ 场景均有 Go 侧 traceability；
4. 跨语言算法列表与行为要么完整对齐，要么对剩余缺口（当前是 XXTEA、纯 ChaCha20）有明确结论和测试标记；
5. 根包公开 API 的创建与删除路径对外可用、可说明、可测试。
