#!/usr/bin/env bash

echo "Waiting for sidecar docker to start"
for i in {0..5}; do
  if docker info >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

docker info

startup.sh
