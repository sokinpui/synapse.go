# Build Stage
FROM golang:1.24-bookworm AS builder

WORKDIR /src

RUN apt-get update && apt-get install -y unzip protobuf-compiler

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

COPY go.mod ./
RUN go mod download

COPY . .

RUN mkdir -p grpc && \
    protoc --proto_path=protos \
    --go_out=grpc --go_opt=paths=source_relative \
    --go-grpc_out=grpc --go-grpc_opt=paths=source_relative \
    protos/generate.proto

RUN CGO_ENABLED=0 go build -o /bin/synapse-server ./cmd/server

# Runtime Stage
FROM ubuntu:24.04

WORKDIR /app

RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/synapse-server .
COPY --from=builder /src/config.yaml .

EXPOSE 50051
EXPOSE 8080

ENTRYPOINT ["./synapse-server"]
