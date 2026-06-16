#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CONSOLE_IMAGE=${CONSOLE_IMAGE:="quay.io/openshift/origin-console:4.21"}
CONSOLE_PORT=${CONSOLE_PORT:=9000}
CONSOLE_PLUGIN_DEV_PORT=${CONSOLE_PLUGIN_DEV_PORT:=9001}
CONSOLE_IMAGE_PLATFORM=${CONSOLE_IMAGE_PLATFORM:="linux/amd64"}
CONSOLE_PLUGIN_DEV_SA=${CONSOLE_PLUGIN_DEV_SA:="console-dev"}
CONSOLE_PLUGIN_DEV_SA_NAMESPACE=${CONSOLE_PLUGIN_DEV_SA_NAMESPACE:="kube-system"}
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:="coraza-kubernetes-operator-integration"}
KIND_CONTEXT=${KIND_CONTEXT:="kind-${KIND_CLUSTER_NAME}"}

read_plugin_name() {
  if command -v jq >/dev/null 2>&1; then
    jq -r '.consolePlugin.name' "${SCRIPT_DIR}/package.json"
  elif command -v node >/dev/null 2>&1; then
    node -p "require('${SCRIPT_DIR}/package.json').consolePlugin.name"
  else
    echo "coraza-console-plugin"
  fi
}

PLUGIN_NAME=$(read_plugin_name)

detect_container_runtime() {
  if [ -x "$(command -v podman)" ]; then
    echo podman
  elif [ -x "$(command -v docker)" ]; then
    echo docker
  else
    echo "docker or podman is required to run the OpenShift console" >&2
    exit 1
  fi
}

use_host_network() {
  case "$(uname -s)" in
    Linux) return 0 ;;
    *) return 1 ;;
  esac
}

