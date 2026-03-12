# libatbus-go 实现计划

> 目标：参照 C++ 版本 libatbus，使用 Go 原生网络库和 goroutine 完成同等功能实现，并确保单元测试覆盖所有 C++ 测试用例。

## 一、项目概述

### 1.1 C++ 仓库位置
`D:\workspace\git\github\atframework\atsf4g-co\atframework\libatbus`

### 1.2 Go 仓库位置
`D:\workspace\git\github\atsf4g-go\atframework\libatbus-go`

### 1.3 核心设计差异
| 特性 | C++ 版本 | Go 版本 |
|------|----------|---------|
| IO 模型 | libuv (回调模式) | Go net 库 + goroutine |
| 事件循环 | uv_run / uv_loop | context + goroutine 协作 |
| 内存通道 | 支持 (mem/shm) | 不支持 (仅 iostream) |
| 连接管理 | 单线程+回调 | 可并发 (需同步保护) |

### 1.4 ⚠️ 跨语言互通要求 (关键约束)

> **本项目的首要目标是确保 Go 版本能够与 C++ 版本完全互通。**

这意味着：
1. **协议格式必须完全一致** - 消息帧、Protobuf 序列化、字段顺序都必须与 C++ 版本保持二进制兼容
2. **握手流程必须兼容** - 注册、认证、加密协商流程必须与 C++ 版本完全一致
3. **拓扑关系必须互认** - Go 节点和 C++ 节点可以在同一集群中混合部署
4. **错误码必须统一** - 使用相同的错误码定义

#### 互通场景示例
```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   C++ atproxy   │◄───►│   Go lobbysvr   │◄───►│   C++ gamesvr   │
│   (上游节点)     │     │   (本项目实现)   │     │   (兄弟节点)     │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

#### 必须验证的互通测试
- [ ] Go 节点注册到 C++ 上游节点
- [ ] C++ 节点注册到 Go 上游节点
- [ ] Go 与 C++ 节点之间的消息收发
- [ ] 混合部署下的路由转发
- [ ] 加密通道的跨语言握手
- [ ] 自定义命令的跨语言调用

---

## 二、现有 Go 代码分析

### 2.1 已实现部分 (impl/)

#### atbus_node.go (~1700 行)
- ✅ Node 结构体定义及基础字段
- ✅ 状态管理 (NodeState, NodeFlag)
- ✅ 配置管理 (NodeConfigure)
- ✅ 事件回调框架 (NodeEventHandleSet)
- ✅ 拓扑管理基础 (TopologyRegistry)
- ✅ 部分 API 实现:
  - `Init` - **待实现**
  - `ReloadCrypto` - ✅ 已实现
  - `ReloadCompression` - ✅ 已实现（当前返回码疑似错误）
  - `Start/StartWithConfigure` - 部分实现
  - `Reset` - **待实现**
  - `Proc` - **待实现**
  - `Poll` - **待实现**
  - `Listen` - 骨架实现，需完善
  - `Connect/ConnectWithEndpoint` - 骨架实现，需完善
  - `Disconnect` - ✅ 已实现
  - `SendData/SendDataWithOptions` - 部分实现（依赖 SendDataMessage）
  - `SendCustomCommand` - ✅ 已实现
  - `GetPeerChannel` - ✅ 已实现
  - 各种 Getter/Setter - ✅ 已实现
  - 事件回调 On* 系列 - ✅ 已实现
- ❌ 缺失:
  - SendDataMessage / SendCtrlMessage
  - 消息派发逻辑 (`DispatchAllSelfMessages`)
  - 完整的 `Proc` 循环
  - ping/pong 定时器处理
  - 连接超时管理
  - `GetContext`
  - `removeConnectionTimer`

#### atbus_connection.go (~165 行)
- ✅ Connection 结构体定义
- ✅ 状态管理 (ConnectionState, ConnectionFlag)
- ✅ `CreateConnection` 骨架
- ❌ **核心方法待实现**:
  - `Reset` - 骨架实现
  - `Proc` - TODO
  - `Listen` - TODO
  - `Connect` - TODO
  - `Disconnect` - TODO
  - `Push` - TODO
  - IO 回调集成

#### atbus_endpoint.go (~590 行)
- ✅ Endpoint 结构体定义
- ✅ 连接管理 (AddConnection/RemoveConnection)
- ✅ 监听地址管理
- ❌ `CreateEndpoint` 返回 nil (待实现)
- ❌ ping timer 相关逻辑待完善

#### atbus_connection_context.go
- ✅ 加密握手上下文
- ✅ 密钥交换支持

#### atbus_topology.go
- ✅ 拓扑关系管理
- ✅ TopologyRegistry 实现

### 2.2 类型定义 (types/)
- ✅ atbus_node.go - Node 接口及配置
- ✅ atbus_connection.go - Connection 接口
- ✅ atbus_endpoint.go - Endpoint 接口
- ✅ atbus_topology.go - 拓扑类型
- ✅ atbus_message.go - 消息类型
- ✅ channel_address.go - 地址解析接口

### 2.3 channel/ 目录
- ✅ utility/ - 地址解析工具
- ❌ io_stream/ - **目录缺失，需实现**

---

## 三、C++ 核心功能分析

### 3.1 atbus_node (include/atbus_node.h + src/atbus_node.cpp, ~3700行)

#### 核心数据结构
```cpp
struct conf_t {
    adapter::loop_t *ev_loop;
    std::string upstream_address;
    std::unordered_map<std::string, std::string> topology_labels;
    int32_t loop_times;              // 消息循环次数限制
    int32_t ttl;                     // 消息转发跳转限制
    std::chrono::microseconds first_idle_timeout;
    std::chrono::microseconds ping_interval;
    std::chrono::microseconds retry_interval;
    size_t fault_tolerant;
    size_t message_size;
    size_t receive_buffer_size;
    size_t send_buffer_size;
    // 加密/压缩配置...
};

