---
name: architecture
description: "libatbus-go module structure, concurrency model, C++ parity patterns, and design decisions. Use when: understanding module organization, debugging concurrency issues, making design decisions, reviewing C++/Go differences, adding new features."
---

# Architecture — libatbus-go

## Module Layout

```
libatbus-go/
├── Root package (github.com/atframework/libatbus-go)
│   └── Public API facades: Node, Endpoint, Connection creation helpers
├── types/          — Public interfaces & type definitions (no impl dependency)
├── impl/           — Core implementation (Node, Endpoint, Connection, Topology)
├── protocol/       — Protobuf v3 wire format (generated code)
├── message_handle/ — Message dispatch & access token authentication
├── channel/        — Transport layer (io_stream for TCP/Unix/pipe, utility for address parsing)
├── buffer/         — Buffer management (varint encoding, ring buffer, static blocks)
└── error_code/     — Error code enum and error-to-string mapping
```

### Dependency Direction

```
Root package → types, impl, protocol, error_code
impl → types, protocol, message_handle, channel, buffer, error_code
message_handle → types, protocol
channel → types, buffer
types → protocol (for proto enums only)
```

**Rule**: `types/` MUST NOT import `impl/`. Types define interfaces; impl provides implementations.

## Concurrency Model

### Go vs C++ Design Differences

| Aspect     | C++ libatbus                         | Go libatbus-go                            |
| ---------- | ------------------------------------ | ----------------------------------------- |
| Event loop | libuv (single-threaded)              | Goroutines (multi-goroutine)              |
| I/O        | libuv async callbacks                | Per-connection goroutine read/write loops |
| Timers     | libuv timer handles                  | `Proc(now)` time-driven (no real timers)  |
| Memory     | Manual (`ref_object`/`unref_object`) | GC-managed                                |
| Channels   | mem, shm, unix, pipe, TCP            | unix, pipe, TCP only                      |

### Goroutine Architecture

```
Main goroutine (caller of Proc)
├── Node.Proc(now) — event loop tick
│   ├── processPingTimers()
│   ├── processConnectingTimeout()
│   ├── executeGC()
│   └── topology updates
│
Per-connection goroutines (channel/io_stream)
├── readLoop()  — reads frames, dispatches to message_handle
└── writeLoop() — writes queued frames, calls disconnect callbacks on error
```

### Critical Synchronization Points

1. **Endpoint mutex** (`sync.Mutex`):
   - Protects `ctrlConn`, `dataConn`, `flags`
   - Accessed from I/O goroutine disconnect callbacks AND main loop GC
   - Pattern: snapshot fields under lock, operate on snapshot outside lock

2. **pingList mutex** (`nodeEventTimer.pingListMu`):
   - Protects the LRU ping timer map
   - `addPingTimer`/`removePingTimer` called from I/O goroutine disconnect path
   - `processPingTimers` and `Node.Reset()` run on main goroutine

3. **Snapshot-and-release pattern** (deadlock prevention):

   ```go
   ep.mu.Lock()
   conns := make([]*Connection, len(ep.dataConn))
   copy(conns, ep.dataConn)
   ep.dataConn = nil
   ep.mu.Unlock()
   // Now operate on conns without holding the lock
   for _, c := range conns {
       c.Reset()
   }
   ```

## C++ Parity Patterns

### Translation Guidelines

| C++ Pattern                  | Go Equivalent                                    |
| ---------------------------- | ------------------------------------------------ |
| `shared_ptr<endpoint>`       | `*Endpoint` (GC-managed)                         |
| `weak_ptr` / `ref_object`    | Not needed (GC)                                  |
| `uv_loop_t` event loop       | `Proc(now)` polling + goroutines                 |
| `std::unordered_map`         | `map` or `LRUMap` (from atframe-utils-go)        |
| `std::list` (ordered timers) | `LRUMap` with time-ordered eviction              |
| Callback function pointers   | Go function values in `types.NodeEventHandleSet` |
| `enum class`                 | Go `const` + `iota` in `protocol` package        |

### GC Behavior (executeGC)

The `executeGC()` function must match C++ behavior:

1. Swap `pendingEndpointGcList` to a local slice (avoid modification during iteration)
2. For each endpoint: check `IsAvailable()`, call `RemoveEndpoint()` if not
3. `RemoveEndpoint()` handles state transitions (Running → LostUpstream for upstream endpoints)
4. Do NOT call `ep.Reset()` redundantly — it races with I/O goroutine disconnect callbacks

### Connecting Timeout

`processConnectingTimeout()` must maintain single-callback semantics:

- After `pair.Value.Reset()`, check if the front entry changed
- Only call `OnInvalidConnection` if the entry was not already cleaned up by `Reset()`
- This matches the C++ `connecting_list.begin() == iter` guard

## Node Topology & Routing

Tree/forest topology:

- Each node has at most one **upstream** (parent) node
- Each node can have multiple **downstream** (child) nodes
- Nodes sharing the same parent are **peers**

Routing logic:

1. Look up target in local endpoint map → direct connection
2. If not found, forward to upstream (recursively routes)
3. TTL prevents infinite forwarding loops

Relation types: `kSelf`, `kImmediateUpstream`, `kTransitiveUpstream`, `kImmediateDownstream`, `kTransitiveDownstream`, `kSameUpstreamPeer`, `kOtherUpstreamPeer`.

## Default Configuration Values

All defaults align with C++ `libatbus`:

| Config                   | Value   |
| ------------------------ | ------- |
| LoopTimes                | 256     |
| TTL                      | 16      |
| FirstIdleTimeout         | 30s     |
| PingInterval             | 8s      |
| RetryInterval            | 3s      |
| FaultTolerant            | 2       |
| BackLog                  | 256     |
| AccessTokenMaxNumber     | 5       |
| CryptoKeyRefreshInterval | 3h      |
| MessageSize              | 2 MiB   |
| RecvBufferSize           | 256 MiB |
| SendBufferSize           | 8 MiB   |
