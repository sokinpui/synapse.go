#!/bin/bash

echo "Building Synapse Server..."
./build.sh
echo "Build complete."

# for local deploy, use vpn proxy
export http_proxy=http://127.0.0.1:1087
export https_proxy=http://127.0.0.1:1087
export ALL_PROXY=socks5://127.0.0.1:1080

echo "Starting server..."
$PWD/bin/server &

cleanup() {
  echo
  echo "Cleaning up all child processes..."
  pkill -f "$PWD/bin/server"
  echo "Cleanup complete."
}

trap cleanup INT TERM EXIT

echo "Server is running (with internal workers). Press Ctrl+C to stop."

wait
