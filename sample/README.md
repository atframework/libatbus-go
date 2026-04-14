# libatbus-go Samples

## sample_usage_01

Go equivalent of the C++ `libatbus/sample/sample_usage_01.cpp`. Demonstrates basic two-node communication using libatbus-go.

### Build & Run (two-node mode)

```bash
cd sample/sample_usage_01
go run .
```

Output:
```
node1 (0x12345678) listening on ipv4://127.0.0.1:<port>
node2 (0x12356789) listening on ipv4://127.0.0.1:<port>
Waiting for connection...
Connection established!
node1 sent: hello world!
atbus node 0x12356789 receive data from 0x12345678(...): hello world!
Sample completed successfully!
```

### Flags

| Flag       | Default      | Description                              |
|------------|-------------|------------------------------------------|
| `--addr1`  | random      | Listen address for node1                 |
| `--addr2`  | random      | Listen address for node2                 |
| `--id1`    | 0x12345678  | Bus ID for node1                         |
| `--id2`    | 0x12356789  | Bus ID for node2                         |
| `--local`  | false       | Single-node mode (for cross-language)    |
| `--remote` | (required)  | Remote address (`--local` mode only)     |

### Single-node mode (cross-language testing)

```bash
# Start a Go node that connects to a remote C++ node:
go run . --local --id1 0x12356789 --addr1 ipv4://127.0.0.1:16388 --remote ipv4://127.0.0.1:16387
```

### Tests

Tests verify bidirectional communication across all node relationship types:

```bash
# Go-only tests (no external dependencies)
go test -v ./...

# With C++ echo server for cross-language tests:
# 1. Start C++ echo server (from atsf4g-co build):
#    atapp_sample_echo_svr.exe -id 1 -c sample_echo_svr.yaml start
# 2. Run all tests including cross-language:
go test -v -args -cpp-echo-addr=ipv4://127.0.0.1:21437 -cpp-echo-id=1
```

| Test | Relationship | Description |
|------|-------------|-------------|
| `TestOtherUpstreamPeer` | OtherUpstreamPeer | No explicit topology (default) |
| `TestUpstreamDownstream` | ImmediateUpstream / ImmediateDownstream | Parent-child |
| `TestSameUpstreamPeer` | SameUpstreamPeer | Siblings under same parent |
| `TestTransitiveUpstreamDownstream` | TransitiveUpstream / TransitiveDownstream | Grandparent-child hierarchy |
| `TestCrossLang_GoToCppEcho` | Go → C++ echo → Go | Cross-language echo roundtrip |
| `TestCrossLang_GoMultipleMessages` | Go → C++ echo → Go | Multiple messages echo verification |
