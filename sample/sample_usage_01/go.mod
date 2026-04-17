module github.com/atframework/libatbus-go/sample/sample_usage_01

go 1.25.1

require github.com/atframework/libatbus-go v0.0.0

require (
	github.com/atframework/atframe-utils-go v1.0.5-0.20260416024202-66c04636f055 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/compress v1.17.10 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/atframework/libatbus-go => ../..
