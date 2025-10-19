# Synapse

Simple distributed task queue system implement in gRPC and Redis

## Architecture

The system consists of two main components: a `server` and a `worker`.

```
Client ---gRPC---> Server ---LPUSH---> Redis Queue ---BRPOP---> Worker
                               ^                                  |
                               |                                  |
                               +----SUBSCRIBE---- Redis Pub/Sub <-+PUBLISH
```

## Prerequisites

- Go (1.21+)
- Protobuf Compiler (`protoc`)
- Docker and Docker Compose (for Redis)

## Getting Started

### 1. Configuration

The application is configured using environment variables. Create a `.env` file in the root directory or export the variables directly.

> **Note:** You can copy the provided `.env.example` to `.env` to get started quickly.

```dotenv
# Redis Configuration
SYNAPSE_REDIS_HOST=localhost
SYNAPSE_REDIS_PORT=6379
SYNAPSE_REDIS_DB=0
SYNAPSE_REDIS_PASSWORD=""

# gRPC Server Port
SYNAPSE_GRPC_PORT=50051

# API key for the underlying LLM provider (sllmi-go)
GENAI_API_KEY="YOUR_GEMINI_API_KEY"
```

### 2. Build with Makefile

Generate gRPC stubs and build server/worker binaries

```sh
make build
```

Binaries will be placed in the `./bin` directory.

### 3. Run

1.  **Start the Worker**:
    ```sh
    ./bin/worker
    ```
2.  **Start the Server**:
    ```sh
    ./bin/server
    ```

The server will listen for gRPC requests on the port specified by `SYNAPSE_GRPC_PORT`.

## Project Structure

```
.
├── cmd/                # Entrypoints
│   ├── server/main.go
│   └── worker/main.go
├── internal/           # Internal application logic
│   ├── config/
│   ├── models/
│   ├── queue/
│   ├── server/
│   └── worker/
├── protos/             # Protobuf definitions
│   └── generate.proto
├── grpc/               # Generated gRPC code
├── go.mod
├── Makefile
└── gen.sh              # Protobuf generation script
```
