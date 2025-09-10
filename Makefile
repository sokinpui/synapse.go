# Makefile for the Synapse project

.PHONY: all build gen tidy clean

# Default target
all: build

# Generate gRPC code from protobuf definitions
gen:
	@echo "Generating gRPC code..."
	@./gen.sh

# Tidy Go module dependencies
tidy:
	@echo "Tidying Go modules..."
	@go mod tidy

# Build the server and worker binaries
build: gen tidy
	@echo "Building binaries..."
	@go build -o bin/server ./cmd/server
	@go build -o bin/worker ./cmd/worker
	@echo "Build complete. Binaries are in the 'bin' directory."

# Clean up generated files and binaries
clean:
	@echo "Cleaning up..."
	@rm -rf ./grpc/*
	@rm -rf ./bin
	@echo "Cleanup complete."
