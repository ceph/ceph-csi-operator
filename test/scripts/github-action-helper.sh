#!/usr/bin/env bash

set -xeEo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
# shellcheck disable=SC1091
[ ! -e "${SCRIPT_DIR}"/utils.sh ] || source "${SCRIPT_DIR}"/utils.sh

#############
# VARIABLES #
#############
: "${FUNCTION:=${1}}"

function create_extra_disk() {
  sudo apt install -y targetcli-fb open-iscsi
  truncate -s 75G ~/iscsi-disk.img
  sudo targetcli /backstores/fileio create disk1 ~/iscsi-disk.img 75G
  local target_iqn=iqn.2026-02.target.local:disk1
  sudo targetcli /iscsi create ${target_iqn}
  sudo targetcli /iscsi/${target_iqn}/tpg1/luns create /backstores/fileio/disk1
  local init_iqn=iqn.2026-02.initiator.local
  echo "InitiatorName=${init_iqn}" | sudo tee /etc/iscsi/initiatorname.iscsi >/dev/null
  sudo targetcli /iscsi/${target_iqn}/tpg1/acls create ${init_iqn}
  sudo targetcli /iscsi/${target_iqn}/tpg1/acls/${init_iqn} create tpg_lun_or_backstore=lun0 mapped_lun=0
  sudo iscsiadm -m discovery -t sendtargets -p 127.0.0.1
  sudo iscsiadm -m node --login
}

# source https://github.com/rook/rook
function find_extra_block_dev() {
  # shellcheck disable=SC2005 # redirect doesn't work with sudo, so use echo
  echo "$(sudo lsblk)" >/dev/stderr # print lsblk output to stderr for debugging in case of future errors
  # relevant lsblk --pairs example: (MOUNTPOINT identifies boot partition)(PKNAME is Parent dev ID)
  # NAME="sda15" SIZE="106M" TYPE="part" MOUNTPOINT="/boot/efi" PKNAME="sda"
  # NAME="sdb"   SIZE="75G"  TYPE="disk" MOUNTPOINT=""          PKNAME=""
  # NAME="sdb1"  SIZE="75G"  TYPE="part" MOUNTPOINT="/mnt"      PKNAME="sdb"
  boot_dev="$(sudo lsblk --noheading --list --output MOUNTPOINT,PKNAME | grep boot | awk '{print $2}')"
  echo "  == find_extra_block_dev(): boot_dev='$boot_dev'" >/dev/stderr # debug in case of future errors
  # --nodeps ignores partitions
  extra_dev="$(sudo lsblk --noheading --list --nodeps --output KNAME | grep -v loop | grep -v "$boot_dev" | head -1)"
  if [ -z "$extra_dev" ]; then
    create_extra_disk >/dev/stderr
    extra_dev="$(sudo lsblk --noheading --list --nodeps --output KNAME | grep -Ev "($boot_dev|loop|nbd)" | head -1)"
  fi
  echo "  == find_extra_block_dev(): extra_dev='$extra_dev'" >/dev/stderr # debug in case of future errors
  echo "$extra_dev"                                                       # output of function
}

: "${BLOCK:=$(find_extra_block_dev)}"

# source https://github.com/rook/rook
use_local_disk() {
  BLOCK_DATA_PART="/dev/${BLOCK}1"
  sudo apt purge snapd -y
  sudo dmsetup version || true
  sudo swapoff --all --verbose
  # Create an extra disk if doesn't exist.
  : "$(block_dev)"
  sudo lsblk

  unset pipefail
  mountpoint -q /mnt || return 0
  set pipefail
  sudo umount /mnt
  # search for the device since it keeps changing between sda and sdb
  sudo wipefs --all --force "$BLOCK_DATA_PART"
}

deploy_rook() {
  rook_version="v1.16.1"
  kubectl_retry create -f https://raw.githubusercontent.com/rook/rook/$rook_version/deploy/examples/common.yaml
  kubectl_retry create -f https://raw.githubusercontent.com/rook/rook/$rook_version/deploy/examples/crds.yaml
  curl https://raw.githubusercontent.com/rook/rook/$rook_version/deploy/examples/operator.yaml -o operator.yaml
  sed -i 's|ROOK_CSI_DISABLE_DRIVER: "false"|ROOK_CSI_DISABLE_DRIVER: "true"|g' operator.yaml
  kubectl_retry create -f operator.yaml
  wait_for_operator_pod_to_be_ready_state
  curl https://raw.githubusercontent.com/rook/rook/$rook_version/deploy/examples/cluster-test.yaml -o cluster-test.yaml
  sed -i "s|#deviceFilter:|deviceFilter: ${BLOCK/\/dev\//}|g" cluster-test.yaml
  cat cluster-test.yaml
  kubectl create -f cluster-test.yaml
  kubectl_retry create -f cluster-test.yaml
  kubectl_retry create -f https://raw.githubusercontent.com/rook/rook/$rook_version/deploy/examples/pool-test.yaml
  kubectl_retry create -f https://raw.githubusercontent.com/rook/rook/$rook_version/deploy/examples/filesystem-test.yaml
  kubectl_retry create -f https://raw.githubusercontent.com/rook/rook/$rook_version/deploy/examples/nfs-test.yaml
  wait_for_mon
  wait_for_pod_to_be_ready_state
  kubectl_retry create -f https://raw.githubusercontent.com/rook/rook/$rook_version/deploy/examples/toolbox.yaml
}

