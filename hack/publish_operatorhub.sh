#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

OPERATOR_NAME="coraza-kubernetes-operator"

show_help() {
    cat <<EOF
publish_operatorhub.sh - submit OLM bundle to community-operators

Usage: ./hack/publish_operatorhub.sh --version VERSION [options]

Required:
  --version VERSION      Bare semver version (no 'v' prefix), e.g. 0.2.0

Options:
  --owner OWNER          Upstream repo owner (default: k8s-operatorhub)
  --operator-hub REPO    Upstream repo name (default: community-operators)
  --fork FORK            Fork owner for the PR branch (default: \$GIT_USER)
  --dry-run              Skip push and PR creation, print what would happen
  -h, --help             Show this help
EOF
}

VERSION=""
OWNER="k8s-operatorhub"
OPERATOR_HUB="community-operators"
FORK=""
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)   VERSION="$2";       shift 2 ;;
        --owner)     OWNER="$2";         shift 2 ;;
        --operator-hub) OPERATOR_HUB="$2"; shift 2 ;;
        --fork)      FORK="$2";          shift 2 ;;
        --dry-run)   DRY_RUN=true;       shift ;;
        -h|--help)   show_help;          exit 0 ;;
        *) echo "Unknown option: $1" >&2; show_help; exit 1 ;;
    esac
done

if [[ -z "${VERSION}" ]]; then
    echo "Error: --version is required" >&2
    show_help
    exit 1
fi

# Strip leading 'v' if accidentally passed
VERSION="${VERSION#v}"

GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GIT_USER="${GIT_USER:-}"
GIT_EMAIL="${GIT_EMAIL:-}"

if [[ -z "${FORK}" ]]; then
    FORK="${GIT_USER}"
fi

if [[ -z "${GIT_USER}" ]]; then
    echo "Error: GIT_USER environment variable is required" >&2
    exit 1
fi

if [[ -z "${GIT_EMAIL}" ]]; then
    echo "Error: GIT_EMAIL environment variable is required" >&2
    exit 1
fi

if [[ "${DRY_RUN}" == false && -z "${GITHUB_TOKEN}" ]]; then
    echo "Error: GITHUB_TOKEN environment variable is required (or use --dry-run)" >&2
    exit 1
fi

skip_in_dry_run() {
    if [[ "${DRY_RUN}" == true ]]; then
        echo "[dry-run] $*"
        return 0
    fi
    "$@"
}

# Validate semver format
if [[ ! "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: version '${VERSION}' is not valid semver (expected X.Y.Z)" >&2
    exit 1
fi

BUNDLE_DIR="${REPO_ROOT}/bundle"
BUNDLE_MANIFESTS="${BUNDLE_DIR}/manifests"
BUNDLE_METADATA="${BUNDLE_DIR}/metadata"

if [[ ! -d "${BUNDLE_MANIFESTS}" || ! -d "${BUNDLE_METADATA}" ]]; then
    echo "Error: bundle not found at ${BUNDLE_DIR}. Run 'make bundle VERSION=v${VERSION}' first." >&2
    exit 1
fi

echo "Publishing ${OPERATOR_NAME} v${VERSION} to ${OWNER}/${OPERATOR_HUB}"

HUB_REPO_URL="https://github.com/${OWNER}/${OPERATOR_HUB}.git"
FORK_REPO_URL="https://github.com/${FORK}/${OPERATOR_HUB}.git"
BRANCH="${OPERATOR_NAME}-${VERSION}"

AUTH_HEADER="Authorization: token ${GITHUB_TOKEN}"

TMP_DIR="$(mktemp -d -t "${OPERATOR_NAME}.XXXXXXXXXX")"
skip_in_dry_run trap 'rm -rf -- "${TMP_DIR}"' EXIT

echo "Cloning ${OWNER}/${OPERATOR_HUB}..."
git clone --depth 1 "${HUB_REPO_URL}" "${TMP_DIR}"

cd "${TMP_DIR}"

git remote add fork "${FORK_REPO_URL}"

git config user.name "${GIT_USER}"
git config user.email "${GIT_EMAIL}"

git checkout -b "${BRANCH}"

OPERATORS_DIR="operators/${OPERATOR_NAME}/${VERSION}"
mkdir -p "${OPERATORS_DIR}/manifests"
mkdir -p "${OPERATORS_DIR}/metadata"
cp -a "${BUNDLE_MANIFESTS}/." "${OPERATORS_DIR}/manifests/"
cp -a "${BUNDLE_METADATA}/." "${OPERATORS_DIR}/metadata/"

CI_YAML_DEST="operators/${OPERATOR_NAME}/ci.yaml"
if [[ ! -f "${CI_YAML_DEST}" ]]; then
    cp "${BUNDLE_DIR}/ci.yaml" "${CI_YAML_DEST}"
    echo "Created ci.yaml (first submission)"
else
    echo "ci.yaml already exists, not overwriting"
fi

TITLE="operators ${OPERATOR_NAME} (${VERSION})"

git add "${OPERATORS_DIR}" "${CI_YAML_DEST}"
git commit -s -m "${TITLE}"

echo "Pushing branch ${BRANCH} to fork..."
skip_in_dry_run git -c "http.extraHeader=${AUTH_HEADER}" push fork "${BRANCH}"

echo "Opening PR against ${OWNER}/${OPERATOR_HUB}..."
PR_BODY="$(cat "${REPO_ROOT}/hack/operatorhub-pr-template.md")"

if [[ "${DRY_RUN}" == true ]]; then
    echo "[dry-run] Would create PR:"
    echo "  title: ${TITLE}"
    echo "  head:  ${FORK}:${BRANCH}"
    echo "  base:  main"
    echo "  repo:  ${OWNER}/${OPERATOR_HUB}"
else
    GITHUB_TOKEN="${GITHUB_TOKEN}" gh pr create \
        --repo "${OWNER}/${OPERATOR_HUB}" \
        --head "${FORK}:${BRANCH}" \
        --base main \
        --title "${TITLE}" \
        --body "${PR_BODY}"
fi

echo "Done."