struct evt_timer_t {
    std::chrono::system_clock::time_point tick;
    std::chrono::system_clock::time_point upstream_op_timepoint;
    timer_desc_ls<const endpoint*, weak_ptr<endpoint>>::type ping_list;
    timer_desc_ls<std::string, connection::ptr_t>::type connecting_list;
    std::list<endpoint::ptr_t> pending_endpoint_gc_list;
    std::list<connection::ptr_t> pending_connection_gc_list;
};
```

#### 关键方法
1. **init()** - 初始化节点配置、创建 self endpoint、分配静态缓冲区
2. **start()** - 启动节点，连接上游
3. **reset()** - 清理所有连接和 endpoint，执行 GC
4. **proc()** - 主循环：
   - 处理所有 proc_connection
   - 处理上游超时/重连
   - 处理 ping 定时器
   - 处理连接超时
   - 执行 GC
5. **poll()** - 执行一次 libuv 事件循环
6. **listen()** - 创建监听连接
7. **connect()** - 创建出站连接
8. **send_data() / send_data_message()** - 发送数据消息
9. **send_ctrl_message()** - 发送控制消息
10. **get_peer_channel()** - 查找目标通道
11. **on_receive_message()** - 消息接收处理入口

### 3.2 atbus_connection (include/atbus_connection.h + src/atbus_connection.cpp)

#### 核心数据结构
```cpp
struct connection_data_t {
    union shared_t {
        conn_data_mem mem;  // 内存通道 (Go 不需要)
        conn_data_shm shm;  // 共享内存通道 (Go 不需要)
        conn_data_ios ios_fd;  // IO流通道 (Go 需要)
    };
    proc_fn_t proc_fn;   // 处理函数指针
    free_fn_t free_fn;   // 释放函数指针
    push_fn_t push_fn;   // 发送函数指针
};
```

#### 关键方法
1. **create()** - 创建连接，根据协议类型初始化不同的通道
2. **listen()** - 监听地址
3. **connect()** - 连接到目标
4. **push()** - 发送数据
5. **proc()** - 处理连接数据 (内存通道专用)
6. **reset()** - 重置连接

#### iostream 回调 (Go 需要转为 goroutine)
- `iostream_on_listen_cb` - 监听成功
- `iostream_on_connected_cb` - 连接建立
- `iostream_on_receive_cb` - 数据接收
- `iostream_on_accepted` - 接受新连接
- `iostream_on_connected` - 主动连接成功
- `iostream_on_disconnected` - 连接断开
- `iostream_on_written` - 写入完成

### 3.3 channel_io_stream (src/channel_io_stream.cpp, ~2040行)

#### 核心功能
1. **io_stream_init()** - 初始化通道
2. **io_stream_close()** - 关闭通道
3. **io_stream_run()** - 运行事件循环
4. **io_stream_listen()** - 监听地址 (TCP/Unix/Pipe)
5. **io_stream_connect()** - 连接目标
6. **io_stream_send()** - 发送数据
7. **io_stream_disconnect()** - 断开连接

#### 数据帧格式
```
+----------------+----------------+----------------+
| Hash (4 bytes) | Length (varint)| Payload        |
+----------------+----------------+----------------+
```

#### 地址支持
- `ipv4://host:port` - IPv4/IPv6 TCP
- `ipv6://host:port` - IPv4/IPv6 TCP
- `atcp://host:port` - IPv4/IPv6 TCP
- `dns://host:port` - DNS 解析后的 TCP
- `unix://path` - Unix domain socket
- `pipe://path` - Named pipe (类似 unix)

