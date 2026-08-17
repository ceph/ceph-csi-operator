#!/usr/bin/env bash

set -xeEo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
# shellcheck disable=SC1091
[ ! -e "${SCRIPT_DIR}"/utils.sh ] || source "${SCRIPT_DIR}"/utils.sh

trap log_errors ERR

OPERATOR_NAMESPACE=${OPERATOR_NAMESPACE:-"ceph-csi-operator-system"}
DRIVER_NAMESPACE=${DRIVER_NAMESPACE:-"csi-driver"}
OPERATOR_RELEASE_NAME=${OPERATOR_RELEASE_NAME:-"csi-operator"}
DRIVER_RELEASE_NAME=${DRIVER_RELEASE_NAME:-"csi-driver"}

DRIVER_TYPE=${DRIVER_TYPE:-"rbd"}

export IMAGE_REGISTRY="quay.io"
export REGISTRY_NAMESPACE="cephcsi"
export IMAGE_NAME="ceph-csi-operator"
export IMAGE_TAG="test"

function log_errors() {
    echo "=== Helm upgrade test failed, collecting debug information ==="
    kubectl get nodes
    kubectl -n "${OPERATOR_NAMESPACE}" get events || true
    kubectl -n "${OPERATOR_NAMESPACE}" describe pods || true
    kubectl -n "${OPERATOR_NAMESPACE}" logs -l app.kubernetes.io/name=ceph-csi-operator --tail=-1 || true
    kubectl -n "${OPERATOR_NAMESPACE}" get deployment,pods -oyaml || true
    kubectl -n "${DRIVER_NAMESPACE}" get events || true
    kubectl -n "${DRIVER_NAMESPACE}" describe pods || true
    kubectl -n "${DRIVER_NAMESPACE}" get deployment,daemonset,pods -oyaml || true

    helm list -n "${OPERATOR_NAMESPACE}" || true
    helm list -n "${DRIVER_NAMESPACE}" || true

    exit 1
}

function upgrade_operator_to_pr_version() {
    echo "Upgrading operator to PR version using Helm"

    make docker-build

    helm upgrade "${OPERATOR_RELEASE_NAME}" ./deploy/charts/ceph-csi-operator \
        --namespace "${OPERATOR_NAMESPACE}" \
        --set controllerManager.manager.image.tag="${IMAGE_TAG}" \
        --set controllerManager.manager.env.watchNamespace="${DRIVER_NAMESPACE}" \
        --wait \
        --timeout 5m

    helm status "${OPERATOR_RELEASE_NAME}" --namespace "${OPERATOR_NAMESPACE}"
}

function upgrade_driver_to_pr_version() {
    echo "Upgrading driver to PR version using Helm"

    DRIVER_NAME="${DRIVER_TYPE}.csi.ceph.com" yq eval --inplace '
    .drivers |=
    with_entries(
      .value.enabled = (.value.name == strenv(DRIVER_NAME)) |
      .value.deployCsiAddons = (.value.name == "rbd.csi.ceph.com") |
      .value.generateOMapInfo = ((.value.name == "cephfs.csi.ceph.com") or (.value.name == "rbd.csi.ceph.com"))
    )
    ' ./deploy/charts/ceph-csi-drivers/values.yaml

    helm upgrade "${DRIVER_RELEASE_NAME}" ./deploy/charts/ceph-csi-drivers \
        --namespace "${DRIVER_NAMESPACE}" \
        --reuse-values \
        --wait \
        --timeout 5m

    git checkout ./deploy/charts/ceph-csi-drivers/values.yaml || true

    helm status "${DRIVER_RELEASE_NAME}" --namespace "${DRIVER_NAMESPACE}"
}

function verify_upgrade() {
    echo "Verifying upgrade was successful"

    OPERATOR_STATUS=$(helm status "${OPERATOR_RELEASE_NAME}" -n "${OPERATOR_NAMESPACE}" -o json | jq -r '.info.status')
    DRIVER_STATUS=$(helm status "${DRIVER_RELEASE_NAME}" -n "${DRIVER_NAMESPACE}" -o json | jq -r '.info.status')

    if [[ "${OPERATOR_STATUS}" != "deployed" ]]; then
        echo "Operator helm release is not in 'deployed' state: ${OPERATOR_STATUS}"
        return 1
    fi

    if [[ "${DRIVER_STATUS}" != "deployed" ]]; then
        echo "Driver helm release is not in 'deployed' state: ${DRIVER_STATUS}"
        return 1
    fi

    echo "Upgrade verification successful"
}

# Main test flow
# Expects: operator and driver charts already installed by deploy-charts composite action
echo "=== Starting Helm upgrade test for ${DRIVER_TYPE} driver ==="

echo "Step 1: Upgrading operator to PR version"
upgrade_operator_to_pr_version
check_operator_health

echo "Step 2: Upgrading driver to PR version"
upgrade_driver_to_pr_version
check_driver_health "${DRIVER_TYPE}"

echo "Step 3: Verifying upgrade"
verify_upgrade

echo "=== Helm upgrade test for ${DRIVER_TYPE} driver completed successfully ==="
