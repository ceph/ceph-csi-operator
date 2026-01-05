# Ceph-CSI to Ceph-CSI-Operator Migration Guide

This guide provides two migration paths:

1. Migration from YAML-based Ceph-CSI deployment

2. Migration from Helm-based Ceph-CSI deployment

Ceph-CSI v3.16+ officially recommends using Ceph-CSI-Operator as the supported deployment mechanism.

## Why migrate?

- Operator provides declarative CRD-based management
- Automated reconciliation & healing
- Cleaner upgrades & version lifecycle
- Matches Kubernetes best practices


> [!WARNING]
> Important Warning (Read Before Proceeding)

**After removing the existing Ceph-CSI components, new Pods cannot mount PVCs, and new PVCs or VolumeSnapshots cannot be created until migration is completed.
Existing Pods using RBD/CephFS kernel mounter will continue to work, as long as they are not restarted.
Any Pod that restarts or gets rescheduled before the new CSI driver is running will fail to mount volumes.
Plan maintenance windows accordingly.**

> [!WARNING]
> The **Ceph-CSI-Operator does *not* automatically create StorageClasses or VolumeSnapshotClasses**.
>
> Legacy Ceph-CSI Helm charts provided automated creation of these objects, but the operator does not include this functionality.



## Common Preparation Steps (Applies to Both YAML & Helm Tracks)

### Backup the existing Ceph-CSI configuration if you have any

```bash
mkdir -p backup/ceph-csi
kubectl get configmap -n ceph-csi -o yaml > backup/ceph-csi/configmap.yaml
kubectl get deployment,daemonset -n ceph-csi -o yaml > backup/ceph-csi/workloads.yaml
kubectl get clusterrole,clusterrolebinding,serviceaccount,role,rolebinding -n ceph-csi -o yaml > backup/ceph-csi/rbac.yaml
kubectl get csidriver -oyaml > backup/ceph-csi/csidriver.yaml
```

**Note:** Replace the namespace where ceph-CSI resources are created.


### Remove Existing Ceph-CSI Components (YAML-based deployments)

```bash
kubectl delete -f backup/ceph-csi/workloads.yaml
kubectl delete -f backup/ceph-csi/csidriver.yaml
kubectl delete -f backup/ceph-csi/rbac.yaml
kubectl delete -f backup/ceph-csi/configmaps.yaml
```

Make sure the above yamls contains only the ceph-CSI resources before issuing delete.

### Remove Existing Ceph-CSI Helm Release (Helm-based deployments)

```bash
helm uninstall ceph-csi -n ceph-csi
```

### Install the Ceph-CSI-Operator

Follow the official [Installation Guide](installation.md) to deploy and configure the Ceph-CSI-Operator.

After installing the operator and creating the required CR (Drivers,CephConnection,ClientProfiles)

Ensure the CR definitions include fields that match your previous Ceph-CSI configuration (monitors, pools etc)

> [!WARNING]
>  Important Migration Note for `ClusterID` Handling

In legacy deployments, `clusterID` is defined in:

- ConfigMap
- StorageClass
- VolumeSnapshotClass

In the operator-based deployments, these must be represented through a ClientProfile CR.

### Requirement: ClientProfile CR

You must create a ClientProfile whose name matches the old `clusterID`, because:

- StorageClasses and SnapshotClasses will reference the ClientProfile name.

For example the `clusterID` was `ceph-csi` the ClientProfile CR looks like below

```yaml
apiVersion: csi.ceph.io/v1
kind: ClientProfile
metadata:
  name: ceph-csi
  namespace: ceph-csi-operator-system
spec:
  cephConnectionRef:
    name: ceph-connection
  ...
```

Once all the pods are up and running, the migration is complete. We can delete the backup files now.
