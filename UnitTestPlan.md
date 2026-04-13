# libatbus-go 单元测试执行计划

> 目标：在 **不引入 `mem://` / `shm://`** 的前提下，建立一套可追溯、可执行、
> 不破坏原始测试目的与核心时序的 Go 测试计划，覆盖 C++ `libatbus`
> 当前测试中的全部相关场景。
> 2026-04 复核结论：全部非 `mem://` / `shm://` 相关已确认差异已收口，
> 当前 `go test ./...` 通过；`parse_*` helper API 已补 direct parity，
> `OnInvalidConnection` 的 connecting timeout 场景也已对齐为 C++ 单回调行为。

---

## 一、范围与执行原则

### 1.1 覆盖范围

- 以 `atsf4g-co/atframework/libatbus/test/case/` 中现有 C++ case 为基准；
- 明确排除 `mem://` 与 `shm://` 相关 case；
- 重点补齐 `atbus_node`、`atbus_endpoint`、`atbus_connection`；
- 对已经存在的 Go 测试，只做**补强**，不做无意义重复。

### 1.2 执行原则

- **保持 case 语义一致**：能复用 C++ case 名称的，优先用相同名称作为 `t.Run(...)` 名；
- **保持核心时序目的**：不要用“直接改内部状态”替代真实注册 / 握手 / 路由过程；
  但如果某个 case 的核心目标是 **消息编解码 + 回调顺序 + request/response 语义**，
  可以使用成对的 loopback/mock connection 保留 pack/unpack 与消息流转，而不强制依赖真实 socket；
- **缩短墙钟时间，但不改协议顺序**：优先通过 `Proc(now)` 驱动时间推进，
  只在 I/O flush 必须时使用短时间等待；
- **网络重测试串行执行**：`node_reg` 与 `node_msg` 不并行跑；
- **平台差异显式处理**：Unix socket 相关场景在 Windows 上使用等价能力，
  若无等价能力则 `t.Skip()`，不要偷偷改测试目的。

---

## 二、现有 Go 覆盖基线

当前仓库已经存在并且应继续保留的测试基线：

| Go 测试文件 | 当前作用 |
| ------------- | ---------- |
| `buffer/static_buffer_block_test.go` | buffer block 行为 |
| `buffer/buffer_block_test.go` | buffer block 管理 |
| `buffer/buffer_algorithm_test.go` | buffer 算法与边界 |
| `channel/io_stream/frame_codec_test.go` | 帧编解码 |
| `channel/io_stream/channel_io_stream_test.go` | TCP / Unix / pipe 风格 I/O 流 |
| `channel/utility/channel_utility_test.go` | 地址解析、schema、优先级辅助 |
| `error_code/libatbus_error_test.go` | 错误码字符串映射（部分） |
| `impl/atbus_topology_test.go` | topology registry / relation |
| `impl/atbus_connection_context_test.go` | 握手、压缩、加密、跨语言向量（含纯 ChaCha20） |
| `message_handle/atbus_message_handler_test.go` | body name、AccessData、dispatch 相关 |

已新增的专项测试文件（自初版计划后新增）：

| Go 测试文件 | 当前作用 |
| ------------- | ---------- |
| `impl/atbus_node_regression_test.go` | P0 Bug 回归（17 项测试） |
| `impl/atbus_node_setup_test.go` | 节点配置 / 算法 setup（5 项测试） |
| `impl/atbus_node_relationship_test.go` | 节点关系与端点增删生命周期（3 项测试） |
| `impl/atbus_node_reg_test.go` | 注册 / 拓扑 / 超时（19 项测试） |
| `impl/atbus_node_msg_test.go` | 消息收发 / 转发 / loopback（15 项测试） |
| `impl/atbus_node_msg_extended_test.go` | 多级拓扑路由 / 转发失败 / 加密集成（11 项测试） |
| `impl/atbus_endpoint_test.go` | 端点基本行为（3 项测试） |
| `impl/atbus_connection_test.go` | 连接生命周期（7 项测试） |

---

## 三、C++ → Go traceability 总表

