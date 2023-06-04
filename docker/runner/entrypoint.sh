#!/usr/bin/env bash

set -euo pipefail

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

registrationToken=$(curl \
  -sf \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "https://api.github.com/repos/${OWNER}/${REPO}/actions/runners/registration-token" \
  | jq -r .token
)

expect -c "
set timeout 10
log_user 0
spawn ./config.sh --url https://github.com/${OWNER}/${REPO} --token \"${registrationToken}\" --ephemeral --labels \"${LABELS}\"
log_user 1
expect -re \"Enter the name of the runner group to add this runner to:.*\"
send \"\n\"
expect -re \"Enter the name of runner:.*\"
send \"\n\"
expect -re \"Enter any additional labels.*\"
send \"\n\"
expect -re \"Enter name of work folder:.*\"
send \"\n\"
expect \"#\"
exit 0
"

./run.sh
