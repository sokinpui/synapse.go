# Synapse

Simple distributed task queue system implement in gRPC and Redis

## Architecture

The system consists of two main components: a `server` and a `worker`.

```Client ---gRPC---> Server ---LPUSH---> Redis Queue ---BRPOP---> Worker
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

### 2. Start Redis

A `docker-compose.yml` file is provided to easily run a Redis instance.

```sh
docker-compose up -d
```

### 3. Build

Generate gRPC stubs and build server/worker binaries

```sh
./build.sh
```

Binaries will be placed in the `./bin` directory.

### 4. Run

A convenience script `start.sh` is provided to run both the server and the worker.

```sh
./start.sh
```

Alternatively, you can run them in separate terminals:

1.  **Start the Worker**:
    ```sh
    ./bin/worker
    ```
2.  **Start the Server**:
    ```sh
    ./bin/server
    ```

The server will listen for gRPC requests on the port specified by `SYNAPSE_GRPC_PORT`.

## Client Usage

A Go client is available in the `./client` directory. Here's a simple example of how to use it:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sokinpui/synapse.go/v2/client"
)

func main() {
	c, err := client.New("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	req := &client.GenerateRequest{
		Prompt:    "Tell me a joke",
		ModelCode: "gemini-pro",
		Stream:    true,
	}

	resultChan, err := c.GenerateTask(context.Background(), req)
	if err != nil {
		log.Fatalf("Failed to generate task: %v", err)
	}

	for result := range resultChan {
		if result.Err != nil {
			log.Printf("Error during generation: %v", result.Err)
			break
		}
		fmt.Print(result.Text)
	}
	fmt.Println()
}
```

## Project Structure

```
.
├── cmd/                # Entrypoints
│   ├── server/main.go
│   └── worker/main.go
├── client/             # Go client library
│   └── client.go
├── internal/           # Internal application logic
│   ├── config/
│   ├── models/
│   ├── queue/
│   ├── server/
│   └── worker/
├── protos/             # Protobuf definitions
│   └── generate.proto
├── grpc/               # Generated gRPC code
├── docker-compose.yml  # Docker compose for Redis
├── go.mod
├── build.sh            # Build script
└── start.sh            # Start script for server & worker
```