---

## 四、C++ 单元测试分析

### 4.1 atbus_node_setup_test.cpp
| 测试用例 | 描述 | 状态 |
|----------|------|------|
| override_listen_path | 测试覆盖已存在的监听路径 | 待实现 |
| crypto_algorithms | 测试加密算法解析 | 待实现 |
| compression_algorithms | 测试压缩算法解析 | 待实现 |

### 4.2 atbus_node_reg_test.cpp (~2233行)
| 测试用例 | 描述 | 状态 |
|----------|------|------|
| reset_and_send_tcp | 主动reset流程 + TCP收发 | 待实现 |
| reg_and_send_unix | Unix socket 注册和收发 | 待实现 |
| reg_with_different_access_token | 不同 access token 注册 | 待实现 |
| reconnect_same_addr | 同地址重连测试 | 待实现 |
| reg_timeout_connection | 连接超时测试 | 待实现 |
| reg_cancel_before_connected | 连接前取消 | 待实现 |
| ... | (更多测试用例) | 待实现 |

### 4.3 atbus_node_msg_test.cpp (~2279行)
| 测试用例 | 描述 | 状态 |
|----------|------|------|
| send_to_self | 发送给自己 | 待实现 |
| send_to_sibling | 发送给兄弟节点 | 待实现 |
| send_data_forward | 数据转发 | 待实现 |
| send_with_router | 带路由转发 | 待实现 |
| custom_command | 自定义命令 | 待实现 |
| ping_pong | ping/pong 测试 | 待实现 |
| ... | (更多测试用例) | 待实现 |

### 4.4 atbus_node_relationship_test.cpp
| 测试用例 | 描述 | 状态 |
|----------|------|------|
| copy_conf | 配置复制 | 待实现 |
| child_endpoint_opr | 子节点端点操作 | 待实现 |

### 4.5 atbus_endpoint_test.cpp
| 测试用例 | 描述 | 状态 |
|----------|------|------|
| connection_basic | 连接基础测试 | 待实现 |
| endpoint_basic | 端点基础测试 | 待实现 |
| is_child | 子节点关系判断 | 待实现 |
| get_connection | 获取连接测试 | 待实现 |
| address | 地址解析测试 | 待实现 |

---

## 五、实现计划

### Phase 1: IO Stream 通道实现 (预估: 3-5天)

#### 5.1.1 创建 channel/io_stream/channel_io_stream.go
```go
// 核心结构
type IoStreamChannel struct {
    conf        IoStreamConf
    ctx         context.Context
    cancel      context.CancelFunc
    listeners   map[string]net.Listener
    connections map[string]*IoStreamConnection
    callbacks   IoStreamCallbacks
    mu          sync.RWMutex
}

type IoStreamConnection struct {
    channel  *IoStreamChannel
    conn     net.Conn
    address  ChannelAddress
    status   ConnectionState
    readBuf  *buffer.DynamicBuffer
    writeBuf chan []byte
    // ...
}

// 核心接口
func NewIoStreamChannel(ctx context.Context, conf *IoStreamConf) *IoStreamChannel
func (c *IoStreamChannel) Listen(addr string, callback IoStreamCallback) error
func (c *IoStreamChannel) Connect(addr string, callback IoStreamCallback) error
func (c *IoStreamChannel) Send(conn *IoStreamConnection, data []byte) error
func (c *IoStreamChannel) Close() error
```

#### 5.1.2 实现数据帧编解码
- 参考 C++ 的 hash + varint length + payload 格式
- 使用 murmur3_x86_32(hash seed = 0) 对 **payload** 计算校验
- varint 采用 libatbus 自定义 vint（`buffer.ReadVint/WriteVint`），不是 protobuf 标准 varint
- 接收端需统计 hash/size 校验失败次数，超过 `MaxReadCheck*` 阈值应断开连接
- 实现 `PackFrame()` 和 `UnpackFrame()`

