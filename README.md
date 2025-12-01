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

The application is configured using a `config.yaml` file. API keys for the LLM providers are still configured using environment variables.

Create a `config.yaml` file in the root directory with the following content:

```yaml
server:
  grpc_port: 50051

redis:
  host: localhost
  port: 6666 # Default port in docker-compose.yml
  db: 0
  password: "root" # Default password in docker-compose.yml

worker:
  concurrency_multiplier: 4

models:
  gemini:
    codes:
      - "gemini-2.5-pro"
  openrouter:
    codes:
      - "z-ai/glm-4.5-air:free"
```

# API key for the underlying LLM provider
Then, export the necessary API keys:

```sh
export GENAI_API_KEYS="YOUR_GEMINI_API_KEY_1,YOUR_GEMINI_API_KEY_2"
export OPENROUTER_API_KEY="YOUR_OPENROUTER_API_KEY"
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
