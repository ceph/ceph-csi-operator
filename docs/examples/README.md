# Example CRs for manifest-based installs

Ready-to-apply custom resources for users who install the Ceph-CSI
operator from manifests (`deploy/all-in-one/install.yaml`) rather than
Helm, and for projects (such as Rook) that link to operator examples.
These files mirror the inline snippets in the
[quick-start guide](../quick-start.md).

## Prerequisites

- The Ceph-CSI operator is installed:
  ```console
  kubectl create -f deploy/all-in-one/install.yaml
  ```
- A running Ceph cluster whose monitor addresses you know.

## Apply order

Resources must be created in this order:

```console
kubectl apply -f docs/examples/operatorconfig.yaml    # optional, cluster-wide defaults
kubectl apply -f docs/examples/drivers/               # rbd, cephfs, nfs, nvmeof drivers
kubectl apply -f docs/examples/cephconnection.yaml     # edit monitors first!
kubectl apply -f docs/examples/clientprofile.yaml
```

Edit `cephconnection.yaml` and replace `REPLACE_WITH_MONITOR_IP` with a
monitor address of your Ceph cluster before applying it.

### Finding your monitor addresses

How you find them depends on where Ceph runs:

- **Standalone or external Ceph cluster:** run `ceph mon dump` on the
  cluster (or ask your Ceph administrator). Each line shows a monitor
  address in the form `IP:6789`. One monitor is enough; listing two or
  three is recommended for fault tolerance.
- **Rook-managed Ceph in the same Kubernetes cluster:** read the
  endpoints Rook maintains:
  ```console
  kubectl -n rook-ceph get cm rook-ceph-mon-endpoints \
    -o jsonpath='{.data.data}'
  ```
  This prints something like `a=10.96.44.171:6789,b=10.96.77.125:6789` —
  use the `IP:port` parts (the leading letter is the monitor name).
  When using Rook, you will likely also want to change the namespace in
  these examples from `ceph-csi-operator-system` to `rook-ceph`.

## What each file does

| File | Kind | Purpose |
|------|------|---------|
| `operatorconfig.yaml` | OperatorConfig | Cluster-wide defaults for all drivers (optional — drivers run with built-in defaults without it) |
| `drivers/*.yaml` | Driver | One CR per protocol: rbd, cephfs, nfs, nvmeof |
| `cephconnection.yaml` | CephConnection | Monitor addresses of your Ceph cluster |
| `clientprofile.yaml` | ClientProfile | Named client config pointing at the connection |

## Notes

- **Namespace:** the examples use `ceph-csi-operator-system`. CRs must
  be created in a namespace the operator watches — by default that is
  only the operator's own namespace (`WATCH_NAMESPACE`). If your
  operator runs elsewhere (Rook commonly uses `rook-ceph`), change the
  namespace in every file to match.
- **OperatorConfig name:** must be `ceph-csi-operator-config` unless the
  operator was installed with a custom `OPERATOR_CONFIG_NAME` environment
  variable.
- **clusterID:** the ClientProfile name (`storage`) must match the
  `clusterID` parameter in your StorageClass and VolumeSnapshotClass
  resources. If you rename the ClientProfile, update the StorageClasses.
- **Placeholders:** only the Ceph monitor addresses in
  `cephconnection.yaml` need editing; everything else can be applied as-is.
- **Secrets, StorageClasses, VolumeSnapshotClasses and PVCs:** see the
  [ceph-csi examples](https://github.com/ceph/ceph-csi/tree/devel/examples)
  — they are independent of the operator and already maintained upstream.

## Coming from Rook?

If you are converting a Rook-managed cluster from Rook's legacy CSI
sidecars to the Ceph-CSI operator, watch for a few differences:

- **Provisioner names:** Rook prefixes driver names with its namespace
  (e.g. `rook-ceph.rbd.csi.ceph.com`); the operator's drivers are named
  `rbd.csi.ceph.com`, `cephfs.csi.ceph.com`, etc. Update the
  `provisioner` field in your StorageClasses accordingly.
- **clusterID:** Rook uses the cluster namespace as the `clusterID`;
  with the operator, the `clusterID` must match your ClientProfile CR
  name (`storage` in these examples).
- **Secrets:** Rook generates CSI secrets automatically. With a
  standalone operator install you create them yourself — see the
  [ceph-csi secret examples](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/secret.yaml).
- **OperatorConfig:** Rook already creates an OperatorConfig named
  `ceph-csi-operator-config` in its namespace. Skip
  `operatorconfig.yaml` on a Rook-managed cluster — applying it would
  modify Rook's configuration.

Also see "Finding your monitor addresses" above for reading the
monitor endpoints from a Rook cluster.
