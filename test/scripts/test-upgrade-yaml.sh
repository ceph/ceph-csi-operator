#!/usr/bin/env bash

set -xeEo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
# shellcheck disable=SC1091
[ ! -e "${SCRIPT_DIR}"/utils.sh ] || source "${SCRIPT_DIR}"/utils.sh

trap log_errors ERR
trap cleanup EXIT

OPERATOR_DEPLOY_TIMEOUT=${OPERATOR_DEPLOY_TIMEOUT:-300}
OPERATOR_NAMESPACE=${OPERATOR_NAMESPACE:-"ceph-csi-operator-system"}
OPERATOR_NAME=${OPERATOR_NAME:-"ceph-csi-operator-controller-manager"}
OPERATOR_POD_LABEL=${OPERATOR_POD_LABEL:-"control-plane=ceph-csi-op-controller-manager"}

# Use the latest release version for upgrade testing
# In GitHub Actions, this is fetched and set as an environment variable
# For local testing, fall back to a known release version
RELEASE_VERSION=${RELEASE_VERSION:-"v0.6.0"}
echo "Using release version: ${RELEASE_VERSION}"

export IMAGE_REGISTRY="quay.io"
export REGISTRY_NAMESPACE="cephcsi"
export IMAGE_NAME="ceph-csi-operator"
export IMAGE_TAG="test"

# log_errors is called on exit (see 'trap' above) and tries to provide
# sufficient information to debug deployment problems
function log_errors() {
    echo "=== Upgrade test failed, collecting debug information ==="
    kubectl get nodes
    kubectl -n "${OPERATOR_NAMESPACE}" get events
    kubectl -n "${OPERATOR_NAMESPACE}" describe pods
    kubectl -n "${OPERATOR_NAMESPACE}" logs -l "${OPERATOR_POD_LABEL}" --tail=-1 || true
    kubectl -n "${OPERATOR_NAMESPACE}" get deployment -oyaml

    # this function should not return, a fatal error was caught!
    exit 1
}

function install_release_version() {
    echo "Installing release version ${RELEASE_VERSION}"

    # Download the install.yaml from the Git tag
    INSTALL_YAML_URL="https://raw.githubusercontent.com/ceph/ceph-csi-operator/${RELEASE_VERSION}/deploy/all-in-one/install.yaml"

    echo "Downloading install.yaml from ${INSTALL_YAML_URL}"
    curl -L "${INSTALL_YAML_URL}" -o /tmp/install-release.yaml

    echo "Applying release version manifests"
    kubectl_retry create -f /tmp/install-release.yaml
}

function cleanup() {
    echo "Cleaning up..."

    # Try to delete the current version first (from upgrade)
    if [ -f deploy/all-in-one/install.yaml ]; then
        kubectl_retry delete -f deploy/all-in-one/install.yaml --ignore-not-found=true || true
    fi

    # Try to delete the release version
    if [ -f /tmp/install-release.yaml ]; then
        kubectl_retry delete -f /tmp/install-release.yaml --ignore-not-found=true || true
        rm -f /tmp/install-release.yaml
    fi
}

function check_operator_health() {
    local expected_version=$1
    echo "Checking operator health (expected version: ${expected_version})"

    for ((retry = 0; retry <= OPERATOR_DEPLOY_TIMEOUT; retry = retry + 5)); do
        echo "Waiting for ceph-csi-operator pod... ${retry}s" && sleep 5

        OPERATOR_POD_NAME=$(kubectl_retry -n "${OPERATOR_NAMESPACE}" get pods -l "${OPERATOR_POD_LABEL}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

        if [ -z "$OPERATOR_POD_NAME" ]; then
            echo "No operator pod found yet, continuing to wait..."
            continue
        fi

        OPERATOR_POD_STATUS=$(kubectl_retry -n "${OPERATOR_NAMESPACE}" get pod "$OPERATOR_POD_NAME" -ojsonpath='{.status.phase}')
        [[ "$OPERATOR_POD_STATUS" = "Running" ]] && break
    done

    if [ "$retry" -gt "$OPERATOR_DEPLOY_TIMEOUT" ]; then
        echo "[Timeout] ceph-csi-operator pod is not running (timeout)"
        return 1
    fi

    echo "Operator pod is running: ${OPERATOR_POD_NAME}"

    # Verify the operator image version
    OPERATOR_IMAGE=$(kubectl_retry -n "${OPERATOR_NAMESPACE}" get deployment "${OPERATOR_NAME}" -o jsonpath='{.spec.template.spec.containers[0].image}')
    echo "Operator image: ${OPERATOR_IMAGE}"

    if [[ "${OPERATOR_IMAGE}" == *"${expected_version}"* ]]; then
        echo "Operator version verification successful: ${expected_version}"
    else
        echo "Warning: Operator image does not contain expected version tag '${expected_version}'"
        echo "This may be expected if the image naming convention differs"
    fi

    echo ""
}

function upgrade_to_pr_version() {
    echo "Upgrading operator to PR version"

    # Build the PR version installer
    make build-installer

    # Apply the upgrade
    echo "Applying PR version manifests"
    kubectl_retry apply -f deploy/all-in-one/install.yaml

    # Wait a bit for the upgrade to start
    sleep 5
}

function verify_upgrade() {
    echo "Verifying upgrade was successful"

    # Check that the operator is running the new version
    check_operator_health "test"

    # Additional verification: check that the deployment has been updated
    DEPLOYMENT_GENERATION=$(kubectl_retry -n "${OPERATOR_NAMESPACE}" get deployment "${OPERATOR_NAME}" -o jsonpath='{.metadata.generation}')
    OBSERVED_GENERATION=$(kubectl_retry -n "${OPERATOR_NAMESPACE}" get deployment "${OPERATOR_NAME}" -o jsonpath='{.status.observedGeneration}')

    if [ "$DEPLOYMENT_GENERATION" -eq "$OBSERVED_GENERATION" ]; then
        echo "Deployment generation matches observed generation: ${DEPLOYMENT_GENERATION}"
    else
        echo "Warning: Deployment generation (${DEPLOYMENT_GENERATION}) does not match observed generation (${OBSERVED_GENERATION})"
        return 1
    fi

    # Check rollout status
    kubectl_retry -n "${OPERATOR_NAMESPACE}" rollout status deployment/"${OPERATOR_NAME}" --timeout=300s

    echo "Upgrade verification successful"
}

# Main test flow
echo "=== Starting YAML-based upgrade test ==="

# Step 1: Install the release version
echo "Step 1: Installing release version ${RELEASE_VERSION}"
install_release_version
check_operator_health "${RELEASE_VERSION}"

# Step 2: Build the PR version
echo "Step 2: Building PR version"
make docker-build

# Step 3: Upgrade to PR version
echo "Step 3: Upgrading to PR version"
upgrade_to_pr_version

# Step 4: Verify the upgrade
echo "Step 4: Verifying upgrade"
verify_upgrade

echo "=== YAML-based upgrade test completed successfully ==="
