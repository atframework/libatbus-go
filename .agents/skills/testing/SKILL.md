---
name: testing
description: "Run and write unit tests for libatbus-go. Use when: running tests, writing new test cases, debugging test failures, working with cross-language test vectors, testing concurrency safety, verifying C++ parity."
---

# Testing — libatbus-go

## Running Tests

```bash
# All tests
cd atframework/libatbus-go
go test ./...

# Verbose output
go test -v ./...

# Specific package
go test -v ./impl/
go test -v ./buffer/
go test -v ./channel/io_stream/
go test -v ./message_handle/

# Specific test by name
go test -v -run "TestNodeRegParity_BasicSingleNode" ./impl/
go test -v -run "TestConnectionContext" ./impl/

# Race detector
go test -race ./...
```

## Test Organization

| File                                           | Domain                                  | Key Tests             |
| ---------------------------------------------- | --------------------------------------- | --------------------- |
| `impl/atbus_node_reg_test.go`                  | Registration, topology, timeouts        | 19 tests              |
| `impl/atbus_node_msg_test.go`                  | Message send/receive, loopback          | 15 tests              |
| `impl/atbus_node_msg_extended_test.go`         | Multi-hop routing, encryption, failures | 11 tests              |
| `impl/atbus_node_regression_test.go`           | P0 bug regressions                      | 17 tests              |
| `impl/atbus_node_setup_test.go`                | Node setup, algorithm parsing           | Setup tests           |
| `impl/atbus_node_relationship_test.go`         | Endpoint lifecycle                      | 3 tests               |
| `impl/atbus_node_blacklist_test.go`            | Blacklist behavior                      | Blacklist tests       |
| `impl/atbus_node_fault_tolerant_test.go`       | Fault tolerance                         | Fault tolerance tests |
| `impl/atbus_connection_context_test.go`        | ECDH, cipher, compression               | ~37 tests             |
| `impl/atbus_connection_test.go`                | Connection lifecycle                    | 7 tests               |
| `impl/atbus_endpoint_test.go`                  | Endpoint behavior                       | 3 tests               |
| `impl/atbus_topology_test.go`                  | Peer CRUD, relations                    | 9 tests               |
| `message_handle/atbus_message_handler_test.go` | Signatures, dispatch                    | ~16 tests             |
| `buffer/*_test.go`                             | Varint, buffer managers                 | ~11 tests             |
| `channel/io_stream/*_test.go`                  | Frame codec, TCP/Unix/pipe              | ~13 tests             |
| Root `*_test.go`                               | Public API parity                       | Interface tests       |

## Test Naming Convention

```
Test[Component][Parity_][Scenario]
```

- `Parity_` prefix indicates C++ ↔ Go parity verification
- Examples: `TestNodeRegParity_BasicSingleNode`, `TestConnectionContextHandshake`, `TestBufferBlockBasic`

## Writing Tests

### Test Structure (AAA Pattern)

```go
func TestNodeRegParity_Scenario(t *testing.T) {
    // Arrange
    conf := createTestNodeConfig(...)
    node, err := impl.NewNode(conf)
    assert.NoError(t, err)

    // Act
    result := node.SomeOperation()

    // Assert
    assert.Equal(t, expected, result)
}
```

### Time-Driven Testing

Tests use `Proc(now)` with explicit time values instead of wall-clock waits:

```go
now := time.Now()
node.Proc(now)
// Advance time for timeout testing
now = now.Add(30 * time.Second)
node.Proc(now)
```

### Cross-Language Test Vectors

Binary test vectors in `impl/testdata/` and `message_handle/testdata/` ensure Go ↔ C++ interoperability:

- `*.bytes` — Binary wire data
- `*.json` — Expected decoded values

The `tools/testdata_sync_custom_cmd/` tool syncs test vectors from the C++ implementation.

### Concurrency Testing

When testing concurrent access patterns:

1. Use `-race` flag: `go test -race ./...`
2. `Endpoint` fields (`ctrlConn`, `dataConn`, `flags`) require `sync.Mutex` protection
3. `nodeEventTimer.pingList` (LRU map) needs mutex — accessed from both main goroutine and I/O goroutine disconnect callbacks
4. Use snapshot-and-release pattern: lock briefly, copy state, unlock, then operate on the copy

## Parent Repo Test Conventions

Follow `/.github/instructions/gotest.instructions.md` for general Go test conventions (AAA pattern, mocks, concurrency, benchmarks).