#### 5.1.3 实现地址解析
- 支持 ipv4/ipv6/atcp/dns/unix/pipe 协议
- 复用现有 channel/utility 代码

### Phase 2: Connection 实现完善 (预估: 2-3天)

#### 5.2.1 补全 impl/atbus_connection.go
```go
func (c *Connection) Listen() ErrorType {
    // 1. 获取 IoStreamChannel
    // 2. 调用 channel.Listen()
    // 3. 设置回调处理接受新连接
    // 4. 更新状态
}

func (c *Connection) Connect() ErrorType {
    // 1. 获取 IoStreamChannel
    // 2. 调用 channel.Connect()
    // 3. 设置回调处理连接结果
    // 4. 更新状态，加入 connecting_list
}

func (c *Connection) Push(buffer []byte) ErrorType {
    // 1. 打包数据帧
    // 2. 调用 channel.Send()
    // 3. 更新统计
}

func (c *Connection) Proc() ErrorType {
    // iostream 模式不需要主动 proc，由 goroutine 驱动
    return EN_ATBUS_ERR_SUCCESS
}
```

#### 5.2.2 实现 iostream 回调转换
```go
// 将 C++ 回调模式转为 Go channel/goroutine 模式
type connectionEventHandler struct {
    onAccepted    chan *IoStreamConnection
    onConnected   chan *IoStreamConnection
    onReceive     chan receiveEvent
    onDisconnect  chan disconnectEvent
}
```

### Phase 3: Node 核心功能完善 (预估: 3-4天)

#### 5.3.1 实现 Node.Init()
```go
func (n *Node) Init(id BusIdType, conf *NodeConfigure) ErrorType {
    // 1. 配置初始化
    // 2. 创建 IoStreamChannel
    // 3. 创建 self endpoint
    // 4. 初始化拓扑注册表
    // 5. 分配消息缓冲区
    // 6. 设置状态为 Inited
}
```

#### 5.3.2 实现 Node.Proc()
```go
func (n *Node) Proc(now time.Time) ErrorType {
    // 1. 更新 tick
    // 2. 处理 self messages (发给自己的消息)
    // 3. 处理上游连接超时/重连
    // 4. 处理 ping 定时器
    // 5. 处理连接超时
    // 6. 执行 GC
}
```

#### 5.3.3 实现 Node.Reset()
```go
func (n *Node) Reset() ErrorType {
    // 1. 设置 Resetting 标志
    // 2. 派发所有 self messages
    // 3. 断开所有连接
    // 4. 清理所有 endpoint
    // 5. 释放 IoStreamChannel
    // 6. 重置状态
}
```

#### 5.3.4 完善消息发送
```go
func (n *Node) SendDataMessage(tid BusIdType, msg *Message, options *NodeSendDataOptions) (ErrorType, Endpoint, Connection) {
    // 1. 获取目标通道 (GetPeerChannel)
    // 2. 序列化消息
    // 3. 调用 connection.Push()
    // 4. 处理发送结果
}
```

### Phase 4: Endpoint 完善 (预估: 1-2天)

#### 5.4.1 实现 CreateEndpoint()
```go
func CreateEndpoint(owner *Node, id BusIdType, pid int32, hostname string) *Endpoint {
    ep := &Endpoint{
        id:       id,
        pid:      pid,
        hostname: hostname,
        owner:    owner,
        dataConn: make([]Connection, 0),
        stat:     endpointStatistic{CreatedTime: time.Now()},
    }
    return ep
}
```

#### 5.4.2 完善 ping/pong 定时器
```go
func (e *Endpoint) AddPingTimer() bool {
    // 添加到 node 的 ping_list
}

func (e *Endpoint) ClearPingTimer() {
    // 从 node 的 ping_list 移除
}

func (e *Endpoint) UpdatePingStatistic(delay time.Duration) {
    // 更新 ping 延迟统计
}
```

### Phase 5: 单元测试 (预估: 4-5天)

#### 5.5.1 测试文件结构
```
impl/
├── atbus_node_test.go           # 对应 atbus_node_setup_test.cpp
├── atbus_node_reg_test.go       # 对应 atbus_node_reg_test.cpp
├── atbus_node_msg_test.go       # 对应 atbus_node_msg_test.cpp
├── atbus_node_relationship_test.go  # 对应 atbus_node_relationship_test.cpp
├── atbus_endpoint_test.go       # 对应 atbus_endpoint_test.cpp
└── testdata/                    # 测试数据
```

