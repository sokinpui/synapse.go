#!/bin/bash

echo "Generating gRPC code..."
mkdir -p grpc
protoc --proto_path=protos \
  --go_out=grpc --go_opt=paths=source_relative \
  --go-grpc_out=grpc --go-grpc_opt=paths=source_relative \
  protos/generate.proto

echo "Tidying Go modules..."
go mod tidy

echo "Building binaries..."
go build -o bin/server ./cmd/server
go build -o bin/worker ./cmd/worker
echo "Build complete. Binaries are in the 'bin' directory."
