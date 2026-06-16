#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:="coraza-kubernetes-operator-integration"}
KIND_CONTEXT=${KIND_CONTEXT:="kind-${KIND_CLUSTER_NAME}"}
CONSOLE_PLUGIN_DIR=${CONSOLE_PLUGIN_DIR:="${REPO_ROOT}/console-plugin"}
CONSOLE_PLUGIN_DEV_SA=${CONSOLE_PLUGIN_DEV_SA:="console-dev"}
CONSOLE_PLUGIN_DEV_SA_NAMESPACE=${CONSOLE_PLUGIN_DEV_SA_NAMESPACE:="kube-system"}
YARN=${YARN:="yarn"}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 is required but not installed" >&2
    exit 1
  fi
}

require_command kubectl
require_command kind

if ! command -v docker >/dev/null 2>&1 && ! command -v podman >/dev/null 2>&1; then
  echo "docker or podman is required but not installed" >&2
  exit 1
fi

if ! kind get clusters 2>/dev/null | grep -qx "${KIND_CLUSTER_NAME}"; then
  echo "KIND cluster '${KIND_CLUSTER_NAME}' not found; run 'make cluster.kind' first" >&2
  exit 1
fi

kubectl config use-context "${KIND_CONTEXT}" >/dev/null

if ! kubectl get crd engines.waf.k8s.coraza.io >/dev/null 2>&1; then
  echo "Coraza CRDs are not installed on ${KIND_CONTEXT}; run 'make cluster.kind' first" >&2
  exit 1
fi

if ! kubectl get serviceaccount "${CONSOLE_PLUGIN_DEV_SA}" -n "${CONSOLE_PLUGIN_DEV_SA_NAMESPACE}" >/dev/null 2>&1; then
  echo "Creating dev ServiceAccount ${CONSOLE_PLUGIN_DEV_SA}/${CONSOLE_PLUGIN_DEV_SA_NAMESPACE}"
  kubectl create serviceaccount "${CONSOLE_PLUGIN_DEV_SA}" -n "${CONSOLE_PLUGIN_DEV_SA_NAMESPACE}"
fi

if ! kubectl get clusterrolebinding "${CONSOLE_PLUGIN_DEV_SA}" >/dev/null 2>&1; then
  echo "Creating dev ClusterRoleBinding ${CONSOLE_PLUGIN_DEV_SA} -> cluster-admin"
  kubectl create clusterrolebinding "${CONSOLE_PLUGIN_DEV_SA}" \
    --clusterrole=cluster-admin \
    --serviceaccount="${CONSOLE_PLUGIN_DEV_SA_NAMESPACE}:${CONSOLE_PLUGIN_DEV_SA}"
fi

if [ ! -x "${YARN}" ] && ! command -v "${YARN}" >/dev/null 2>&1; then
  echo "yarn is required; run 'make yarn' or set YARN to a yarn binary" >&2
  exit 1
fi

echo "Installing console plugin dependencies"
(cd "${CONSOLE_PLUGIN_DIR}" && "${YARN}" install)

echo "Console plugin dev setup complete (context: ${KIND_CONTEXT})"
