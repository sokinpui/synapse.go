#!/bin/bash

mkdir -p grpc
protoc --proto_path=protos \
  --go_out=grpc --go_opt=paths=source_relative \
  --go-grpc_out=grpc --go-grpc_opt=paths=source_relative \
  protos/generate.proto
