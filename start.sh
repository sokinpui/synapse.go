#!/bin/bash

echo "Building Synapse Server..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

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
echo "Build complete. Binaries are in the 'bin' directory."

# for local deploy, use vpn proxy
export http_proxy=http://127.0.0.1:1087
export https_proxy=http://127.0.0.1:1087
export ALL_PROXY=socks5://127.0.0.1:1080

echo "Starting server..."
$PWD/bin/server &
SERVER_PID=$!

cleanup() {
  echo
  echo "Cleaning up server process (PID: $SERVER_PID)..."
  kill $SERVER_PID 2>/dev/null
  wait $SERVER_PID 2>/dev/null
  echo "Cleanup complete."
  exit 0
}

trap cleanup INT TERM

echo "Server is running. Press Ctrl+C to stop."

wait
