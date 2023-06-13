#!/usr/bin/env bash

echo "Start Docker daemon (rootless)"
dockerd-rootless.sh  --iptables=false &

for i in {1..5}; do
  if docker info &>/dev/null; then
    break
  fi
  echo "Waiting for Docker daemon to start..."
  sleep 1
done

docker info

echo "Start GitHub Actions Runner"
startup.sh