rewrite_localhost_endpoint() {
  local endpoint=$1
  local host=$2
  if [[ "${endpoint}" =~ ^https://127\.0\.0\.1:([0-9]+)$ ]]; then
    echo "https://${host}:${BASH_REMATCH[1]}"
    return
  fi
  if [[ "${endpoint}" =~ ^http://127\.0\.0\.1:([0-9]+)$ ]]; then
    echo "http://${host}:${BASH_REMATCH[1]}"
    return
  fi
  if [[ "${endpoint}" =~ ^https://localhost:([0-9]+)$ ]]; then
    echo "https://${host}:${BASH_REMATCH[1]}"
    return
  fi
  if [[ "${endpoint}" =~ ^http://localhost:([0-9]+)$ ]]; then
    echo "http://${host}:${BASH_REMATCH[1]}"
    return
  fi
  echo "${endpoint}"
}

configure_openshift_cluster() {
  BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT=$(oc whoami --show-server)
  set +e
  BRIDGE_K8S_MODE_OFF_CLUSTER_THANOS=$(oc -n openshift-config-managed get configmap monitoring-shared-config -o jsonpath='{.data.thanosPublicURL}' 2>/dev/null)
  BRIDGE_K8S_MODE_OFF_CLUSTER_ALERTMANAGER=$(oc -n openshift-config-managed get configmap monitoring-shared-config -o jsonpath='{.data.alertmanagerPublicURL}' 2>/dev/null)
  GITOPS_HOSTNAME=$(oc -n openshift-gitops get route cluster -o jsonpath='{.spec.host}' 2>/dev/null)
  set -e
  BRIDGE_K8S_AUTH_BEARER_TOKEN=$(oc whoami --show-token 2>/dev/null || true)
  if [ -z "${BRIDGE_K8S_AUTH_BEARER_TOKEN}" ]; then
    BRIDGE_K8S_AUTH_BEARER_TOKEN=$(kubectl create token "${CONSOLE_PLUGIN_DEV_SA}" -n "${CONSOLE_PLUGIN_DEV_SA_NAMESPACE}" --duration=24h)
  fi
  if [ -n "${GITOPS_HOSTNAME}" ]; then
    BRIDGE_K8S_MODE_OFF_CLUSTER_GITOPS="https://${GITOPS_HOSTNAME}"
  fi
}

configure_kind_cluster() {
  if ! kubectl config get-contexts "${KIND_CONTEXT}" >/dev/null 2>&1; then
    echo "kubectl context ${KIND_CONTEXT} not found; run 'make cluster.kind' first" >&2
    exit 1
  fi

  BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT=$(
    kubectl --context="${KIND_CONTEXT}" config view --minify -o jsonpath='{.clusters[0].cluster.server}'
  )
  BRIDGE_K8S_AUTH_BEARER_TOKEN=$(
    kubectl --context="${KIND_CONTEXT}" create token "${CONSOLE_PLUGIN_DEV_SA}" \
      -n "${CONSOLE_PLUGIN_DEV_SA_NAMESPACE}" --duration=24h
  )
}

echo "Starting local OpenShift console..."

BRIDGE_USER_AUTH="disabled"
BRIDGE_K8S_MODE="off-cluster"
BRIDGE_K8S_AUTH="bearer-token"
BRIDGE_K8S_MODE_OFF_CLUSTER_SKIP_VERIFY_TLS=true
BRIDGE_USER_SETTINGS_LOCATION="localstorage"
BRIDGE_I18N_NAMESPACES="plugin__${PLUGIN_NAME}"

if kubectl config get-contexts "${KIND_CONTEXT}" >/dev/null 2>&1 &&
  command -v kind >/dev/null 2>&1 &&
  kind get clusters 2>/dev/null | grep -qx "${KIND_CLUSTER_NAME}"; then
  echo "Using KIND cluster credentials from kubectl (${KIND_CONTEXT})"
  configure_kind_cluster
elif command -v oc >/dev/null 2>&1 && oc whoami --show-server >/dev/null 2>&1; then
  echo "Using OpenShift cluster credentials from oc"
  configure_openshift_cluster
else
  echo "No usable cluster found; run 'make cluster.kind' or log in with oc" >&2
  exit 1
fi

if [ -z "${BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT}" ] || [ -z "${BRIDGE_K8S_AUTH_BEARER_TOKEN}" ]; then
  echo "Failed to resolve cluster API endpoint or bearer token" >&2
  exit 1
fi

CONTAINER_RUNTIME=$(detect_container_runtime)
RUN_ARGS=(--pull always --platform "${CONSOLE_IMAGE_PLATFORM}" --rm)
ENV_ARGS=()

if use_host_network; then
  BRIDGE_PLUGINS="${PLUGIN_NAME}=http://localhost:${CONSOLE_PLUGIN_DEV_PORT}"
  RUN_ARGS+=(--network=host)
else
  PLUGIN_HOST="host.docker.internal"
  BRIDGE_PLUGINS="${PLUGIN_NAME}=http://${PLUGIN_HOST}:${CONSOLE_PLUGIN_DEV_PORT}"
  BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT=$(rewrite_localhost_endpoint "${BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT}" "${PLUGIN_HOST}")
  RUN_ARGS+=(-p "${CONSOLE_PORT}:9000" --add-host="${PLUGIN_HOST}:host-gateway")
fi

while IFS= read -r line; do
  case "${line}" in
    BRIDGE_*)
      ENV_ARGS+=(--env "${line}")
      ;;
  esac
done < <(set | grep '^BRIDGE_')

echo "API Server: ${BRIDGE_K8S_MODE_OFF_CLUSTER_ENDPOINT}"
echo "Console Image: ${CONSOLE_IMAGE}"
echo "Console URL: http://localhost:${CONSOLE_PORT}"
echo "Console Platform: ${CONSOLE_IMAGE_PLATFORM}"
echo "Plugin: ${BRIDGE_PLUGINS}"

"${CONTAINER_RUNTIME}" run "${RUN_ARGS[@]}" "${ENV_ARGS[@]}" "${CONSOLE_IMAGE}"