#### 5.5.2 测试辅助工具
```go
// test_utils.go
func SetupTestNode(id BusIdType, port int) (*Node, func())
func WaitUntil(condition func() bool, timeout time.Duration) bool
func CreateTestConf() *NodeConfigure
```

#### 5.5.3 测试用例实现顺序
1. **基础测试** - endpoint_basic, connection_basic, address
2. **节点设置** - crypto_algorithms, compression_algorithms
3. **注册流程** - reset_and_send_tcp, reg_and_send_unix
4. **消息收发** - send_to_self, send_to_sibling, send_data_forward
5. **高级功能** - custom_command, ping_pong, 拓扑关系
6. **边界情况** - 超时、重连、错误处理

### Phase 6: 跨语言互通测试 (预估: 2-3天) ⭐

#### 5.6.1 测试文件结构
```
integration/
├── crosslang_reg_test.go        # 跨语言注册测试
├── crosslang_msg_test.go        # 跨语言消息收发测试
├── crosslang_crypto_test.go     # 跨语言加密通道测试
├── crosslang_route_test.go      # 跨语言路由转发测试
└── crosslang_test_utils.go      # 测试辅助工具
```

> **注意**: 跨语言互通测试使用 C++ 的 atapp echo server (atsf4g-co 项目提供)，Go 项目不需要编译 C++ 代码。

#### 5.6.2 互通测试场景
| 测试 ID | 场景 | Go 角色 | C++ 角色 |
|---------|------|---------|----------|
| CL-001 | 基础注册 | 客户端 | 服务端(上游) |
| CL-002 | 反向注册 | 服务端(上游) | 客户端 |
| CL-003 | 消息收发 | 发送方 | 接收方 |
| CL-004 | 消息收发 | 接收方 | 发送方 |
| CL-005 | 路由转发 | 中间节点 | 源/目标 |
| CL-006 | 加密通道 | 客户端 | 服务端 |
| CL-007 | 自定义命令 | 发起方 | 响应方 |
| CL-008 | Ping/Pong | 发起方 | 响应方 |

#### 5.6.3 互通测试辅助工具
```go
// crosslang_test_utils.go

// StartAtAppEchoServer 启动 C++ atapp echo server (来自 atsf4g-co 项目)
// echoServerPath: atsf4g-co 编译产物中的 echosvr 可执行文件路径
func StartAtAppEchoServer(echoServerPath string, busId uint64, listenAddr string) (*exec.Cmd, error)

// WaitForServerReady 等待服务器就绪 (通过尝试连接检测)
func WaitForServerReady(addr string, timeout time.Duration) error

// StopServer 停止服务器进程
func StopServer(cmd *exec.Cmd) error
```

#### 5.6.4 互通测试环境要求
- 需要预先编译 atsf4g-co 项目的 echosvr
- 通过环境变量 `ATBUS_CROSSLANG_ECHOSVR_PATH` 指定 echosvr 路径
- 如未设置环境变量，跨语言测试将被跳过 (skip)

---

## 六、关键实现细节

### 6.1 Go 与 C++ 的关键差异处理

#### 6.1.1 事件循环
```go
// C++: uv_run(loop, UV_RUN_ONCE)
// Go: 使用 goroutine + channel

// 每个监听器一个 goroutine
go func() {
    for {
        conn, err := listener.Accept()
        if err != nil {
            return
        }
        go handleConnection(conn)
    }
}()

// 每个连接一个读 goroutine
go func() {
    for {
        n, err := conn.Read(buf)
        if err != nil {
            onDisconnect(conn, err)
            return
        }
        onReceive(conn, buf[:n])
    }
}()
```

#### 6.1.2 Poll 实现
```go
func (n *Node) Poll() ErrorType {
    // Go 中不需要显式 poll，网络 IO 由 runtime 调度
    // Poll 主要用于处理 self 消息/GC/定时器收尾
    return EN_ATBUS_ERR_SUCCESS
}
```

#### 6.1.3 并发安全
```go
// 关键数据结构需要加锁
type Node struct {
    mu sync.RWMutex
    // ...
}

// 或使用 sync/atomic 进行无锁操作
func (n *Node) AllocateMessageSequence() uint64 {
    return n.messageSequenceAllocator.Add(1)
}
```

### 6.2 消息帧格式 (与 C++ 保持一致 - 跨语言互通关键)

