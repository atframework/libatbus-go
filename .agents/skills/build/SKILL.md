---
name: build
description: "Build libatbus-go, generate protobuf code, and manage dependencies. Use when: building the module, running code generation, resolving import errors, updating proto definitions, checking prerequisites."
---

# Build & Code Generation — libatbus-go

## Prerequisites

- Go 1.25+
- `protoc` (protobuf compiler)
- `protoc-gen-go` (`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`)
- `protoc-gen-mutable` (custom plugin, generates `*_mutable.pb.go` with Mutable/Clone/Merge extensions)
- [go-task](https://taskfile.dev/) (recommended)

## Quick Commands

```bash
# Generate protobuf code (recommended)
task generate-protocol

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
//go:generate task generate-protocol
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
| Stale generated code            | Run `task generate-protocol` and verify `protocol/*.pb.go` timestamps                                 |
