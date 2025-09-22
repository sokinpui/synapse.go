#!/bin/zsh

export http_proxy=http://127.0.0.1:1087;export https_proxy=http://127.0.0.1:1087;export ALL_PROXY=socks5://127.0.0.1:1080

for i in {1..10}; do
  ./bin/worker &
  worker_pids+=($!)
done

cleanup() {
  echo "Cleaning up worker processes..."
  for pid in "${worker_pids[@]}"; do
    kill "$pid"
  done
}


trap cleanup EXIT

echo "All workers started. Press Ctrl+C to stop the script and kill all workers."
wait