> ⚠️ **此格式必须与 C++ 版本完全一致，否则无法互通！**

```go
const (
    FRAME_HASH_SIZE = 4  // murmur3 hash (32-bit, little-endian)
)

// 帧格式: [hash:4字节][length:varint][payload:N字节]
// 
// C++ 参考: channel_io_stream.cpp 中的 io_stream_send() 和接收逻辑
// hash 算法: murmur3_x86_32( seed=0 ), **仅对 payload 计算**
// Hash 字节序: 与 C++ memcpy(uint32_t) 一致，按 little-endian 写入/读取
// length 编码: libatbus 自定义 vint (使用 buffer.ReadVint/WriteVint)

func PackFrame(data []byte) []byte {
    // 1. 计算 payload 长度的 varint 编码 (使用 buffer.WriteVint)
    lenBuf := make([]byte, buffer.VintEncodedSize(uint64(len(data))))
    lenN := buffer.WriteVint(uint64(len(data)), lenBuf)
    
    // 2. 组装: hash + length + data
    frame := make([]byte, FRAME_HASH_SIZE+lenN+len(data))
    copy(frame[FRAME_HASH_SIZE:], lenBuf[:lenN])
    copy(frame[FRAME_HASH_SIZE+lenN:], data)
    
    // 3. 计算并填充 hash (对 payload 计算)
    hash := murmur3.Sum32(data)
    binary.LittleEndian.PutUint32(frame[:FRAME_HASH_SIZE], hash)
    
    return frame
}

func UnpackFrame(reader io.Reader) ([]byte, error) {
    // 1. 读取 hash
    hashBuf := make([]byte, FRAME_HASH_SIZE)
    if _, err := io.ReadFull(reader, hashBuf); err != nil {
        return nil, err
    }
    
    // 2. 读取 length (varint，使用 buffer.ReadVint)
    // 注意: 需要先读取足够的字节到缓冲区，再调用 buffer.ReadVint
    length, bytesRead := buffer.ReadVint(headerBuf)
    if bytesRead == 0 {
        return nil, ErrInvalidFrame
    }
    
    // 3. 读取 payload
    data := make([]byte, length)
    if _, err := io.ReadFull(reader, data); err != nil {
        return nil, err
    }
    
    // 4. 验证 hash (对 payload 计算)
    // ...
    
    return data, nil
}
```

### 6.3 Protobuf 消息兼容性 (跨语言互通关键)

> ⚠️ **必须使用与 C++ 相同的 .proto 文件生成 Go 代码！**

```
C++ proto 文件位置: atframework/libatbus/include/libatbus_protocol.proto
Go proto 文件位置: libatbus-go/protocol/

确保以下一致性:
1. 字段编号完全一致
2. 字段类型完全一致  
3. 枚举值完全一致
4. 默认值处理一致
```

#### 关键消息结构
| 消息类型 | 用途 | 跨语言注意点 |
|----------|------|--------------|
| `msg` | 顶层消息容器 | head + body oneof |
| `msg_head` | 消息头 | version, type, sequence, source_bus_id |
| `forward_data` | 数据转发 | from, to, router[], content, flags |
| `node_register_req/rsp` | 节点注册 | bus_id, pid, hostname, access_key |
| `ping_data` | 心跳 | time_point |
| `access_data` | 认证数据 | algorithm, nonce, signature[] |

### 6.4 加密握手兼容性 (跨语言互通关键)

> ⚠️ **加密密钥交换必须与 C++ 使用完全相同的算法和参数！**

支持的密钥交换算法 (必须与 C++ 一致):
- `ATBUS_CRYPTO_KEY_EXCHANGE_X25519` - ECDH with X25519
- `ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1` - ECDH with P-256
- `ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1` - ECDH with P-384
- `ATBUS_CRYPTO_KEY_EXCHANGE_SECP521R1` - ECDH with P-521

支持的对称加密算法:
- `ATBUS_CRYPTO_ALGORITHM_XXTEA`
- `ATBUS_CRYPTO_ALGORITHM_AES_*_CBC/GCM`
- `ATBUS_CRYPTO_ALGORITHM_CHACHA20*`

```go
// 握手流程必须与 C++ 完全一致:
// 1. 客户端发送 node_register_req (包含公钥)
// 2. 服务端回复 node_register_rsp (包含公钥 + 选定算法)
// 3. 双方使用 ECDH 计算共享密钥
// 4. 后续消息使用协商的对称加密算法
```

