#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

CONSOLE_PLUGIN_DIR=${CONSOLE_PLUGIN_DIR:="${REPO_ROOT}/console-plugin"}
CONSOLE_PLUGIN_DEV_PORT=${CONSOLE_PLUGIN_DEV_PORT:=9001}
CONSOLE_PORT=${CONSOLE_PORT:=9000}
YARN=${YARN:="yarn"}
WEBPACK_PID=""

cleanup() {
  local exit_code=$?
  if [ -n "${WEBPACK_PID}" ] && kill -0 "${WEBPACK_PID}" >/dev/null 2>&1; then
    echo "Stopping webpack dev server (pid ${WEBPACK_PID})"
    kill "${WEBPACK_PID}" >/dev/null 2>&1 || true
    wait "${WEBPACK_PID}" 2>/dev/null || true
  fi
  if [ "${exit_code}" -eq 0 ]; then
    echo "Console plugin dev session ended. Open http://localhost:${CONSOLE_PORT}/coraza to use the plugin."
  fi
}

trap cleanup EXIT INT TERM

"${REPO_ROOT}/hack/console-plugin-dev-setup.sh"

echo "Starting webpack dev server on port ${CONSOLE_PLUGIN_DEV_PORT}"
(
  cd "${CONSOLE_PLUGIN_DIR}"
  exec "${YARN}" start
) &
WEBPACK_PID=$!

webpack_ready() {
  curl -sS -o /dev/null "http://localhost:${CONSOLE_PLUGIN_DEV_PORT}/" 2>/dev/null
}

echo "Waiting for webpack dev server..."
deadline=$((SECONDS + 30))
while [ "${SECONDS}" -lt "${deadline}" ]; do
  if webpack_ready; then
    break
  fi
  if ! kill -0 "${WEBPACK_PID}" >/dev/null 2>&1; then
    echo "webpack dev server exited before becoming ready" >&2
    wait "${WEBPACK_PID}" || true
    exit 1
  fi
  sleep 1
done

if ! webpack_ready; then
  echo "Timed out waiting for webpack dev server on port ${CONSOLE_PLUGIN_DEV_PORT}" >&2
  exit 1
fi

echo "webpack dev server is ready"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-coraza-kubernetes-operator-integration}" \
KIND_CONTEXT="${KIND_CONTEXT:-kind-${KIND_CLUSTER_NAME}}" \
CONSOLE_IMAGE="${CONSOLE_IMAGE:-quay.io/openshift/origin-console:4.21}" \
CONSOLE_PORT="${CONSOLE_PORT}" \
CONSOLE_PLUGIN_DEV_PORT="${CONSOLE_PLUGIN_DEV_PORT}" \
CONSOLE_PLUGIN_DEV_SA="${CONSOLE_PLUGIN_DEV_SA:-console-dev}" \
exec "${CONSOLE_PLUGIN_DIR}/start-console.sh"