wait_for_osd_pod_to_be_ready_state() {
  timeout 200 bash <<-'EOF'
    until [ $(kubectl get pod -l app=rook-ceph-osd -n rook-ceph -o jsonpath='{.items[*].metadata.name}' -o custom-columns=READY:status.containerStatuses[*].ready | grep -c true) -eq 1 ]; do
      echo "waiting for the osd pods to be in ready state"
      sleep 1
    done
EOF
  timeout_command_exit_code
}

wait_for_operator_pod_to_be_ready_state() {
  timeout 100 bash <<-'EOF'
    until [ $(kubectl get pod -l app=rook-ceph-operator -n rook-ceph -o jsonpath='{.items[*].metadata.name}' -o custom-columns=READY:status.containerStatuses[*].ready | grep -c true) -eq 1 ]; do
      echo "waiting for the operator to be in ready state"
      sleep 1
    done
EOF
  timeout_command_exit_code
}

wait_for_mon() {
  timeout 150 bash <<-'EOF'
    until [ $(kubectl -n rook-ceph get deploy -l app=rook-ceph-mon,mon_canary!=true | grep rook-ceph-mon | wc -l | awk '{print $1}' ) -eq 1 ]; do
      echo "$(date) waiting for one mon deployment to exist"
      sleep 2
    done
EOF
  timeout_command_exit_code
}

timeout_command_exit_code() {
  # timeout command return exit status 124 if command times out
  if [ $? -eq 124 ]; then
    echo "Timeout reached"
    exit 1
  fi
}

install_minikube_with_none_driver() {
  CRICTL_VERSION="v1.35.0"
  MINIKUBE_VERSION="v1.38.0"
  kubernetes_version="v1.35.0"

  sudo apt update
  sudo apt install -y conntrack socat
  curl -LO https://storage.googleapis.com/minikube/releases/$MINIKUBE_VERSION/minikube_latest_amd64.deb
  sudo dpkg -i minikube_latest_amd64.deb
  rm -f minikube_latest_amd64.deb

  curl -LO https://github.com/Mirantis/cri-dockerd/releases/download/v0.3.24/cri-dockerd_0.3.24.3-0.ubuntu-focal_amd64.deb
  sudo dpkg -i cri-dockerd_0.3.24.3-0.ubuntu-focal_amd64.deb
  rm -f cri-dockerd_0.3.24.3-0.ubuntu-focal_amd64.deb

  wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$CRICTL_VERSION/crictl-$CRICTL_VERSION-linux-amd64.tar.gz
  sudo tar zxvf crictl-$CRICTL_VERSION-linux-amd64.tar.gz -C /usr/local/bin
  rm -f crictl-$CRICTL_VERSION-linux-amd64.tar.gz
  sudo sysctl fs.protected_regular=0

  CNI_PLUGIN_VERSION="v1.5.1"
  CNI_PLUGIN_TAR="cni-plugins-linux-amd64-$CNI_PLUGIN_VERSION.tgz" # change arch if not on amd64
  CNI_PLUGIN_INSTALL_DIR="/opt/cni/bin"

  curl -LO "https://github.com/containernetworking/plugins/releases/download/$CNI_PLUGIN_VERSION/$CNI_PLUGIN_TAR"
  sudo mkdir -p "$CNI_PLUGIN_INSTALL_DIR"
  sudo tar -xf "$CNI_PLUGIN_TAR" -C "$CNI_PLUGIN_INSTALL_DIR"
  rm "$CNI_PLUGIN_TAR"

  export MINIKUBE_HOME=$HOME CHANGE_MINIKUBE_NONE_USER=true KUBECONFIG=$HOME/.kube/config
  minikube start --kubernetes-version="$kubernetes_version" --driver=none --memory 6g --cpus=2 --addons ingress --cni=calico
  minikube logs
}

########
# MAIN #
########

FUNCTION="$1"
shift # remove function arg now that we've recorded it
# call the function with the remainder of the user-provided args
if ! $FUNCTION "$@"; then
  echo "Call to $FUNCTION was not successful" >&2
  exit 1
fi