### 6.5 Access Token 验证 (跨语言互通关键)

> ⚠️ **签名算法必须与 C++ 完全一致！**

```go
// C++ 参考: atbus_node.cpp 中的 check_access_hash()
// 算法: HMAC-SHA256(token, plaintext)

func CalculateAccessDataSignature(
    accessKey *protocol.AccessData,
    token []byte,
    plainText string,
) []byte {
    // 必须与 C++ 的 message_handle::generate_access_data_for_* 完全一致
    // 1. plainText 来自 MakeAccessDataPlaintextFromHandshake/CustomCommand
    // 2. 使用 HMAC-SHA256(token, plainText)
    // 3. 返回签名 (注意 token 可能截断到 32868 bytes)
}
```

---

## 七、风险与依赖

### 7.1 依赖项
- `github.com/atframework/atframe-utils-go` - 工具库
- `github.com/atframework/libatbus-go/protocol` - Protobuf 协议 **(必须与 C++ 同步)**
- `github.com/atframework/libatbus-go/buffer` - 缓冲区算法 **(varint 已实现)**
- `github.com/spaolacci/murmur3` (或类似) - Hash 计算 **(必须与 C++ 算法一致)**
- `golang.org/x/crypto` - 加密算法支持

### 7.1.1 跨语言测试依赖
- atsf4g-co 项目编译产物 (echosvr) - **不需要在 Go 项目中编译 C++**
- 通过环境变量配置 echosvr 路径

### 7.2 潜在风险
1. **⚠️ 协议不兼容** - 最大风险，必须仔细对照 C++ 实现
2. **⚠️ 字节序问题** - 确保使用 Little-Endian (与 C++ 一致)
3. **⚠️ Varint 编码差异** - 必须使用 libatbus 自定义 vint (`buffer.ReadVint/WriteVint`)
4. **并发问题** - Go 的并发模型与 C++ 单线程回调不同，需要仔细处理
5. **性能差异** - goroutine 调度可能影响低延迟场景

### 7.3 跨语言互通风险缓解措施
| 风险 | 缓解措施 |
|------|----------|
| 消息帧格式不一致 | 编写二进制对比测试，与 C++ 输出逐字节比较 |
| Protobuf 序列化差异 | 使用相同 .proto 文件，验证序列化结果一致 |
| 加密算法实现差异 | 使用标准库，编写向量测试与 C++ 对照 |
| 握手流程差异 | 抓包分析 C++ 握手流程，严格对照实现 |

### 7.3 测试环境
- 需要支持 TCP、Unix socket 测试
- Windows 下可能需要跳过部分 Unix socket 测试

---

## 八、时间估算

