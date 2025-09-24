#!/bin/bash

num_workers=${1:-1}

if ! [[ "$num_workers" =~ ^[1-9][0-9]*$ ]]; then
  echo "Exiting: Number of workers must be a positive integer." >&2
  exit 1
fi

set -e

export http_proxy=http://127.0.0.1:1087
export https_proxy=http://127.0.0.1:1087
export ALL_PROXY=socks5://127.0.0.1:1080

echo "Starting server..."
$PWD/bin/server &

echo "Starting $num_workers worker(s)..."
for i in $(seq 1 "$num_workers"); do
  $PWD/bin/worker &
done

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
