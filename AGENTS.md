# libatbus-go

**libatbus-go** is the Go implementation of the [libatbus](https://github.com/atframework/libatbus) message bus abstraction layer.
It provides cross-language interoperability with the C++ `libatbus` for building tree-structured, encrypted, compressed service-to-service communication.

- **Module**: `github.com/atframework/libatbus-go`
- **License**: MIT
- **Language**: Go 1.25+
- **C++ reference**: `atsf4g-co/atframework/libatbus`

## Directory Structure

```
libatbus-go/
├── atbus_connection.go          # Root-level connection public API
├── atbus_endpoint.go            # Root-level endpoint public API
├── atbus_node.go                # Root-level node public API
├── libatbus_error.go            # Root-level error code mapping
├── libatbus_protocol.go         # Root-level protocol helpers
├── generate.go                  # go:generate marker (task generate-protocol)
├── buffer/                      # Buffer management (varint, ring buffer, static block)
│   ├── buffer_algorithm.go
│   ├── buffer_block.go
│   ├── buffer_manager.go
│   └── static_buffer_block.go
├── channel/                     # Transport channels
│   ├── io_stream/               # TCP / Unix socket / named pipe I/O
│   │   ├── channel_io_stream.go # Channel lifecycle, accept, connect, callbacks
│   │   ├── frame_codec.go       # Frame encoding/decoding (varint length prefix)
│   │   ├── types.go             # Channel types and constants
│   │   ├── listen_unix.go       # Unix-specific listener
│   │   ├── listen_windows.go    # Windows-specific listener
│   │   ├── pipe_unix.go         # Unix pipe support
│   │   └── pipe_windows.go      # Windows named pipe support
│   └── utility/                 # Address parsing, priority calculation
│       └── channel_utility.go
├── error_code/                  # Error code enum (ATBUS_ERROR_TYPE)
│   └── libatbus_error.go
├── impl/                        # Core implementation
│   ├── atbus_node.go            # Node: init, routing, event loop, crypto, GC
│   ├── atbus_endpoint.go        # Endpoint: remote node, ctrl+data connections
│   ├── atbus_connection.go      # Connection: state machine, I/O dispatch
│   ├── atbus_connection_context.go      # ECDH handshake, cipher, compression, pack/unpack
│   ├── atbus_connection_context_test.go # Cross-language encryption/compression vectors
│   ├── atbus_topology.go        # Topology: peer registry, relation queries
│   └── testdata/                # Binary test vectors (*.bytes + *.json)
├── message_handle/              # Message dispatch (register, ping/pong, forward, auth)
│   ├── atbus_message_handler.go
│   └── testdata/                # Signature test vectors
├── protocol/                    # Protobuf v3 wire format
│   ├── libatbus_protocol.proto  # Source of truth
│   ├── libatbus_protocol.pb.go  # Generated
│   └── libatbus_protocol_mutable.pb.go  # Generated (Mutable/Clone extensions)
├── types/                       # Public type definitions & interfaces
│   ├── atbus_node.go            # Node types, config, event callbacks
│   ├── atbus_endpoint.go        # Endpoint types, statistics
│   ├── atbus_connection.go      # Connection types, state enum
│   ├── atbus_connection_context.go  # Crypto context types
│   ├── atbus_message.go         # Message types
│   ├── atbus_topology.go        # Topology types, relation enum
│   ├── atbus_algorithm_parse.go # Algorithm name parsing helpers
│   ├── atbus_common_types.go    # Shared constants and types
│   └── channel_address.go       # Channel address types
├── tools/                       # Utilities
│   └── testdata_sync_custom_cmd/  # Cross-language test vector sync tool
├── Plan.md                      # Functional completion plan (Chinese)
└── UnitTestPlan.md              # Unit test execution plan (Chinese)
```

## Architecture

```
┌───────────────────────────────────────────────────────┐
│  Application (SendData / OnForwardRequest callback)   │
├───────────────────────────────────────────────────────┤
│  Node          — routing, event loop, crypto config   │
├───────────────────────────────────────────────────────┤
│  Topology      — peer registry, relation queries      │
├───────────────────────────────────────────────────────┤
│  Endpoint      — remote node, ctrl + data connections │
├───────────────────────────────────────────────────────┤
│  Message Handler — dispatch: register/ping/forward    │
├───────────────────────────────────────────────────────┤
│  Connection    — state machine, I/O read/write        │
├───────────────────────────────────────────────────────┤
│  ConnectionContext — ECDH handshake, cipher, compress  │
├───────────────────────────────────────────────────────┤
│  Channel       — transport (TCP, Unix, named pipe)    │
├───────────────────────────────────────────────────────┤
│  Protocol      — Protobuf v3 wire format              │
└───────────────────────────────────────────────────────┘
```

## C++ Parity & Design Boundaries

This module aims for feature parity with C++ `libatbus`, with the following **intentional exclusions**:

| Feature | Reason |
|---------|--------|
| `mem://` channel | In-process memory ring buffer (Go uses goroutines, not needed) |
| `shm://` channel | Cross-process shared memory (not applicable in Go deployment) |
| `ref_object()` / `unref_object()` | Go uses GC; no manual reference counting |
| `get_evloop()` | Go uses goroutines instead of libuv event loop |
| `get_crypto_key_exchange_context()` | Go `crypto/ecdh` is stateless |

### Supported Channel Types

| Channel | Scheme | Scope | Priority |
|---------|--------|-------|----------|
| Unix Socket | `unix://` | Same host | Medium-High (+0x16) |
| Named Pipe | `pipe://` | Same host (Windows) | Medium-High (+0x16) |
| TCP | `ipv4://`, `ipv6://`, `dns://` | Network | Base (+0x03) |

### Encryption & Compression

**Key Exchange**: X25519, SECP256R1, SECP384R1, SECP521R1

**Symmetric Ciphers**: XXTEA, AES-128/192/256-CBC, AES-128/192/256-GCM, ChaCha20, ChaCha20-Poly1305-IETF, XChaCha20-Poly1305-IETF

**KDF**: HKDF-SHA256

**Compression**: Zstd, LZ4, Snappy

**Access Token Auth**: HMAC-SHA256 signatures with timestamp tolerance (±300s)

### Connection Lifecycle

```
[Created] → kConnecting → kHandshaking → kConnected ⇄ (key refresh) → kDisconnecting → [Destroyed]
```

## Concurrency Model

Go libatbus-go uses goroutines instead of a libuv event loop:

- **Main goroutine**: Node event loop (`Proc()`), topology, GC, timer processing
- **I/O goroutines**: Per-connection read/write loops (channel_io_stream)
- **Synchronization**: `sync.Mutex` is required on shared state:
  - `Endpoint` — protects `ctrlConn`, `dataConn`, `flags` (accessed from I/O goroutine disconnect callbacks and main loop GC)
  - `nodeEventTimer.pingList` — protects the LRU ping timer map (accessed from I/O goroutine disconnect callbacks and main loop `processPingTimers`)
- **Design rule**: Snapshot-and-release pattern — lock briefly to copy/clear state, then operate on the snapshot outside the lock to avoid deadlocks

## Build & Code Generation

```bash
# Generate protobuf code
task generate-protocol

# Build
go build ./...

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run specific test
go test -v -run "TestNodeRegParity" ./impl/
```

### Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/atframework/atframe-utils-go` | Utility library (LRU map, algorithms) |
| `github.com/spaolacci/murmur3` | Murmur3 hash |
| `github.com/stretchr/testify` | Test assertions |
| `golang.org/x/crypto` | ChaCha20, XXTEA, extended crypto |
| `google.golang.org/protobuf` | Protobuf runtime |

## Unit Testing

### Test Framework

- Go native `testing` package + `github.com/stretchr/testify/assert`
- Time-driven testing via `Proc(now)` instead of wall-clock waits
- Cross-language binary test vectors for crypto/compression parity

### Test Organization

| Location | Count | Coverage |
|----------|-------|---------|
| `impl/atbus_node_*_test.go` | ~80+ tests | Node lifecycle, registration, messaging, routing, blacklist, fault tolerance, regression |
| `impl/atbus_connection_context_test.go` | ~37 tests | ECDH handshake, cipher, compression (cross-language vectors) |
| `impl/atbus_connection_test.go` | 7 tests | Connection lifecycle |
| `impl/atbus_endpoint_test.go` | 3 tests | Endpoint behavior |
| `impl/atbus_topology_test.go` | 9 tests | Peer CRUD, relation queries |
| `message_handle/*_test.go` | ~16 tests | Access data signatures, message dispatch |
| `buffer/*_test.go` | ~11 tests | Varint, buffer managers |
| `channel/io_stream/*_test.go` | ~13 tests | Frame codec, TCP/Unix/pipe I/O |
| `channel/utility/*_test.go` | Tests | Address parsing |
| `error_code/*_test.go` | 6 tests | Error code mapping |
| Root `*_test.go` | Tests | Public API parity |

### Test Naming Convention

```
Test[Component][Parity_][Scenario]
```

Examples: `TestNodeRegParity_BasicSingleNode`, `TestConnectionContextHandshake`, `TestBufferBlockBasic`.

### Running Tests

```bash
cd atframework/libatbus-go
go test ./...
```

## Coding Conventions

1. **Naming**: Follow Go standard — exported names `PascalCase`, unexported `camelCase`
2. **Error handling**: Return `error` values; use `error_code` package for bus-specific errors
3. **Testing**: AAA pattern (Arrange-Act-Assert); use `testify/assert`
4. **Proto generation**: `protoc-gen-go` + custom `protoc-gen-mutable` plugin
5. **Logging**: Use `github.com/atframework/atframe-utils-go/log`
6. **如非必要，勿增实体** (Do not add entities unless necessary)

## How-to Guides (Skills)

Detailed operational playbooks are in `.agents/skills/` (cross-client [Agent Skills](https://agentskills.io/) standard):

| Skill | Path | Description |
|-------|------|-------------|
| Build | `.agents/skills/build/SKILL.md` | Build, code generation, dependencies |
| Testing | `.agents/skills/testing/SKILL.md` | Run and write tests, cross-language vectors, concurrency testing |
| Protocol & Crypto | `.agents/skills/protocol-crypto/SKILL.md` | ECDH handshake, encryption/compression, message framing, access token auth |
| Architecture | `.agents/skills/architecture/SKILL.md` | Module structure, concurrency model, C++ parity patterns |
