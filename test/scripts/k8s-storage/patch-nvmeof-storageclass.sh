#!/usr/bin/env bash

# Inject the running Rook NVMe-oF gateway's connection details into the e2e
# StorageClass before the tests run. The gateway Pod IP is only known at run
# time, so the address and listeners cannot be hard-coded in the checked-in
# StorageClass/values files.
#
# Usage:
#   patch-nvmeof-storageclass.sh kubectl [storageclass.yaml]
#   patch-nvmeof-storageclass.sh helm    [values-nvmeof.yaml]

set -xeEo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"

# Namespace where Rook and the NVMe-oF gateway are deployed.
ROOK_NAMESPACE="${ROOK_NAMESPACE:-rook-ceph}"

# Default targets for each deployment path.
DEFAULT_KUBECTL_SC="${SCRIPT_DIR}/kubectl/sc-nvmeof.yaml"
DEFAULT_HELM_VALUES="${SCRIPT_DIR}/helm/values-nvmeof.yaml"

# The gateway registers under a name that equals its Kubernetes Service name.
nvmeof_gateway_name() {
  kubectl -n "${ROOK_NAMESPACE}" get svc -l app=rook-ceph-nvmeof \
    -o jsonpath='{.items[0].metadata.name}'
}

# In-cluster Pod IP that initiators use to reach the data port (4420).
nvmeof_gateway_ip() {
  kubectl -n "${ROOK_NAMESPACE}" get pod -l app=rook-ceph-nvmeof \
    -o jsonpath='{.items[0].status.podIP}'
}

# The listener hostname must match the gateway name Rook registered; the address
# is the gateway Pod IP and the port is the NVMe-oF data port.
nvmeof_listeners_json() {
  local gw_name="$1" gw_ip="$2"
  printf '[{"hostname":"%s","address":"%s","port":4420}]' "${gw_name}" "${gw_ip}"
}

# patch_kubectl updates the kubectl-path StorageClass parameters in place.
patch_kubectl() {
  local sc_file="$1" gw_ip="$2" listeners="$3"
  GW_IP="${gw_ip}" LISTENERS="${listeners}" yq eval --inplace '
    .parameters.nvmeofGatewayAddress = strenv(GW_IP) |
    .parameters.nvmeofGatewayPort = "5500" |
    .parameters.listeners = strenv(LISTENERS)
  ' "${sc_file}"
  cat "${sc_file}"
}

# patch_helm updates the helm-path StorageClass values parameters in place.
patch_helm() {
  local values_file="$1" gw_ip="$2" listeners="$3"
  GW_IP="${gw_ip}" LISTENERS="${listeners}" yq eval --inplace '
    .drivers.nvmeof.storageClasses[0].parameters.nvmeofGatewayAddress = strenv(GW_IP) |
    .drivers.nvmeof.storageClasses[0].parameters.nvmeofGatewayPort = "5500" |
    .drivers.nvmeof.storageClasses[0].parameters.listeners = strenv(LISTENERS)
  ' "${values_file}"
  cat "${values_file}"
}

main() {
  local target="${1:-}"
  local file="${2:-}"

  local gw_name gw_ip listeners
  gw_name=$(nvmeof_gateway_name)
  gw_ip=$(nvmeof_gateway_ip)
  if [ -z "${gw_name}" ] || [ -z "${gw_ip}" ]; then
    echo "ERROR: could not discover the Rook NVMe-oF gateway (name='${gw_name}', ip='${gw_ip}')" >&2
    exit 1
  fi
  listeners=$(nvmeof_listeners_json "${gw_name}" "${gw_ip}")

  case "${target}" in
  kubectl)
    patch_kubectl "${file:-${DEFAULT_KUBECTL_SC}}" "${gw_ip}" "${listeners}"
    ;;
  helm)
    patch_helm "${file:-${DEFAULT_HELM_VALUES}}" "${gw_ip}" "${listeners}"
    ;;
  *)
    echo "Usage: $0 {kubectl|helm} [file]" >&2
    exit 1
    ;;
  esac
}

main "$@"
