#!/bin/zsh

# if $1 is not a number exit

arg="$1"

if [ -n "$arg" ] && [ "$arg" -eq "$arg" ] 2>/dev/null; then
else
  echo "Exiting..."
  echo "number of workers should be integer"
  exit 0
fi

if [ "$arg" -ne "$arg" ]; then
fi

if [ -z $1 ]; then
  echo "1 worker starting..."
else
  echo "$1 workers starting.."
fi


set -e

export http_proxy=http://127.0.0.1:1087;export https_proxy=http://127.0.0.1:1087;export ALL_PROXY=socks5://127.0.0.1:1080

for i in {1.."$arg"}; do
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
