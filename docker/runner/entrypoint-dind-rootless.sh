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

if [ -d /home/runner/.local ]; then
  if [ ! -d /home/runner/.local/share ]; then
    log.notice "Creating /home/runner/.local/share owned by runner:runner \
so that rootless dockerd will not fail with a permission error when creating /home/runner/.local/share/docker"

    sudo mkdir /home/runner/.local/share
    sudo chmod 755 /home/runner/.local/share
    sudo chown runner:runner /home/runner/.local/share
  fi
fi

if [ "${DEBUG}" == "true" ]; then
  jq ".\"debug\" = ${DEBUG}" /home/runner/.config/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /home/runner/.config/docker/daemon.json
  export DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS=--debug
fi

dumb-init bash <<'SCRIPT' &
echo "Start Docker daemon (rootless)"
dockerd-rootless.sh --config-file /home/runner/.config/docker/daemon.json &

echo "Start GitHub Actions Runner"
startup.sh
SCRIPT

RUNNER_INIT_PID=$!
echo "Runner init started with pid $RUNNER_INIT_PID"
wait $RUNNER_INIT_PID

trap - TERM
