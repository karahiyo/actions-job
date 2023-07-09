#!/usr/bin/env bash

RUNNER_ASSETS_DIR=${RUNNER_ASSETS_DIR:-/runnertmp}
RUNNER_HOME=${RUNNER_HOME:-/home/runner}
RUNNER_WORKDIR=${RUNNER_WORKDIR:-${RUNNER_HOME}/_work}

if [ ! -d "${RUNNER_HOME}" ]; then
  echo "Create runner home directory: ${RUNNER_HOME}"
  sudo mkdir -p "${RUNNER_HOME}"
  sudo chown -R runner:docker "${RUNNER_HOME}"
fi

cp -r "$RUNNER_ASSETS_DIR"/* "$RUNNER_HOME"/

if ! cd "${RUNNER_HOME}"; then
  echo "Failed to cd into ${RUNNER_HOME}" >&2
  exit 1
fi

if [ -z "${ACCESS_TOKEN}" ]; then
  echo "ACCESS_TOKEN is not set" >&2
  exit 1
fi

if [ -z "${OWNER}" ]; then
  echo "OWNER is not set" >&2
  exit 1
fi

if [ -z "${REPO}" ]; then
  echo "REPO is not set" >&2
  exit 1
fi

if [ -z "${LABELS}" ]; then
  echo "LABELS is not set" >&2
  exit 1
fi

if [ -z "${RUNNER_NAME}" ]; then
  echo "RUNNER_NAME is not set, using Cloud Run Jobs ExecutionID instead"
  RUNNER_NAME=${CLOUD_RUN_EXECUTION}
fi

RUNNER_TOKEN=$(curl \
  -sf \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${OWNER}/${REPO}/actions/runners/registration-token" \
  | jq -r .token
)

config_args=()
if [ "${DISABLE_RUNNER_UPDATE:-}" == "true" ]; then
  config_args+=(--disableupdate)
  echo 'Passing --disableupdate to config.sh to disable automatic runner updates.'
fi

./config.sh --unattended \
  --url "https://github.com/${OWNER}/${REPO}" \
  --token "${RUNNER_TOKEN}" \
  --labels "${LABELS}" \
  --name "${RUNNER_NAME}" \
  --replace \
  --ephemeral \
  --work "${RUNNER_WORKDIR}" \
  "${config_args[@]}"

# Unset entrypoint environment variables so they don't leak into the runner environment
unset OWNER REPO LABELS ACCESS_TOKEN RUNNER_TOKEN RUNNER_NAME

mapfile -t env </etc/environment
exec env -- "${env[@]}" ./run.sh
