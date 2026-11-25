---
name: build
description: "Build libatbus-go, generate protobuf code, and manage dependencies. Use when: building the module, running code generation, resolving import errors, updating proto definitions, checking prerequisites."
---

# Build & Code Generation — libatbus-go

## Prerequisites

- Go 1.27.4+ (pinned in `mise.toml`)
- `protoc` (auto-downloaded into `.cache/atbuild/tools` if absent)
- `atbuild`, `protoc-gen-go`, `protoc-gen-mutable` (pinned in `tools.mod`)

## Quick Commands

```bash
# Generate protobuf code (recommended)
go tool -modfile tools.mod atbuild proto

# Or run protoc directly
cd protocol && protoc --go_out=. --mutable_out=. \
  --go_opt=paths=source_relative --mutable_opt=paths=source_relative \
  --proto_path=./ ./*.proto

# Build all packages
go build ./...

# Run all tests
go test ./...
```

## Proto Code Generation

The protocol definition lives in `protocol/libatbus_protocol.proto`. Generated files:

| File                              | Generator            | Content                                 |
| --------------------------------- | -------------------- | --------------------------------------- |
| `libatbus_protocol.pb.go`         | `protoc-gen-go`      | Standard protobuf types                 |
| `libatbus_protocol_mutable.pb.go` | `protoc-gen-mutable` | Mutable/Clone/Merge/Readonly extensions |

The `generate.go` file at module root contains:

```go
//go:generate go tool -modfile tools.mod atbuild proto
```

So you can also run: `go generate ./...`

## Module Path

```
github.com/atframework/libatbus-go
```

In the parent repo (`atsf4g-go`), this is referenced via `go.mod` replace:

```
replace github.com/atframework/libatbus-go => ./atframework/libatbus-go
```

## Dependencies

| Dependency                                | Purpose                                     |
| ----------------------------------------- | ------------------------------------------- |
| `github.com/atframework/atframe-utils-go` | Utility library (LRU map, algorithms)       |
| `github.com/spaolacci/murmur3`            | Murmur3 hash for buffer management          |
| `github.com/stretchr/testify`             | Test assertions                             |
| `golang.org/x/crypto`                     | ChaCha20, XXTEA, extended crypto algorithms |
| `google.golang.org/protobuf`              | Protobuf runtime                            |

## Troubleshooting

| Symptom                         | Fix                                                                                                   |
| ------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `protoc-gen-mutable: not found` | Install the custom plugin from the parent repo's tools                                                |
| Import cycle errors             | Check that `types/` does not import `impl/` (types defines interfaces, impl provides implementations) |
| Stale generated code            | Run `go tool -modfile tools.mod atbuild proto --force` and verify `protocol/*.pb.go` timestamps       |
