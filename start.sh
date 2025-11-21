#!/bin/bash

echo "Building Synapse Server..."
./build.sh
echo "Build complete."

echo "Starting Redis..."

docker compose up -d

echo "Redis started."

set -e

export http_proxy=http://127.0.0.1:1087
export https_proxy=http://127.0.0.1:1087
export ALL_PROXY=socks5://127.0.0.1:1080

echo "Starting server..."
$PWD/bin/server &

echo "Starting worker..."
$PWD/bin/worker &

cleanup() {
  echo
  echo "Cleaning up all child processes..."
  pkill -f "$PWD/bin/server"
  pkill -f "$PWD/bin/worker"
  echo "Cleanup complete."
}

trap cleanup INT TERM EXIT

echo "Server and workers are running. Press Ctrl+C to stop everything."

wait
