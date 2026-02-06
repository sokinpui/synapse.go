# Synapse

Simple distributed task queue system implement in gRPC and Go Channels

## Architecture

The system consists of two main components: a `server` (supporting gRPC and REST/JSON) and a `worker` running within the same process.

```
Client ---gRPC/HTTP---> Server <---Go Channels---> Worker
```

## Prerequisites

- Go (1.24+)
- Protobuf Compiler (`protoc`)

## Getting Started

### 1. Configuration

The application is configured using a `config.yaml` file. API keys for the LLM providers are configured using environment variables.

Create a `config.yaml` file in the root directory with the following content:

```yaml
server:
  grpc_port: 50051
  http_port: 8080

worker:
  # Multiple of CPU cores to use for processing requests
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

### 2. Run

Generate gRPC stubs, tidy modules, and build the server binary:

```sh
./start.sh
```

### 3. Docker

```
docker compose up -d
```

The server will listen for gRPC requests on the port specified in `config.yaml`.

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
		ModelCode: "gemini-2.5-pro",
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
		if result.IsKeepAlive {
			continue
		}
		fmt.Print(result.Text)
	}
	fmt.Println()
}
```

## HTTP/REST API

The server also exposes a REST/JSON API. You can send requests using `curl` or any HTTP client.

List Models:

```
curl http://localhost:8080/models
```

**Generate (Non-Streaming):**

```
curl -X POST http://localhost:8080/generate \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Why is the sky blue?",
    "model_code": "gemini-2.5-flash",
    "stream": false
  }'
```

**Generate (Streaming via SSE):**

```
curl -X POST http://localhost:8080/generate \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Write a long poem.",
    "model_code": "gemini-2.5-flash",
    "stream": true
  }'
```
