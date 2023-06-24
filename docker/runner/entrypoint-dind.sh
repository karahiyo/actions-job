#!/usr/bin/env bash

id

sudo /bin/bash <<SCRIPT
mkdir -p /etc/docker/

if [ ! -f /etc/docker/daemon.json ]; then
  echo "{}" > /etc/docker/daemon.json
fi

# userns remap
# jq ".\"userns-remap\" = \"runner:docker\"" /etc/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /etc/docker/daemon.json

if [ -n "${MTU}" ]; then
jq ".\"mtu\" = ${MTU}" /etc/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /etc/docker/daemon.json
# See https://docs.docker.com/engine/security/rootless/ and https://github.com/docker/engine/blob/8955d8da8951695a98eb7e15bead19d402c6eb27/contrib/dockerd-rootless.sh#L13
echo "DOCKERD_ROOTLESS_ROOTLESSKIT_MTU=${MTU}" | sudo tee -a /etc/environment
fi

if [ -n "${DOCKER_DEFAULT_ADDRESS_POOL_BASE}" ] && [ -n "${DOCKER_DEFAULT_ADDRESS_POOL_SIZE}" ]; then
  jq ".\"default-address-pools\" = [{\"base\": \"${DOCKER_DEFAULT_ADDRESS_POOL_BASE}\", \"size\": ${DOCKER_DEFAULT_ADDRESS_POOL_SIZE}}]" /etc/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /etc/docker/daemon.json
fi

if [ -n "${DOCKER_REGISTRY_MIRROR}" ]; then
jq ".\"registry-mirrors\"[0] = \"${DOCKER_REGISTRY_MIRROR}\"" /etc/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /etc/docker/daemon.json
fi

if [ -n "${DOCKER_INSECURE_REGISTRY}" ]; then
jq ".\"insecure-registries\"[0] = \"${DOCKER_INSECURE_REGISTRY}\"" /etc/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /etc/docker/daemon.json
fi

if [ "${DEBUG}" == "true" ]; then
  jq ".\"debug\" = ${DEBUG}" /etc/docker/daemon.json > /tmp/.daemon.json && mv /tmp/.daemon.json /etc/docker/daemon.json
fi

SCRIPT

dumb-init bash <<'SCRIPT' &
echo "Start Docker daemon"
sudo /usr/bin/dockerd &

if [ -n "${MTU}" ]; then
  sudo ifconfig docker0 mtu "${MTU}" up
fi

echo "Start GitHub Actions Runner"
startup.sh
SCRIPT

RUNNER_INIT_PID=$!
echo "Runner init started with pid $RUNNER_INIT_PID"
wait $RUNNER_INIT_PID

trap - TERM
