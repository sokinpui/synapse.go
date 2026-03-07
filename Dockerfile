# Build Stage
FROM golang:1.24-bookworm AS builder

WORKDIR /src

RUN apt-get update && apt-get install -y ca-certificates

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /bin/synapse-server ./cmd/server

# Runtime Stage
FROM ubuntu:24.04

WORKDIR /app

RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/synapse-server .
COPY --from=builder /src/config.yaml .

EXPOSE 8080

ENTRYPOINT ["./synapse-server"]