| 阶段 | 预估时间 | 输出 |
|------|----------|------|
| Phase 1: IO Stream 通道 | 3-5 天 | channel/io_stream/*.go |
| Phase 2: Connection 完善 | 2-3 天 | impl/atbus_connection.go 完善 |
| Phase 3: Node 核心功能 | 3-4 天 | impl/atbus_node.go 完善 |
| Phase 4: Endpoint 完善 | 1-2 天 | impl/atbus_endpoint.go 完善 |
| Phase 5: 单元测试 | 4-5 天 | impl/*_test.go |
| **Phase 6: 跨语言互通测试** | **2-3 天** | **integration/*_test.go** |
| **总计** | **15-22 天** | 完整功能 + 测试 + 互通验证 |

---

## 九、验收标准

### 9.1 ⭐ 跨语言互通验收 (最高优先级)

> **如果无法与 C++ 版本互通，则整个实现视为失败！**

| 测试场景 | 描述 | 验收条件 |
|----------|------|----------|
| Go→C++ 注册 | Go 节点注册到 C++ 上游 | 注册成功，拓扑关系正确 |
| C++→Go 注册 | C++ 节点注册到 Go 上游 | 注册成功，拓扑关系正确 |
| Go→C++ 消息 | Go 发送数据到 C++ 节点 | C++ 正确接收并解析 |
| C++→Go 消息 | C++ 发送数据到 Go 节点 | Go 正确接收并解析 |
| 混合路由转发 | 消息经过 Go 和 C++ 节点转发 | 路由正确，数据完整 |
| 加密通道互通 | Go/C++ 使用加密通道通信 | 握手成功，数据正确加解密 |
| 自定义命令互通 | Go/C++ 互发自定义命令 | 命令正确解析和响应 |
| Ping/Pong 互通 | Go/C++ 互相 ping | 延迟统计正确 |

#### 互通测试方法
```bash
# 1. 启动 C++ atproxy 作为上游
./atproxy --bus-id=0x10000001 --listen=ipv4://127.0.0.1:9100

# 2. 启动 Go 节点连接到 C++ 上游
./go-node --bus-id=0x10010001 --upstream=ipv4://127.0.0.1:9100

# 3. 启动 C++ 节点作为兄弟节点
./cpp-node --bus-id=0x10010002 --upstream=ipv4://127.0.0.1:9100

# 4. 验证 Go 和 C++ 节点之间可以互相发送消息
```

### 9.2 功能完整性
   - [ ] 所有 Node API 实现且行为与 C++ 一致
   - [ ] 所有 Connection 方法实现
   - [ ] 所有 Endpoint 方法实现
   - [ ] IO Stream 通道支持 TCP/Unix socket

### 9.3 协议兼容性
   - [ ] 消息帧格式与 C++ 完全一致 (hash + varint + payload)
   - [ ] Protobuf 消息与 C++ 二进制兼容
   - [ ] 加密握手流程与 C++ 一致
   - [ ] Access Token 签名算法与 C++ 一致
   - [ ] **能够与 C++ 版本完全互通**

### 9.4 测试覆盖
   - [ ] 覆盖所有 C++ 测试用例
   - [ ] **新增跨语言互通测试**
   - [ ] 测试覆盖率 > 80%

### 9.5 代码质量
   - [ ] 通过 golangci-lint 检查
   - [ ] 有完整的 godoc 注释

---

## 附录 A: C++ 测试用例完整列表

### atbus_node_setup_test.cpp
1. `override_listen_path` (仅 Unix)
2. `crypto_algorithms`
3. `compression_algorithms`

### atbus_node_reg_test.cpp
1. `reset_and_send_tcp`
2. `reg_and_send_unix` (仅 Unix)
3. `reg_with_different_access_token`
4. `reconnect_same_addr`
5. `reg_timeout_connection`
6. `reg_cancel_before_connected`
7. `reconnect_fail_when_reset`
8. `auto_dealloc`
9. `hash_code`
10. `message_handler_callback`
11. `reg_with_crypto` (多种加密算法)
12. `reg_with_compression` (多种压缩算法)
13. ... (更多)

### atbus_node_msg_test.cpp
1. `send_to_self_basic`
2. `send_to_self_loop`
3. `send_to_sibling_basic`
4. `send_data_forward`
5. `send_with_router`
6. `custom_command_basic`
7. `custom_command_with_rsp`
8. `ping_pong_basic`
9. `ping_pong_timeout`
10. ... (更多)

### atbus_node_relationship_test.cpp
1. `copy_conf`
2. `child_endpoint_opr`

### atbus_endpoint_test.cpp
1. `connection_basic`
2. `endpoint_basic`
3. `is_child`
4. `get_connection`
5. `address`

---

## 附录 B: 文件修改清单

### 新建文件
- `channel/io_stream/channel_io_stream.go`
- `channel/io_stream/io_stream_conf.go`
- `channel/io_stream/io_stream_connection.go`
- `channel/io_stream/frame_codec.go` - **帧编解码 (跨语言关键)**
- `impl/atbus_node_test.go`
- `impl/atbus_node_reg_test.go`
- `impl/atbus_node_msg_test.go`
- `impl/atbus_node_relationship_test.go`
- `impl/atbus_endpoint_test.go`
- `impl/test_utils.go`
- `integration/crosslang_reg_test.go` - **跨语言注册测试 (使用 atapp echosvr)**
- `integration/crosslang_msg_test.go` - **跨语言消息测试 (使用 atapp echosvr)**
- `integration/crosslang_crypto_test.go` - **跨语言加密测试 (使用 atapp echosvr)**
- `integration/crosslang_test_utils.go` - **跨语言测试工具 (启动/管理 echosvr)**

### 修改文件
- `impl/atbus_node.go` - 完善核心方法
- `impl/atbus_connection.go` - 实现 IO 方法
- `impl/atbus_endpoint.go` - 完善 CreateEndpoint
- `types/atbus_node.go` - 可能需要补充接口

---

*文档版本: v1.1*  
*创建日期: 2026-02-04*  
*更新日期: 2026-02-04*  
*作者: GitHub Copilot*  
*关键约束: **必须支持与 C++ 版本的跨语言互通***
