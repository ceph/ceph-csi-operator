#!/usr/bin/env bash

set -xeEo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
# shellcheck disable=SC1091
[ ! -e "${SCRIPT_DIR}"/utils.sh ] || source "${SCRIPT_DIR}"/utils.sh

trap log_errors ERR
trap cleanup EXIT

OPERATOR_NAMESPACE=${OPERATOR_NAMESPACE:-"ceph-csi-operator-system"}
DRIVER_NAMESPACE=${DRIVER_NAMESPACE:-"csi-driver"}
OPERATOR_RELEASE_NAME=${OPERATOR_RELEASE_NAME:-"csi-operator"}
DRIVER_RELEASE_NAME=${DRIVER_RELEASE_NAME:-"csi-driver"}

# Use the latest release version for upgrade testing
RELEASE_VERSION=${RELEASE_VERSION:-"v0.6.0"}
# Helm chart version derived from release version (strip 'v' prefix), can be overridden
HELM_CHART_VERSION=${HELM_CHART_VERSION:-"${RELEASE_VERSION#v}"}

# Driver type from environment (set by CI matrix)
DRIVER_TYPE=${DRIVER_TYPE:-"rbd"}

export IMAGE_REGISTRY="quay.io"
export REGISTRY_NAMESPACE="cephcsi"
export IMAGE_NAME="ceph-csi-operator"
export IMAGE_TAG="test"

# log_errors is called on exit (see 'trap' above) and tries to provide
# sufficient information to debug deployment problems
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

    # this function should not return, a fatal error was caught!
    exit 1
}

function install_helm() {
    echo "Installing Helm"
    curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    helm version
}

function install_release_version() {
    echo "Installing release version ${RELEASE_VERSION} using Helm"

    # Add the Ceph CSI Helm repository
    helm repo add ceph-csi https://ceph.github.io/ceph-csi-operator || true
    helm repo update

    # Install operator chart from the repository
    echo "Installing operator chart version ${HELM_CHART_VERSION}"
    helm install "${OPERATOR_RELEASE_NAME}" ceph-csi/ceph-csi-operator \
        --version "${HELM_CHART_VERSION}" \
        --create-namespace \
        --namespace "${OPERATOR_NAMESPACE}" \
        --wait \
        --timeout 5m

    # Verify operator helm release
    helm status "${OPERATOR_RELEASE_NAME}" --namespace "${OPERATOR_NAMESPACE}"
}

function install_driver_release_version() {
    echo "Installing driver release version ${RELEASE_VERSION} using Helm"

    # Patch operator to watch driver namespace
    kubectl patch deployment "${OPERATOR_RELEASE_NAME}-ceph-csi-operator-controller-manager" \
        -n "${OPERATOR_NAMESPACE}" \
        --type='json' \
        -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/env/2/value", "value": "'"${DRIVER_NAMESPACE}"'"}]' || \
    kubectl patch deployment csi-operator-ceph-csi-operator-controller-manager \
        -n "${OPERATOR_NAMESPACE}" \
        --type='json' \
        -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/env/2/value", "value": "'"${DRIVER_NAMESPACE}"'"}]'

    # Wait for operator to restart
    sleep 10
    kubectl wait --for=condition=available --timeout=300s \
        -n "${OPERATOR_NAMESPACE}" \
        deployment -l app.kubernetes.io/name=ceph-csi-operator

    # Download the values.yaml for the driver chart and enable only the specified driver
    helm show values ceph-csi/ceph-csi-drivers --version "${HELM_CHART_VERSION}" > /tmp/driver-values.yaml

    # Enable only the specified driver
    DRIVER_NAME="${DRIVER_TYPE}.csi.ceph.com" yq eval --inplace '
    .drivers |=
    with_entries(
      .value.enabled = (.value.name == strenv(DRIVER_NAME)) |
      .value.deployCsiAddons = (.value.name == "rbd.csi.ceph.com") |
      .value.generateOMapInfo = ((.value.name == "cephfs.csi.ceph.com") or (.value.name == "rbd.csi.ceph.com"))
    )
    ' /tmp/driver-values.yaml

    # Install driver chart from the repository
    echo "Installing driver chart version ${HELM_CHART_VERSION} for ${DRIVER_TYPE}"
    helm install "${DRIVER_RELEASE_NAME}" ceph-csi/ceph-csi-drivers \
        --version "${HELM_CHART_VERSION}" \
        --create-namespace \
        --namespace "${DRIVER_NAMESPACE}" \
        --values /tmp/driver-values.yaml \
        --wait \
        --timeout 5m

    # Verify driver helm release
    helm status "${DRIVER_RELEASE_NAME}" --namespace "${DRIVER_NAMESPACE}"
}

function cleanup() {
    echo "Cleaning up..."

    # Uninstall driver chart
    helm uninstall "${DRIVER_RELEASE_NAME}" --namespace "${DRIVER_NAMESPACE}" --ignore-not-found || true

    # Uninstall operator chart
    helm uninstall "${OPERATOR_RELEASE_NAME}" --namespace "${OPERATOR_NAMESPACE}" --ignore-not-found || true

    # Delete namespaces
    kubectl delete namespace "${DRIVER_NAMESPACE}" --ignore-not-found=true || true
    kubectl delete namespace "${OPERATOR_NAMESPACE}" --ignore-not-found=true || true

    rm -f /tmp/driver-values.yaml
}

