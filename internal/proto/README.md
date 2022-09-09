# internal/proto package documentation for developers

The `*.pb.go` files in this directory are automatically generated with the
`make generate-protobuf` command.

New `*.proto` files will need to be added to the `generate.go` file, with a
`+generate` and `go:generate` build-tag.

It is expected that the protobuf compiler (`protoc`) is available on the
system. Plugins for the compiler to generate Go code (`protoc-gen-go`) and
Go-GRPC interfaces (`protoc-gen-go-grpc`) will be automatically installed
through the `tools.go` in the root the project.
