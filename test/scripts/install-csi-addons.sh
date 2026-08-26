#!/bin/bash -e

# This script installs/deletes the CSI-Addons controller, its RBAC and CRDs.
# The ceph-csi driver pods run a csi-addons sidecar and reference the
# csiaddonsnodes RBAC, so the CSIAddons controller and CRDs must be present in
# the cluster for addons operations (reclaim space, network fence, etc.).
# source https://github.com/csi-addons/kubernetes-csi-addons

SCRIPT_DIR="$(dirname "${0}")"

# shellcheck disable=SC1091
[ ! -e "${SCRIPT_DIR}"/utils.sh ] || source "${SCRIPT_DIR}"/utils.sh

CSI_ADDONS_VERSION=${CSI_ADDONS_VERSION:-"v0.14.0"}
CSI_ADDONS_NAMESPACE=${CSI_ADDONS_NAMESPACE:-"csi-addons-system"}

CSI_ADDONS_URL="https://raw.githubusercontent.com/csi-addons/kubernetes-csi-addons/${CSI_ADDONS_VERSION}/deploy/controller"

# CRDs (csiaddonsnodes, reclaimspacejobs, networkfences, etc.)
CSI_ADDONS_CRDS="${CSI_ADDONS_URL}/crds.yaml"
# controller RBAC (ClusterRoles, ClusterRoleBindings, ServiceAccount)
CSI_ADDONS_RBAC="${CSI_ADDONS_URL}/rbac.yaml"
# controller manager Deployment
CSI_ADDONS_CONTROLLER="${CSI_ADDONS_URL}/setup-controller.yaml"

function install_csi_addons() {
    kubectl_retry create -f "${CSI_ADDONS_CRDS}"
    kubectl_retry create -f "${CSI_ADDONS_CONTROLLER}"
    kubectl_retry create -f "${CSI_ADDONS_RBAC}"

    if ! kubectl_retry -n "${CSI_ADDONS_NAMESPACE}" rollout status \
        deployment/csi-addons-controller-manager --timeout=300s; then
        echo "csi-addons controller creation failed"
        kubectl_retry -n "${CSI_ADDONS_NAMESPACE}" get pods -o wide
        kubectl_retry -n "${CSI_ADDONS_NAMESPACE}" describe pods
        exit 1
    fi

    echo "csi-addons controller creation successful"
}

function cleanup_csi_addons() {
    kubectl delete -f "${CSI_ADDONS_CONTROLLER}" --ignore-not-found=true || true
    kubectl delete -f "${CSI_ADDONS_RBAC}" --ignore-not-found=true || true
    kubectl delete -f "${CSI_ADDONS_CRDS}" --ignore-not-found=true || true
}

case "${1:-}" in
install)
    install_csi_addons
    ;;
cleanup)
    cleanup_csi_addons
    ;;
*)
    echo "usage:" >&2
    echo "  $0 install" >&2
    echo "  $0 cleanup" >&2
    ;;
esac
