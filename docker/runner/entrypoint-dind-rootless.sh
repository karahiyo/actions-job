#!/usr/bin/env bash

echo "Start Docker daemon (rootless)"
dockerd-rootless.sh  --iptables=false &

echo "Start GitHub Actions Runner"
startup.sh
