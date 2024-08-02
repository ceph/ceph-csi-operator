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

export IMAGE_REGISTRY="quay.io"
export REGISTRY_NAMESPACE="cephcsi"
export IMAGE_NAME="ceph-csi-operator"
export IMAGE_TAG="test"

# log_errors is called on exit (see 'trap' above) and tries to provide
# sufficient information to debug deployment problems
function log_errors() {
    kubectl get nodes
    kubectl -n ${OPERATOR_NAMESPACE} get events
    kubectl -n ${OPERATOR_NAMESPACE} describe pods
    kubectl -n ${OPERATOR_NAMESPACE} logs -l ${OPERATOR_POD_LABEL} --tail=-1
    kubectl -n ${OPERATOR_NAMESPACE} get deployment -oyaml

    # this function should not return, a fatal error was caught!
    exit 1
}

function install_operator() {
    make build-installer
    kubectl_retry create -f dist/install.yaml
}

function cleanup() {
    echo "Uninstalling the operator..."
    kubectl_retry delete -f dist/install.yaml
}

function check_operator_health() {
    for ((retry = 0; retry <= OPERATOR_DEPLOY_TIMEOUT; retry = retry + 5)); do
        echo "Waiting for ceph-csi-operator pod... ${retry}s" && sleep 5

        OPERATOR_POD_NAME=$(kubectl_retry -n ${OPERATOR_NAMESPACE} get pods -l ${OPERATOR_POD_LABEL} -o jsonpath='{.items[0].metadata.name}')
        OPERATOR_POD_STATUS=$(kubectl_retry -n ${OPERATOR_NAMESPACE} get pod "$OPERATOR_POD_NAME" -ojsonpath='{.status.phase}')
        [[ "$OPERATOR_POD_STATUS" = "Running" ]] && break
    done

    if [ "$retry" -gt "$OPERATOR_DEPLOY_TIMEOUT" ]; then
        echo "[Timeout] ceph-csi-operator pod is not running (timeout)"
        return 1
    fi
    echo ""
}

# Build the operator
make docker-build
# Install the operator
install_operator
# Check the operator health
check_operator_health