| C++ 测试文件 | Case 数 | Go 当前状态 | 处理策略 |
| -------------- | --------- | ------------- | ---------- |
| `libatbus_error_test.cpp` | 6 | ✅ 已覆盖 | `error_code/libatbus_error_test.go` 已覆盖已知/未知码、哨兵值、完整性校验、格式一致性与 `error` 接口 |
| `buffer_test.cpp` | 11 | 已覆盖 | 维持现状 |
| `channel_io_stream_tcp_test.cpp` | 8 | 已覆盖 | 维持现状 |
| `channel_io_stream_unix_test.cpp` | 5 | 已覆盖（平台相关） | Windows 上按能力等价或 `t.Skip()` |
| `atbus_topology_test.cpp` | 9 | 已覆盖 | 维持现状 |
| `atbus_endpoint_test.cpp` | 5 | ✅ 已覆盖（分散落点） | `impl/atbus_endpoint_test.go` 覆盖 `connection_basic` / `endpoint_basic` / `get_connection`，`impl/atbus_topology_test.go` 覆盖关系类断言，`channel/utility/channel_utility_test.go` 覆盖 `address` |
| `atbus_connection_context_test.cpp` | 37 | ✅ 已覆盖 | 95+ 测试函数，包含全部算法的跨语言向量 |
| `atbus_connection_context_crosslang_generator.cpp` | 10 | ✅ 已覆盖 | `TestCrossLangAllEncryptedDataTransformReq` + `TestCrossLangAllEncryptedCustomCmd` 覆盖全部算法（含纯 ChaCha20） |
| `atbus_access_data_crosslang_generator.cpp` | 8 | ✅ 已覆盖 | AccessData plaintext / HMAC 跨语言向量已在 |
| `atbus_message_handler_test.cpp` | 19 | ✅ 已覆盖 | P0 Bug 已修复并有回归测试 |
| `atbus_node_setup_test.cpp` | 3 | ✅ direct parity + 语义覆盖 | `impl/atbus_node_setup_test.go` 覆盖 override_listen_path / crypto / compression / key_exchange / reload_crypto，且 `atbus_node_test.go` + `types.Parse*AlgorithmName()` 补齐 C++ `parse_*` helper 的 direct parity |
| `atbus_node_relationship_test.cpp` | 3 | ✅ 已覆盖 | `impl/atbus_node_relationship_test.go` (3 项测试：copy_conf、child_endpoint_opr、endpoint_events) |
| `atbus_node_reg_test.cpp` | 21（其中 2 个排除） | ✅ 已覆盖 | `impl/atbus_node_reg_test.go` (19 项测试，覆盖全部非 `mem://` / `shm://` 相关 case：set_hostname / reset_and_send / timeout / message_size_limit / reg_failed / reg_success / destruct / conflict / reconnect / topology_changes 等) |
| `atbus_node_msg_test.cpp` | 23 | ✅ 已全部覆盖 | `impl/atbus_node_msg_test.go` (15 项) + `impl/atbus_node_msg_extended_test.go` (11 项: multi_level_route ×2, transfer_failed ×2, crypto_config ×7) |

补充说明：

- C++ 没有单独的 `atbus_connection_test.cpp`；
- Go 已新增 `impl/atbus_connection_test.go` (7 项测试) 做派生回归，
  将连接生命周期问题从 `Node` / `Endpoint` 测试里剥出单测。

---

## 四、建议新增的测试文件与 case 清单

### 4.1 `impl/atbus_node_regression_test.go` ✅ 已完成

P0 Bug 回归网，17 项测试已全部通过。

### 4.2 `impl/atbus_connection_test.go` ✅ 已完成

连接生命周期派生回归，7 项测试已全部通过。

### 4.3 `impl/atbus_endpoint_test.go` ✅ 已完成

端点基本行为，3 项测试已全部通过。

### 4.4 `impl/atbus_node_setup_test.go` ✅ 已完成

5 项测试已全部通过，覆盖 override_listen_path / crypto / compression / key_exchange / reload_crypto。

### 4.5 `impl/atbus_node_relationship_test.go` ✅ 已完成

3 项测试已全部通过，覆盖 copy_conf / child_endpoint_opr / endpoint_events。

