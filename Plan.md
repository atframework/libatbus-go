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

- ✅ `atbus_node` **所有 P0 Bug 均已修复**，且有 17 项专项回归测试兜底。
  当前已有 19 项注册测试、26 项消息测试（15 基础 + 11 扩展）、5 项 setup 测试、3 项关系测试。
- ✅ `atbus_endpoint` 与 `atbus_connection` 已不再受 `Node` 集成问题拖累；
  根包公开 API 已暴露充分，各有专项测试。
- ✅ `atbus_connection_context`、`protocol`、`topology`、`buffer`、`io_stream`
  各模块均已完整对齐。
- ✅ 此前跨语言互通的主要缺口集中在 **XXTEA** 与 **纯 ChaCha20**；
  当前两者的实现与跨语言测试向量已全部补齐。
  AES/CBC、AES/GCM、ChaCha20-Poly1305、XChaCha20-Poly1305、纯 ChaCha20、XXTEA、压缩、
  AccessData/HMAC、握手确认流程均有实现与跨语言向量覆盖。
- 所有模块 `go test ./...` 全部通过。

---

## 二、模块状态总表

| 模块 | 当前状态 | 已核实结论 | 主要缺口 / 风险 |
| ------ | ---------- | ------------ | ----------------- |
| `atbus_node` | ✅ 已修复并有回归测试 | 主体接口存在，所有 P0 Bug 均已修复 | P0-1 ~ P0-9 全部修复，有 17 项回归测试、19 项注册测试、15 项消息测试、5 项 setup 测试 |
| `atbus_endpoint` | ✅ 已对齐 | 统计、连接选择、listen 地址管理主体已在；已有 dedicated tests | Node 集成问题已解决；3 项专项测试 |
| `atbus_connection` | ✅ 已对齐 | 生命周期主体存在；不支持 `mem://` / `shm://` 属设计范围 | 根包已有 `CreateConnection` 公开入口；7 项专项回归测试 |
| `atbus_connection_context` | ✅ 已对齐 | 握手、HKDF、压缩、主流 cipher、XXTEA、纯 ChaCha20 已在 | 跨语言向量已全覆盖（含纯 ChaCha20） |
| `atbus_message_handler` | ✅ 已对齐 | 功能面与 C++ 接近；P0-7 / P0-8 均已修复 | 无已知遗留问题 |
| `atbus_topology` | ✅ 已对齐 | 接口和核心行为已基本对齐 | 保持回归测试即可 |
| `channel/io_stream` | ✅ 已对齐 | TCP / Unix / pipe 风格 I/O 流能力基本齐 | 无需新增 mem/shm 能力 |
| `channel/utility` | ✅ 已对齐 | 地址解析、优先级相关能力已在 | 保持回归测试即可 |
| `buffer` | ✅ 已对齐 | 覆盖较充分 | 无 |
| `error_code` | ✅ 主体已在 | 错误码映射主体已在 | 补足剩余字符串映射用例 |
| `protocol` | ✅ 已对齐 | `.proto` 一致 | 无 |
| `channel_mem` | N/A | 不在 Go 范围内 | 明确不做 |
| `channel_shm` | N/A | 不在 Go 范围内 | 明确不做 |

---

## 三、默认配置对齐与设计边界

### 3.1 默认配置对齐结果

| 配置项 | C++ 当前值 | Go 当前值 | 结论 |
| -------- | ------------ | ----------- | ------ |
| `LoopTimes` | `256` | `256` | ✅ 一致 |
| `TTL` | `16` | `16` | ✅ 一致 |
| `FirstIdleTimeout` | `30s` | `30s` | ✅ 一致 |
| `PingInterval` | `8s` | `8s` | ✅ 一致 |
| `RetryInterval` | `3s` | `3s` | ✅ 一致 |
| `FaultTolerant` | `2` | `2` | ✅ 一致 |
| `BackLog` | `256` | `256` | ✅ 一致 |
| `AccessTokenMaxNumber` | `5` | `5` | ✅ 一致 |
| `CryptoKeyRefreshInterval` | `3h` | `3h` | ✅ 一致 |
| `MessageSize` | `2 MiB` | `2 MiB` | ✅ 一致 |
| `RecvBufferSize` | `256 MiB` | `256 MiB` | ✅ 一致 |
| `SendBufferSize` | `8 MiB` | `8 MiB` | ✅ 一致 |

### 3.2 明确不做的能力

