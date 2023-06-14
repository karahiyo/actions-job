#!/usr/bin/env bash

mkdir -p /home/runner/.config/docker/

if [ ! -f /home/runner/.config/docker/daemon.json ]; then
  echo "{}" > /home/runner/.config/docker/daemon.json
fi

if [ -n "${MTU}" ]; then
jq ".\"mtu\" = ${MTU}" /home/runner/.config/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /home/runner/.config/docker/daemon.json
# See https://docs.docker.com/engine/security/rootless/ and https://github.com/docker/engine/blob/8955d8da8951695a98eb7e15bead19d402c6eb27/contrib/dockerd-rootless.sh#L13
echo "DOCKERD_ROOTLESS_ROOTLESSKIT_MTU=${MTU}" | sudo tee -a /etc/environment
fi

if [ -n "${DOCKER_DEFAULT_ADDRESS_POOL_BASE}" ] && [ -n "${DOCKER_DEFAULT_ADDRESS_POOL_SIZE}" ]; then
  jq ".\"default-address-pools\" = [{\"base\": \"${DOCKER_DEFAULT_ADDRESS_POOL_BASE}\", \"size\": ${DOCKER_DEFAULT_ADDRESS_POOL_SIZE}}]" /home/runner/.config/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /home/runner/.config/docker/daemon.json
fi

if [ -n "${DOCKER_REGISTRY_MIRROR}" ]; then
jq ".\"registry-mirrors\"[0] = \"${DOCKER_REGISTRY_MIRROR}\"" /home/runner/.config/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /home/runner/.config/docker/daemon.json
fi

echo "Start Docker daemon (rootless)"
export DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS=--debug
export DOCKERD_ROOTLESS_ROOTLESSKIT_NET=slirp4netns
export DOCKERD_ROOTLESS_ROOTLESSKIT_PORT_DRIVER=slirp4netns
dockerd-rootless.sh --config-file /home/runner/.config/docker/daemon.json &

for i in {1..5}; do
  if docker info &>/dev/null; then
    break
  fi
  echo "Waiting for Docker daemon to start..."
  sleep 1
done

if ! docker info; then
  echo "failed to start Docker daemon" >&2
  exit 1
fi

echo "Start GitHub Actions Runner"
startup.sh