### 4.6 `impl/atbus_node_reg_test.go` ✅ 已完成

19 项测试已全部通过。

### 4.7 `impl/atbus_node_msg_test.go` ✅ 已完成

15 项测试已全部通过，覆盖 custom_cmd / send_cmd_to_self /
send_msg_to_self_and_need_rsp / send_data / upstream_downstream /
ping_pong / transfer / loopback / crypto 等。

### 4.8 `impl/atbus_node_msg_extended_test.go` ✅ 已完成

11 项测试已全部通过，覆盖 C++ `atbus_node_msg_test.cpp` 中之前缺失的场景：

- `topology_registry_multi_level_route` — 3 节点链式多级路由（upstream→mid→downstream）
- `topology_registry_multi_level_route_reverse` — 反向多级路由（downstream→upstream）
- `transfer_failed` — 转发到不存在节点的失败响应
- `transfer_failed_cross_upstreams` — 跨上游转发失败后本地连接不断开
- `crypto_config_key_exchange_algorithms` — 4 种密钥交换算法的集成测试（含 subtests）
- `crypto_config_cipher_algorithms` — 10 种加密算法的集成测试（含 subtests）
- `crypto_config_comprehensive_matrix` — 4 × 10 = 40 种组合的全矩阵测试
- `crypto_config_multiple_algorithms` — 多算法优先级协商
- `crypto_config_upstream_downstream` — 上下游加密拓扑双向消息
- `crypto_config_disabled` — 明文回退
- `crypto_list_available_algorithms` — 算法可用性枚举

### 4.9 现有测试文件的补强项

#### `error_code/libatbus_error_test.go`

需要保证可追溯到以下 C++ case：

- `strerror_known_success`
- `strerror_known_samples`
- `strerror_unknown_thread_local_cache`
- `wstrerror_known_and_unknown`
- `u16_u32_strerror`
- `u8_strerror`

#### `impl/atbus_connection_context_test.go` ✅ 已完成

95+ 测试函数，全部算法（含 XXTEA、纯 ChaCha20）均有跨语言向量覆盖。

#### `message_handle/atbus_message_handler_test.go` ✅ 已补强

已补以下定向回归：

- `get_connection_binding`
- `send_transfer_response_body_case`

### 4.10 `impl/atbus_node_setup_test.go` ✅ 已收口 direct API parity

- C++ `atbus_node_setup_test.cpp` 的 `crypto_algorithms` /
  `compression_algorithms` 直接测试
  `node::parse_crypto_algorithm_name()` /
  `node::parse_compression_algorithm_name()`；
- Go 现已补齐 `types.ParseCryptoAlgorithmName()` /
  `types.ParseCompressionAlgorithmName()`，并由根包
  `ParseCryptoAlgorithmName()` /
  `ParseCompressionAlgorithmName()` 对外公开；
- `impl/atbus_node_setup_test.go` 现同时验证 parse helper parity 与原有元数据/能力语义，
  `atbus_node_test.go` 进一步锁定根包公开 helper 的 direct API parity。

### 4.11 `impl/atbus_node_fault_tolerant_test.go` ✅ 已对齐回调语义

- `TestOnInvalidConnection_FiresOnConnectingTimeout` 现明确断言
  Go 在 connecting timeout 场景只触发**一次** `OnInvalidConnection`；
- `impl/atbus_node.go` 的 timeout 处理已改为在 `Reset()` 后复查当前 front entry，
  与 C++ `node::proc()` 的保护性清理语义一致；
- 该 case 不再固化 Go 特有双回调行为，而是直接作为 C++ parity 断言。

### 4.12 `atbus_endpoint_test.cpp` 的 traceability 已写清“分散覆盖”

- `connection_basic` / `endpoint_basic` / `get_connection` 主要对应
  `impl/atbus_endpoint_test.go`；
- `is_child` / 关系类断言主要落在 `impl/atbus_topology_test.go` 与相关拓扑集成测试；
- `address` 对应 `channel/utility/channel_utility_test.go`；
- 已在本文档和 `impl/atbus_endpoint_test.go` 顶部注释中明确这些落点；
- 因此该 C++ 文件应表述为“Go 已覆盖，但 coverage 分散在多个测试文件中”。