- `mem://` 通道；
- `shm://` 通道；
- C++ `endpoint::watch()` 这类手动引用计数语义；
- C++ `ref_object()` / `unref_object()` — Go 使用 GC，无需手动引用计数；
- C++ `get_evloop()` — Go 使用原生 goroutine 而非 libuv 事件循环；
- C++ `get_crypto_key_exchange_context()` — Go 的 `crypto/ecdh` 标准库是无状态的，
  不需要共享 DH 上下文对象；
- 为 `mem://` / `shm://` 保留的连接回调与时间推进接口；
- `Shutdown` / `on_node_down` 根据回调返回值延迟 `reset()` — C++ 当前实现也不会这样做；
- `Poll()` 与 `connection.Proc()` 的设计差异属正常范围，不应按 `mem://` / `shm://` 模型强行追平。

---

## 四、确认存在的缺失与错误

### 4.1 P0：必须优先修复的已确认问题

> **✅ 截至 2026-04 所有 P0 问题均已修复，且有对应回归测试 (`impl/atbus_node_regression_test.go`, `message_handle/atbus_message_handler_test.go`)。**

| 编号 | 问题 | 状态 | 涉及文件 |
| ------ | ------ | ------ | ---------- |
| P0-1 | `SetDefaultNodeConfigure()` 未初始化 `RetryInterval` | ✅ 已修复 — `RetryInterval = 3s` | `types/atbus_common_types.go` |
| P0-2 | `ReloadCompression()` 成功写入配置后却返回 `EN_ATBUS_ERR_PARAMS` | ✅ 已修复 — 返回 `EN_ATBUS_ERR_SUCCESS` | `impl/atbus_node.go` |
| P0-3 | `Listen()` 使用了反向的 `self` 判定，已初始化节点反而直接返回 `NOT_INITED` | ✅ 已修复 — 正确检查 `n.self == nil` | `impl/atbus_node.go` |
| P0-4 | `GetPeerChannel()` 的状态门槛写反，非 `Created` 状态直接返回 `NOT_INITED` | ✅ 已修复 — 拒绝 `NodeState_Created` | `impl/atbus_node.go` |
| P0-5 | `AddEndpoint()` 拒绝 `owner != nil` 的 endpoint；而 `CreateEndpoint()` 恰恰总会设置 owner | ✅ 已修复 — 检查 `ep.GetOwner() != n` | `impl/atbus_node.go`, `impl/atbus_endpoint.go` |
| P0-6 | `removeChild()` 删除成功后固定返回 `false` | ✅ 已修复 — 成功后返回 `true` | `impl/atbus_node.go` |
| P0-7 | `getConnectionBinding()` 无限递归 | ✅ 已修复 — 委托 `conn.GetBinding()` | `message_handle/atbus_message_handler.go` |
| P0-8 | `SendTransferResponse()` 检查了错误的 body type | ✅ 已修复 — 检查 `DataTransformReq/Rsp` | `message_handle/atbus_message_handler.go` |
| P0-9 | `Proc()` 正常路径未调用 `dispatchAllSelfMessages()` | ✅ 已修复 — 正常路径已调用 | `impl/atbus_node.go` |

### 4.2 P1：功能完整性与公开接口对齐问题

| 编号 | 问题 | 目标 | 涉及文件 |
| ------ | ------ | ------ | ---------- |
| P1-1 | ✅ 根包已新增清晰的 `CreateNode` / `NewNode` 公开入口 | 继续通过根包测试锁定公开用法 | `atbus_node.go`, `atbus_node_test.go` |
| P1-2 | ✅ `Endpoint` / `Connection` 已提供根包公开 helper | 继续通过根包测试锁定创建路径 | `atbus_endpoint.go`, `atbus_connection.go`, `atbus_*_test.go` |
| P1-3 | ✅ 已新增 `RemoveEndpointByID` | 对齐 C++ 的按 `bus_id_t` 删除入口 | `types/atbus_node.go`, `impl/atbus_node.go`, `atbus_node_test.go` |
| P1-4 | ✅ XXTEA 已实现 | 已补齐实现与跨语言回归 | `atframe-utils-go/algorithm/crypto`, `impl/atbus_connection_context.go`, `impl/atbus_connection_context_test.go` |
| P1-5 | ✅ 纯 ChaCha20 已实现 | 已补齐 cipher 列表一致性、pack/unpack 与跨语言向量 | `impl/atbus_connection_context.go`, `impl/atbus_connection_context_test.go` |
| P1-6 | ✅ 已新增全部专项测试文件 | 已建立全面回归网，共 80+ 测试函数覆盖 regression / relationship / reg / msg / setup / endpoint / connection / crypto / topology / transfer | `impl/atbus_node_regression_test.go` (17), `impl/atbus_node_reg_test.go` (19), `impl/atbus_node_msg_test.go` (15), `impl/atbus_node_msg_extended_test.go` (11), `impl/atbus_node_setup_test.go` (5), `impl/atbus_node_relationship_test.go` (3), `impl/atbus_endpoint_test.go` (3), `impl/atbus_connection_test.go` (7) |

