# libatbus-go 单元测试执行计划

> 目标：在 **不引入 `mem://` / `shm://`** 的前提下，建立一套可追溯、可执行、
> 不破坏原始测试目的与核心时序的 Go 测试计划，覆盖 C++ `libatbus`
> 当前测试中的全部相关场景。

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
| `impl/atbus_connection_context_test.go` | 握手、压缩、加密、部分跨语言向量 |
| `message_handle/atbus_message_handler_test.go` | body name、AccessData、dispatch 相关 |

当前明显缺失的专项测试文件：

- `impl/atbus_node_regression_test.go`
- `impl/atbus_connection_test.go`
- `impl/atbus_endpoint_test.go`
- `impl/atbus_node_setup_test.go`
- `impl/atbus_node_relationship_test.go`
- `impl/atbus_node_reg_test.go`
- `impl/atbus_node_msg_test.go`

---

## 三、C++ → Go traceability 总表

| C++ 测试文件 | Case 数 | Go 当前状态 | 处理策略 |
| -------------- | --------- | ------------- | ---------- |
| `libatbus_error_test.cpp` | 6 | 部分覆盖 | 在现有 `error_code/libatbus_error_test.go` 中补齐剩余 case |
| `buffer_test.cpp` | 11 | 已覆盖 | 维持现状 |
| `channel_io_stream_tcp_test.cpp` | 8 | 已覆盖 | 维持现状 |
| `channel_io_stream_unix_test.cpp` | 5 | 已覆盖（平台相关） | Windows 上按能力等价或 `t.Skip()` |
| `atbus_topology_test.cpp` | 9 | 已覆盖 | 维持现状 |
| `atbus_endpoint_test.cpp` | 5 | 缺少 dedicated suite | 新增 `impl/atbus_endpoint_test.go`，地址类场景优先复用 `channel/utility` |
| `atbus_connection_context_test.cpp` | 37 | 大部分已覆盖 | 仅补缺口，不重复已有矩阵 |
| `atbus_connection_context_crosslang_generator.cpp` | 10 | 已有部分覆盖 | 视可维护性决定是否拆出 dedicated fixture loader |
| `atbus_access_data_crosslang_generator.cpp` | 8 | 已有部分覆盖 | 视可维护性决定是否拆出 dedicated fixture loader |
| `atbus_message_handler_test.cpp` | 19 | 基本覆盖 | 修完 P0 Bug 后保留并补回归 |
| `atbus_node_setup_test.cpp` | 3 | 缺失 | 新增 `impl/atbus_node_setup_test.go` |
| `atbus_node_relationship_test.cpp` | 3 | 缺失 | 新增 `impl/atbus_node_relationship_test.go`，包含 `basic_test` |
| `atbus_node_reg_test.cpp` | 21（其中 2 个排除） | 缺失 | 新增 `impl/atbus_node_reg_test.go`，覆盖 19 个非 mem/shm case |
| `atbus_node_msg_test.cpp` | 23 | 缺失 | 新增 `impl/atbus_node_msg_test.go`，覆盖全部 23 个 case |

补充说明：

- C++ 没有单独的 `atbus_connection_test.cpp`；
- 但 Go 当前 `atbus_connection` 也没有直接回归文件，因此建议新增
  `impl/atbus_connection_test.go` 做派生回归，把连接生命周期问题从
  `Node` / `Endpoint` 测试里剥出来单测。

---

## 四、建议新增的测试文件与 case 清单

### 4.1 `impl/atbus_node_regression_test.go`

这组不是 C++ 原始 case 的直接映射，而是本次核查确认的 P0 Bug 回归网。
它们应当最先添加、最先通过。

建议子测试：

- `retry_interval_default`
- `reload_compression_returns_success`
- `listen_after_init`
- `get_peer_channel_after_init`
- `add_endpoint_accepts_owned_endpoint`
- `remove_endpoint_returns_success_after_removal`
- `message_handler_get_connection_binding`
- `message_handler_transfer_response_body_case`
- `proc_dispatches_self_messages_in_normal_path`

### 4.2 `impl/atbus_connection_test.go`

建议以连接生命周期为主，做 Go 派生回归：

- `construction_and_address`
- `binding_and_reset`
- `temporary_connection_state`
- `push_status_errors`
- `disconnect_state_transition`

目标：让 `atbus_connection` 的问题不再只能通过 `Node` 集成测试暴露。

### 4.3 `impl/atbus_endpoint_test.go`

建议使用一个顶层 `TestEndpointParity`，并用 `t.Run(...)` 复用 C++ case 名称：

- `connection_basic`
- `endpoint_basic`
- `is_child`
- `get_connection`
- `address`

备注：

- `address` 这项优先 trace 到 `channel/utility/channel_utility_test.go`；
- 如果现有覆盖已经完整，不必在 `impl/atbus_endpoint_test.go` 中重复写一份。

### 4.4 `impl/atbus_node_setup_test.go`

