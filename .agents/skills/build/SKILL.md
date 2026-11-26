---
name: build
description: "Build libatbus-go, generate protobuf code, and manage dependencies. Use when: building the module, running code generation, resolving import errors, updating proto definitions, checking prerequisites."
---

# Build & Code Generation — libatbus-go

## Prerequisites

- Go 1.27.4+ (pinned in `mise.toml`)
- `protoc` (auto-downloaded into `.cache/atbuild/tools` if absent)
- `atbuild`, `protoc-gen-go`, `protoc-gen-mutable` (pinned in `tools.mod` / `tools.sum`)

## Quick Commands

```bash
# Generate protobuf code (recommended; GOWORK=off is required when a go.work is active)
GOWORK=off go tool -modfile tools.mod atbuild proto
# Windows PowerShell:
# $env:GOWORK='off'; go tool -modfile tools.mod atbuild proto

# Or via go generate (works in both standalone and go.work workspace modes)
go generate ./...

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

The `generate.go` (`!windows`) and `generate_windows.go` (`windows`) files at module root invoke `atbuild proto` with `GOWORK=off` so `go generate ./...` works in both standalone and workspace (`go.work`) modes:

```go
// generate.go (!windows)
//go:generate env GOWORK=off go tool -modfile tools.mod atbuild proto

// generate_windows.go (windows)
//go:generate cmd /c "set GOWORK=off&& go tool -modfile tools.mod atbuild proto"
```

So you can run `go generate ./...` directly even when a `go.work` file is present.

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
| `-modfile cannot be used in workspace mode` | Prefix `go tool -modfile` commands with `GOWORK=off` (or run `go generate ./...`) |
| Stale generated code            | Run `GOWORK=off go tool -modfile tools.mod atbuild proto --force` and verify `protocol/*.pb.go` timestamps |