### 4.3 已通过专项测试确认的行为边界

以下项目已通过专项测试确认覆盖情况：

- ✅ **ID 冲突**：`TestNodeRegParity_Conflict` 覆盖子网重叠检测；`TestNodeRegParity_RegFailedWithMismatchAccessToken` 覆盖 token 不匹配冲突；
- ✅ **上游切换**：`TestNodeRegParity_ReconnectUpstreamFailed`（重连失败）、`TestNodeRegParity_OnTopologyUpstreamChangeId`（上游 ID 变更）、`TestNodeRegParity_OnTopologyUpstreamClear`（上游丢失）；
- ✅ **跨子网注册**：`TestNodeRegParity_RegPcSuccessCrossSubnet`（跨子网成功）、`TestNodeRegParity_RegPcFailedWithSubnetMismatch`（子网不匹配拒绝）；
- ✅ **冲突路由 / fallback 到上游**：`TestNodeMsgParity_TransferAndConnect`、`TestNodeMsgParity_TransferOnly` 覆盖多跳转发与路由；
- ✅ **上游 ping / reconnect 调度**：`TestNodeMsgParity_PingPong` 覆盖 ping 定时器触发与响应；
- ⚠️ **`fault_tolerant` 计数累积、`OnInvalidConnection` 触发阈值**：Go 使用 goroutine 模型，与 C++ libuv 单线程模型存在设计差异，当前无直接专项测试，属于可选改进；
- ⚠️ **`NodeGetPeerOptions.blacklist`**：当前无直接专项测试，优先级较低。

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
| 纯 ChaCha20 | ✅ | 已补齐实现、协商、pack/unpack 与 C++ 跨语言向量（`data_transform_req` + `custom_cmd`） |
| XXTEA | ✅ | 已实现并有跨语言测试 |

结论：

- ✅ Go 与 C++ 的跨语言互通基础已经具备，全部算法均有实现；
- ✅ 算法接口列表已一致：XXTEA 与纯 ChaCha20 均已补齐；
- ✅ 纯 ChaCha20 的跨语言向量已全覆盖（`TestCrossLangAllEncryptedDataTransformReq` / `TestCrossLangAllEncryptedCustomCmd` 均含 ChaCha20 子 case）；
- `mem://` / `shm://` 不应重新纳入范围。

---

## 六、单元测试计划摘要

详细执行计划见 `UnitTestPlan.md`。这里仅保留最重要的结论：

- 现有 Go 测试已经覆盖 `buffer`、`channel/io_stream`、`channel/utility`、
  `error_code`、`topology`、`message_handle`、`connection_context` 等模块；
- ✅ `Node`、`Endpoint`、`Connection` 的测试空洞已补齐，各有独立测试文件；
- C++ 基准 case 计数：
  - `atbus_message_handler_test.cpp`：**19** 个 case；
  - `atbus_node_msg_test.cpp`：**23** 个 case（含 crypto_config 7 个、multi_level_route 2 个、transfer_failed 2 个），
    Go 侧已全部覆盖（`atbus_node_msg_test.go` 15 项 + `atbus_node_msg_extended_test.go` 11 项 = 26 项）；
  - `atbus_node_reg_test.cpp`：**21** 个 case，其中 `mem_and_send`、`shm_and_send`
    不在 Go 范围内，因此要求覆盖 **19** 个 case；
  - `atbus_node_relationship_test.cpp`：包含 `basic_test`；
  - `atbus_node_reg_test.cpp`：包含 `reg_pc_failed_with_subnet_mismatch`。

已新增的测试文件：