---

## 五、执行顺序

### 阶段 1：基线确认 ✅ 已完成

全部现有 Go 测试通过，P0 Bug 已修复，`impl/atbus_node_regression_test.go` 17 项回归测试通过。

### 阶段 2：快速回归层 ✅ 已完成

1. `impl/atbus_connection_test.go` — 7 项测试
2. `impl/atbus_endpoint_test.go` — 3 项测试
3. `impl/atbus_node_setup_test.go` — 5 项测试
4. `impl/atbus_node_relationship_test.go` — 3 项测试

### 阶段 3：核心网络层 ✅ 已完成

1. `impl/atbus_node_reg_test.go` — 19 项测试
2. `impl/atbus_node_msg_test.go` — 15 项测试
3. `impl/atbus_node_msg_extended_test.go` — 11 项测试（多级路由 / 转发失败 / 加密集成）

### 阶段 4：补强与收口 ✅ 已完成

1. ✅ `error_code/libatbus_error_test.go` — 补齐 `EN_ATBUS_ERR_MIN` 边界、全常量完整性校验、`error` 接口、格式一致性（+5 项）；
2. ✅ `impl/atbus_connection_context_test.go` — 跨语言向量已全覆盖；
3. ✅ `impl/atbus_node_fault_tolerant_test.go` — `addEndpointFault`/`addConnectionFault` 阈值、`OnInvalidConnection` 回调触发（21 项）；
4. ✅ `impl/atbus_node_blacklist_test.go` — `isInGetPeerBlacklist`、`GetPeerChannel` 黑名单路由过滤（17 项）。

### 阶段 5：差异收口 ✅ 已完成

1. ✅ 已补 `parse_crypto_algorithm_name` /
  `parse_compression_algorithm_name` 等价 helper，并补 direct API tests；
2. ✅ connecting timeout 的 `OnInvalidConnection` case 已改成单回调 parity 断言；
3. ✅ `atbus_endpoint_test.cpp` 的 5 个 C++ case Go 落点已在文档与测试文件顶部注释中写清。

---

## 六、保持测试目的与时序的规则

### 6.1 通用规则

- 网络测试使用唯一端口 / 唯一路径分配器；
- `defer Reset()` 负责资源清理；
- 对 goroutine / channel / socket 清理做显式等待，避免幽灵资源影响后续 case；
- `node_reg` 与 `node_msg` 不并行执行。

### 6.2 时间推进规则

- 对 timeout / retry / ping 等场景，优先通过 `Proc(now)` 显式推进时间；
- 只在底层 I/O flush 或 goroutine 调度不可避免时做短暂 wall-clock wait；
- 不为了“让测试快一点”而绕过真实注册 / 握手 / 响应顺序。

### 6.3 Windows 规则

- 不能强行把 Unix socket 的测试目标改成另一个完全不同的语义；
- 若 named pipe 可以表达同一测试目的，则可使用 named pipe；
- 若做不到完全等价，则 `t.Skip()`，并在注释中说明原因。

### 6.4 命名与 traceability 规则

- 顶层 Go 测试名可使用 `TestNodeRegParity` / `TestNodeMsgParity` 等；
- 子测试名优先直接复用 C++ case 名称；
- 每个新测试文件顶部附注其来源 C++ 文件，方便长期维护。

---

## 七、完成标准

| 条件 | 状态 |
| ------ | ------ |
| 1. 所有非 `mem://` / `shm://` C++ case 有 Go traceability | ✅ 已完成 |
| 2. 每个 P0 Bug 有单独回归测试 | ✅ 17 项回归测试 |
| 3. `Node` / `Endpoint` / `Connection` 有独立测试文件 | ✅ 各有 dedicated suite |
| 4. 跨语言覆盖完整，无静默忽略的算法缺口 | ✅ 含纯 ChaCha20 / XXTEA |
| 5. 新增测试保持原有测试目的与时序 | ✅ 已完成；`OnInvalidConnection` timeout 场景已收口到 C++ parity |