function check_operator_health() {
    local expected_phase=$1
    echo "Checking operator health (expected phase: ${expected_phase})"

    kubectl wait --for=condition=available --timeout=300s \
        -n "${OPERATOR_NAMESPACE}" \
        deployment -l app.kubernetes.io/name=ceph-csi-operator

    OPERATOR_POD_NAME=$(kubectl -n "${OPERATOR_NAMESPACE}" get pods -l app.kubernetes.io/name=ceph-csi-operator -o jsonpath='{.items[0].metadata.name}')
    echo "Operator pod: ${OPERATOR_POD_NAME}"

    OPERATOR_POD_STATUS=$(kubectl -n "${OPERATOR_NAMESPACE}" get pod "${OPERATOR_POD_NAME}" -ojsonpath='{.status.phase}')
    if [[ "$OPERATOR_POD_STATUS" != "Running" ]]; then
        echo "Operator pod is not running: ${OPERATOR_POD_STATUS}"
        return 1
    fi

    echo "Operator is healthy"
}

function check_driver_health() {
    echo "Checking driver health for ${DRIVER_TYPE}"

    # Determine expected pod count based on driver type
    local expected_pod_count=2
    if [ "$DRIVER_TYPE" = "rbd" ]; then
        expected_pod_count=3
    fi

    for _ in {1..180}; do
        running_pods=$(kubectl get pods -n "${DRIVER_NAMESPACE}" --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
        if [ "$running_pods" -eq "$expected_pod_count" ]; then
            echo "All ${expected_pod_count} CSI driver pods are running"
            kubectl get pods,deployment,daemonset,replicaset -n "${DRIVER_NAMESPACE}"
            return 0
        fi
        echo "Waiting for CSI driver pods to be ready (${running_pods}/${expected_pod_count})..."
        sleep 1
    done

    echo "Timeout: Not all CSI driver pods are running after 3 minutes"
    kubectl get pods,deployment,daemonset,replicaset -n "${DRIVER_NAMESPACE}"
    return 1
}

function upgrade_operator_to_pr_version() {
    echo "Upgrading operator to PR version using Helm"

    # Build the PR version image
    make docker-build

    # Upgrade the operator chart from local source
    helm upgrade "${OPERATOR_RELEASE_NAME}" ./deploy/charts/ceph-csi-operator \
        --namespace "${OPERATOR_NAMESPACE}" \
        --set controllerManager.manager.image.tag="${IMAGE_TAG}" \
        --wait \
        --timeout 5m

    # Verify upgrade
    helm status "${OPERATOR_RELEASE_NAME}" --namespace "${OPERATOR_NAMESPACE}"
}

function upgrade_driver_to_pr_version() {
    echo "Upgrading driver to PR version using Helm"

    # Enable only the specified driver in local chart
    DRIVER_NAME="${DRIVER_TYPE}.csi.ceph.com" yq eval --inplace '
    .drivers |=
    with_entries(
      .value.enabled = (.value.name == strenv(DRIVER_NAME)) |
      .value.deployCsiAddons = (.value.name == "rbd.csi.ceph.com") |
      .value.generateOMapInfo = ((.value.name == "cephfs.csi.ceph.com") or (.value.name == "rbd.csi.ceph.com"))
    )
    ' ./deploy/charts/ceph-csi-drivers/values.yaml

    # Upgrade the driver chart from local source
    helm upgrade "${DRIVER_RELEASE_NAME}" ./deploy/charts/ceph-csi-drivers \
        --namespace "${DRIVER_NAMESPACE}" \
        --wait \
        --timeout 5m

    # Restore original values.yaml if it was changed
    git checkout ./deploy/charts/ceph-csi-drivers/values.yaml || true

    # Verify upgrade
    helm status "${DRIVER_RELEASE_NAME}" --namespace "${DRIVER_NAMESPACE}"
}

function verify_upgrade() {
    echo "Verifying upgrade was successful"

    # Check operator health
    check_operator_health "upgraded"

    # Check driver health
    check_driver_health

    # Verify helm releases are in deployed state
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
echo "=== Starting Helm-based upgrade test for ${DRIVER_TYPE} driver ==="

# Step 0: Install Helm
echo "Step 0: Installing Helm"
install_helm

# Step 1: Install the release version
echo "Step 1: Installing operator release version ${RELEASE_VERSION}"
install_release_version
check_operator_health "release"

echo "Step 2: Installing driver release version ${RELEASE_VERSION}"
install_driver_release_version
check_driver_health

# Step 3: Upgrade operator to PR version
echo "Step 3: Upgrading operator to PR version"
upgrade_operator_to_pr_version

# Step 4: Upgrade driver to PR version
echo "Step 4: Upgrading driver to PR version"
upgrade_driver_to_pr_version

# Step 5: Verify the upgrade
echo "Step 5: Verifying upgrade"
verify_upgrade

echo "=== Helm-based upgrade test for ${DRIVER_TYPE} driver completed successfully ==="