- `impl/atbus_node_regression_test.go`：17 项测试兜住所有 P0 Bug；
- `impl/atbus_connection_test.go`：7 项测试覆盖连接生命周期；
- `impl/atbus_endpoint_test.go`：3 项测试覆盖端点基本行为；
- `impl/atbus_node_setup_test.go`：5 项测试覆盖配置 / 算法 setup；
- `impl/atbus_node_relationship_test.go`：3 项测试覆盖关系与端点增删生命周期；
- `impl/atbus_node_reg_test.go`：19 项测试覆盖注册 / 拓扑 / 超时矩阵；
- `impl/atbus_node_msg_test.go`：15 项测试覆盖消息收发 / loopback / 转发；
- `impl/atbus_node_msg_extended_test.go`：11 项测试覆盖多级拓扑路由 / 转发失败 / 加密集成（crypto config 矩阵）；
- `error_code/libatbus_error_test.go` 与现有 `connection_context` /
  `message_handle` 测试做定向补强。

---

## 七、实施完成状态

1. ✅ P0 代码问题全部修复：默认值、`Node` 关键路径、`message_handle` 两个 Bug；
2. ✅ `impl/atbus_node_regression_test.go` 已补齐，每个 P0 Bug 都有直接回归；
3. ✅ `Endpoint` / `Connection` / `NodeRelationship` / `NodeSetup` 低成本对齐测试已补；
4. ✅ `NodeReg`（19 项）与 `NodeMsg`（15 项）两大网络核心测试集已补；
5. ✅ 根包公开 API 的构造入口已明确，`RemoveEndpointByID` 等长期对齐项已完成；
6. ✅ 纯 ChaCha20 跨语言向量已补齐（`data_transform_req` + `custom_cmd`）；
7. ✅ 行为边界大部分已有专项测试（ID 冲突、上游切换、跨子网注册、ping/reconnect、多跳转发）；
8. ✅ `fault_tolerant` 计数累积与 `OnInvalidConnection` 触发阈值的专项测试已补（`atbus_node_fault_tolerant_test.go`，21 项）；
9. ✅ `NodeGetPeerOptions.blacklist` 路由过滤专项测试已补（`atbus_node_blacklist_test.go`，17 项）；
10. ✅ `error_code` 字符串映射完整性、边界值与 `error` 接口测试已补强（`libatbus_error_test.go`，+5 项）。
11. ✅ 多级拓扑路由（`topology_registry_multi_level_route` / `_reverse`）集成测试已补（`atbus_node_msg_extended_test.go`，2 项）；
12. ✅ 转发失败（`transfer_failed` / `transfer_failed_cross_upstreams`）集成测试已补（`atbus_node_msg_extended_test.go`，2 项）；
13. ✅ 加密配置集成测试（`crypto_config_*`）全部已补（`atbus_node_msg_extended_test.go`，7 项）：
    - `crypto_config_key_exchange_algorithms`（4 subtests: X25519/SECP256R1/SECP384R1/SECP521R1）
    - `crypto_config_cipher_algorithms`（10 subtests: 全部 cipher）
    - `crypto_config_comprehensive_matrix`（40 subtests: 4 × 10 全组合）
    - `crypto_config_multiple_algorithms`（多算法协商）
    - `crypto_config_upstream_downstream`（上下游加密拓扑）
    - `crypto_config_disabled`（明文回退）
    - `crypto_list_available_algorithms`（算法可用性枚举）

### 剩余可选改进

全部已完成。

---

## 八、最终判定标准

| 条件 | 状态 |
| ------ | ------ |
| 1. 全部 P0 问题被修复并有对应回归测试 | ✅ 已达成 |
| 2. `Node` / `Endpoint` / `Connection` 拥有独立测试文件 | ✅ 已达成 |
| 3. `UnitTestPlan.md` 中全部非 `mem://` / `shm://` C++ 场景有 Go 侧 traceability | ✅ 已全面覆盖，含 `fault_tolerant` / `blacklist` / `error_code` / `crypto_config` / `multi_level_route` / `transfer_failed` 补强 |
| 4. 跨语言算法列表与行为完整对齐或有明确结论 | ✅ 全部算法均有实现与跨语言向量（含纯 ChaCha20） |
| 5. 根包公开 API 的创建与删除路径对外可用、可测试 | ✅ 已达成 |

**结论：`libatbus-go` 已进入"接口、行为与跨语言互通全面对齐"状态。**
全部 P0/P1 问题已修复，跨语言向量全覆盖，行为边界均有专项测试。
加密配置集成测试（4 key exchange × 10 cipher 全矩阵）、多级拓扑路由、
转发失败与跨上游转发失败容错均已通过集成测试覆盖。
