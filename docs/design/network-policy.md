# NetworkPolicy for Ceph-CSI Operator and Driver Pods

This document describes the NetworkPolicy implementation for the
ceph-csi-operator and the CSI driver pods it manages.

## Component Inventory

| Component | Pod Labels | hostNetwork | NP Coverage |
|---|---|---|---|
| Operator pod | `control-plane: ceph-csi-op-controller-manager` | No | Static manifest |
| Controller-plugin (per driver) | `app: {driver}-ctrlplugin` | Configurable | Code-generated |
| csi-addons node agent (per driver) | `app: {driver}-nodeplugin-csi-addons` | No | Code-generated |
| Node-plugin DaemonSet | `app: {driver}-nodeplugin` | Yes (hardcoded) | Exempt |

## Policies

### Operator Pod

- **Ingress:** Deny all. No exposed ports (metrics disabled by default,
  kubelet health probes bypass NetworkPolicy).
- **Egress:** Open (`- {}`). Required for API server access. The API server
  ClusterIP varies per cluster (10.96.0.1 on kubeadm, 172.30.0.1 on OpenShift)
  and CNI ipBlock behavior is inconsistent across implementations, so
  restricting egress to a specific IP is not portable.

### Controller-Plugin

- **Skipped** when `hostNetwork: true` (NP is exempt).
- **Ingress** rules are conditional:
  1. (When `DeployCsiAddons: true`) csi-addons controller-manager →
     csi-addons gRPC port (9070 for RBD, 9080 for CephFS). Source
     restricted to pods with labels `app.kubernetes.io/name: csi-addons`
     and `control-plane: controller-manager` in any namespace.
  2. (RBD only, when TLS volume `tls-key` is configured) Any pod →
     snapshot-metadata gRPC port 50051. Open source because backup
     applications (Velero, etc.) can run in any namespace with any labels.
     See [CSI snapshot-metadata](https://kubernetes-csi.github.io/docs/external-snapshot-metadata.html).
- **Egress:** Open. Required for API server and Ceph cluster access.
- When no ingress rules apply, the NP is kept with an empty ingress
  list (deny-all ingress) for defense-in-depth.

### csi-addons Nodeplugin

- **Ingress:** csi-addons controller-manager → port 9071 only.
- **Egress:** Open.
- **Lifecycle:** Created when `DeployCsiAddons` is true. Not created for
  NFS drivers. Explicitly deleted when `DeployCsiAddons` is toggled
  false. The Driver CR is the controller owner reference for watch
  triggers and garbage collection on Driver deletion.

### Node-Plugin DaemonSet

No NetworkPolicy. Runs with `hostNetwork: true`, which makes it exempt
from NetworkPolicy enforcement.

## Inclusion

### Static Manifest (Operator Pod)

The operator pod NP is in `config/network-policy/` and referenced from
`config/default/kustomization.yaml`. It is included in all generated
manifests by default.

### Code-Generated NPs (Driver Pods)

Driver pod NPs are created unconditionally by the operator at runtime.
The `networking.k8s.io/v1` API group has been stable since Kubernetes 1.7
and is available on every supported cluster version.

## API Server Egress Rationale

Open egress (`- {}`) is used instead of restricting to a specific API server
IP because:

1. The API server ClusterIP varies per cluster and is unknown at manifest
   authoring time.
2. CNI implementations handle `ipBlock` inconsistently with DNAT (OVN-K8s
   timing, Cilium quirks).
3. HyperShift clusters may use non-standard API server ports.

This follows the precedent set by
[operator-marketplace PR #723](https://github.com/operator-framework/operator-marketplace/pull/723).

## References

- [CSI snapshot-metadata sidecar](https://kubernetes-csi.github.io/docs/external-snapshot-metadata.html)
- [kubernetes-csi/external-snapshot-metadata](https://github.com/kubernetes-csi/external-snapshot-metadata)
- [operator-marketplace NP precedent](https://github.com/operator-framework/operator-marketplace/pull/723)
