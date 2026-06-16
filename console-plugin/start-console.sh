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

PROMETHEUS_NODE_PORT=${PROMETHEUS_NODE_PORT:=30090}
PROMETHEUS_NAMESPACE=${PROMETHEUS_NAMESPACE:=monitoring}
PROMETHEUS_SVC_NAME=${PROMETHEUS_SVC_NAME:=kube-prometheus-stack-prometheus}
PROMETHEUS_PF_PID=""

cleanup_prometheus_port_forward() {
  if [ -n "${PROMETHEUS_PF_PID}" ] && kill -0 "${PROMETHEUS_PF_PID}" 2>/dev/null; then
    kill "${PROMETHEUS_PF_PID}" 2>/dev/null || true
    wait "${PROMETHEUS_PF_PID}" 2>/dev/null || true
  fi
  PROMETHEUS_PF_PID=""
}

prometheus_svc_exists() {
  kubectl --context="${KIND_CONTEXT}" get svc -n "${PROMETHEUS_NAMESPACE}" \
    "${PROMETHEUS_SVC_NAME}" >/dev/null 2>&1
}

start_kind_prometheus_port_forward() {
  cleanup_prometheus_port_forward
  kubectl --context="${KIND_CONTEXT}" port-forward -n "${PROMETHEUS_NAMESPACE}" \
    "svc/${PROMETHEUS_SVC_NAME}" "${PROMETHEUS_NODE_PORT}:9090" >/dev/null 2>&1 &
  PROMETHEUS_PF_PID=$!

  local attempt
  for attempt in $(seq 1 40); do
    if curl -sf "http://127.0.0.1:${PROMETHEUS_NODE_PORT}/-/ready" >/dev/null 2>&1; then
      return 0
    fi
    if ! kill -0 "${PROMETHEUS_PF_PID}" 2>/dev/null; then
      break
    fi
    sleep 0.25
  done

  echo "Prometheus port-forward to ${PROMETHEUS_SVC_NAME} failed" >&2
  cleanup_prometheus_port_forward
  return 1
}

resolve_kind_prometheus_url() {
  if [ -n "${PROMETHEUS_URL:-}" ]; then
    echo "${PROMETHEUS_URL}"
    return 0
  fi

  if ! prometheus_svc_exists; then
    return 1
  fi

  # KIND NodePorts are on the docker node IP, not 127.0.0.1. Port-forward to localhost
  # so the console container can reach Prometheus (with host.docker.internal on non-Linux).
  if start_kind_prometheus_port_forward; then
    echo "http://127.0.0.1:${PROMETHEUS_NODE_PORT}"
    return 0
  fi
  return 1
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

  local prometheus_url
  if prometheus_url=$(resolve_kind_prometheus_url); then
    BRIDGE_K8S_MODE_OFF_CLUSTER_THANOS="${prometheus_url}"
    echo "Prometheus query URL: ${BRIDGE_K8S_MODE_OFF_CLUSTER_THANOS} (via kubectl port-forward pid ${PROMETHEUS_PF_PID})"
  else
    echo "Prometheus not detected; console graphs will be unavailable (use make cluster.kind.monitoring)" >&2
  fi
}

echo "Starting local OpenShift console..."

trap cleanup_prometheus_port_forward EXIT INT TERM

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
  if [ -n "${BRIDGE_K8S_MODE_OFF_CLUSTER_THANOS:-}" ]; then
    BRIDGE_K8S_MODE_OFF_CLUSTER_THANOS=$(rewrite_localhost_endpoint "${BRIDGE_K8S_MODE_OFF_CLUSTER_THANOS}" "${PLUGIN_HOST}")
  fi
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
