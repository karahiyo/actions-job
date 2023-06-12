#!/usr/bin/env bash

# Start Docker daemon (rootless)
dockerd-rootless.sh  --iptables=false &

startup.sh