顶层建议：`TestNodeSetupParity`

子测试名称直接与 C++ 对齐：

- `override_listen_path`
- `crypto_algorithms`
- `compression_algorithms`

### 4.5 `impl/atbus_node_relationship_test.go`

顶层建议：`TestNodeRelationshipParity`

子测试：

- `basic_test`
- `copy_conf`
- `child_endpoint_opr`

### 4.6 `impl/atbus_node_reg_test.go`

顶层建议：`TestNodeRegParity`

必须覆盖的 **19** 个非 `mem://` / `shm://` case：

- `reset_and_send_tcp`
- `timeout`
- `message_size_limit`
- `reg_failed_with_mismatch_access_token`
- `reg_failed_with_missing_access_token`
- `reg_failed_with_unsupported`
- `destruct`
- `reg_pc_success`
- `reg_pc_success_cross_subnet`
- `reg_pc_failed_with_subnet_mismatch`
- `reg_bro_success`
- `conflict`
- `reconnect_upstream_failed`
- `set_hostname`
- `on_close_connection_normal`
- `on_close_connection_by_peer`
- `on_topology_upstream_set`
- `on_topology_upstream_clear`
- `on_topology_upstream_change_id`

明确排除：

- `mem_and_send`
- `shm_and_send`

### 4.7 `impl/atbus_node_msg_test.go`

顶层建议：`TestNodeMsgParity`

必须覆盖的 **23** 个 case：

- `ping_pong`
- `custom_cmd`
- `custom_cmd_by_temp_node`
- `send_cmd_to_self`
- `reset_and_send`
- `send_loopback_error`
- `send_msg_to_self_and_need_rsp`
- `upstream_and_downstream`
- `transfer_and_connect`
- `transfer_only`
- `topology_registry_multi_level_route`
- `topology_registry_multi_level_route_reverse`
- `send_failed`
- `transfer_failed`
- `transfer_failed_cross_upstreams`
- `msg_handler_get_body_name`
- `crypto_config_key_exchange_algorithms`
- `crypto_config_cipher_algorithms`
- `crypto_config_comprehensive_matrix`
- `crypto_config_multiple_algorithms`
- `crypto_config_upstream_downstream`
- `crypto_config_disabled`
- `crypto_list_available_algorithms`

### 4.8 现有测试文件的补强项

#### `error_code/libatbus_error_test.go`

需要保证可追溯到以下 C++ case：

- `strerror_known_success`
- `strerror_known_samples`
- `strerror_unknown_thread_local_cache`
- `wstrerror_known_and_unknown`
- `u16_u32_strerror`
- `u8_strerror`

#### `impl/atbus_connection_context_test.go`

仅补 gap-driven case，不重复已有完整矩阵。优先补：

- 未覆盖的 padding / size-limit / invalid-data 边界；
- 无共同算法 / 错误 sequence / 失败路径；
- 对“尚未支持算法”的明确结论测试（例如 XXTEA、纯 ChaCha20）。

#### `message_handle/atbus_message_handler_test.go`

至少新增两个定向回归：

- `get_connection_binding`
- `send_transfer_response_body_case`

### 4.9 跨语言 fixture loader（可选拆分）

如果现有 `connection_context_test.go` / `atbus_message_handler_test.go`
继续扩展后变得难维护，则再考虑拆出：

- `impl/atbus_connection_context_crosslang_test.go`
- `message_handle/atbus_access_data_crosslang_test.go`

前提是：**拆分是为了维护性，不是为了把已有覆盖重复写一遍。**

---

## 五、执行顺序

### 阶段 1：基线确认

1. 先跑现有 Go 测试，确认当前基线仍然可通过；
2. 在修复 P0 Bug 后，优先让 `impl/atbus_node_regression_test.go` 通过。

### 阶段 2：快速回归层

1. `impl/atbus_connection_test.go`
2. `impl/atbus_endpoint_test.go`
3. `impl/atbus_node_setup_test.go`
4. `impl/atbus_node_relationship_test.go`

### 阶段 3：核心网络层

1. `impl/atbus_node_reg_test.go`
2. `impl/atbus_node_msg_test.go`

### 阶段 4：补强与收口

1. `error_code/libatbus_error_test.go` 补齐剩余 traceability；
2. `impl/atbus_connection_context_test.go` 做 gap-driven 补强；
3. 如有必要，再拆分跨语言 fixture loader。

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

当以下条件全部满足时，可以认为 Go 测试计划已经把 C++ 相关场景接住：

1. `UnitTestPlan.md` 中列出的所有非 `mem://` / `shm://` C++ case 都有 Go traceability；
2. 每个已确认的 P0 Bug 都有单独回归测试；
3. `Node` / `Endpoint` / `Connection` 不再依赖“旁路覆盖”；
4. 跨语言已有覆盖被保留，剩余算法缺口不会被静默忽略；
5. 新增网络测试不会破坏原有测试目的，也不会把关键时序偷换成 mock。
