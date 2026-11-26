module github.com/atframework/libatbus-go/tools

go 1.27.4

tool (
	github.com/atframework/atframe-utils-go/buildkit/cmd/atbuild
	github.com/atframework/atframe-utils-go/protoc-gen-mutable
	google.golang.org/protobuf/cmd/protoc-gen-go
)

require (
	github.com/atframework/atframe-utils-go v1.0.5-0.20260416024202-66c04636f055 // indirect
	github.com/atframework/atframe-utils-go/buildkit v0.0.0-20261125073917-01de2a20e478 // indirect
	golang.org/x/sync v0.20.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
