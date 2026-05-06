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
> **Critical Migration Constraints**
>
> **1. Existing PersistentVolumes and Secret Namespace:**
>
> Existing PersistentVolumes contain **immutable** references to secrets in the old namespace (e.g., `ceph-csi`). These secret references **cannot** be changed after a PV is created.
>
> This means:
> - You **MUST keep** the old CSI secrets in the original namespace (e.g., `ceph-csi`)
> - You **CANNOT** delete the old namespace where secrets are referenced
> - Existing volumes will **FAIL to mount** if the old secrets are deleted
>
> **2. Migration Downtime:**
>
> After removing existing Ceph-CSI components:
> - New Pods cannot mount PVCs until migration is completed
> - New PVCs or VolumeSnapshots cannot be created until drivers are running
> - Existing Pods using RBD/CephFS kernel mounter will continue to work (as long as they are not restarted)
> - Any Pod that restarts or gets rescheduled before the new CSI driver is running will fail to mount volumes
>
> **Plan maintenance windows accordingly.**



## Common Preparation Steps (Applies to Both YAML & Helm Tracks)

### Identify Your Existing Configuration

Before migrating, identify the `clusterID` used in your existing StorageClasses:

```bash
# View your existing StorageClass to find the clusterID
kubectl get storageclass <your-storageclass-name> -o yaml | grep clusterID
```

Note the `clusterID` value - you'll use this exact name for your ClientProfile.

### Backup the existing Ceph-CSI configuration

```bash
mkdir -p backup/ceph-csi
kubectl get configmap -n ceph-csi -o yaml > backup/ceph-csi/configmap.yaml
kubectl get deployment,daemonset -n ceph-csi -o yaml > backup/ceph-csi/workloads.yaml
kubectl get serviceaccount,role,rolebinding -n ceph-csi -o yaml > backup/ceph-csi/rbac.yaml
kubectl get csidriver -oyaml > backup/ceph-csi/csidriver.yaml
```

**Note:** Replace the namespace where ceph-CSI resources are created.


### Remove Existing Ceph-CSI Components (YAML-based deployments)

```bash
kubectl delete -f backup/ceph-csi/workloads.yaml
kubectl delete -f backup/ceph-csi/csidriver.yaml
kubectl delete -f backup/ceph-csi/rbac.yaml
kubectl delete -f backup/ceph-csi/configmaps.yaml
kubectl delete clusterrolebinding.rbac.authorization.k8s.io/rbd-csi-nodeplugin
kubectl delete clusterrole.rbac.authorization.k8s.io/rbd-csi-nodeplugin
kubectl delete clusterrolebinding.rbac.authorization.k8s.io/rbd-csi-provisioner-role
kubectl delete clusterrole.rbac.authorization.k8s.io/rbd-external-provisioner-runner
kubectl delete clusterrolebinding.rbac.authorization.k8s.io/nfs-csi-provisioner-role
kubectl delete clusterrole.rbac.authorization.k8s.io/nfs-external-provisioner-runner
kubectl delete clusterrolebinding.rbac.authorization.k8s.io/cephfs-csi-nodeplugin
kubectl delete clusterrole.rbac.authorization.k8s.io/cephfs-csi-nodeplugin
kubectl delete clusterrole.rbac.authorization.k8s.io/cephfs-external-provisioner-runner
kubectl delete clusterrolebinding.rbac.authorization.k8s.io/cephfs-csi-provisioner-role
```

Make sure the above yamls contains only the ceph-CSI resources before issuing delete.

### Remove Existing Ceph-CSI Helm Release (Helm-based deployments)

```bash
helm uninstall ceph-csi -n ceph-csi
```

### Install the Ceph-CSI-Operator

Install the operator using the [Installation Guide](installation.md) (stop before the "Create Ceph Secrets" section).

You will need to:
1. Install the operator
2. Deploy the CSI drivers (RBD, CephFS, NFS as needed)
3. Create CephConnection and ClientProfile CRs (see below)

> [!WARNING]
>  Important Migration Note for `ClusterID` Handling

In legacy deployments, `clusterID` is defined in:

- ConfigMap
- StorageClass
- VolumeSnapshotClass

In the operator-based deployments, these must be represented through a ClientProfile CR.

## Create CephConnection and ClientProfile

### Step 1: Extract Configuration from Existing Setup

First, extract the Ceph monitor addresses from your existing ConfigMap:

```bash
# View your existing ceph-csi ConfigMap
kubectl get configmap ceph-csi-config -n ceph-csi -o yaml
```

The ConfigMap contains monitor addresses under the `config.json` key.

### Step 2: Create CephConnection

Create a CephConnection using the monitor addresses from your existing ConfigMap:

```yaml
apiVersion: csi.ceph.io/v1
kind: CephConnection
metadata:
  name: ceph-connection
  namespace: ceph-csi-operator-system
spec:
  monitors:
    - <monitor-1>:6789  # From your existing ceph-csi ConfigMap
    - <monitor-2>:6789
    - <monitor-3>:6789
```

### Step 3: Create ClientProfile with Matching ClusterID

Create a ClientProfile with a name that **exactly matches** the `clusterID` from your existing StorageClass:

```yaml
apiVersion: csi.ceph.io/v1
kind: ClientProfile
metadata:
  name: my-ceph-cluster  # Must match the clusterID from your existing StorageClass
  namespace: ceph-csi-operator-system
spec:
  cephConnectionRef:
    name: ceph-connection
  cephFs:
    subVolumeGroup: csi  # Match your existing CephFS configuration
  rbd:
    radosNamespace: ""  # Match your existing RBD configuration
```

> [!IMPORTANT]
> **ClusterID Matching is Critical**
>
> The ClientProfile name **must exactly match** the `clusterID` value in your existing StorageClasses. This allows your existing StorageClasses to work seamlessly with the operator without modification.

## Understanding Secret Namespace Requirements During Migration

### The Secret Namespace Challenge

When you create a PersistentVolume using CSI, Kubernetes stores secret references directly in the PV object's spec. These fields are **immutable** and include:

- `spec.csi.nodeStageSecretRef` - Used when mounting the volume on a node
- `spec.csi.controllerExpandSecretRef` - Used when expanding the volume

Example from an existing PV:
```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pvc-12345678-abcd-1234-5678-123456789012
spec:
  csi:
    driver: rbd.csi.ceph.com
    nodeStageSecretRef:
      name: csi-rbd-secret
      namespace: ceph-csi  # This cannot be changed!
    controllerExpandSecretRef:
      name: csi-rbd-secret
      namespace: ceph-csi  # This cannot be changed!
```

### Required Actions

> [!IMPORTANT]
> **You MUST Keep Old Secrets in Their Original Namespace**
>
> **For Existing PersistentVolumes:**
> - Existing PVs reference secrets in the old namespace (e.g., `ceph-csi`)
> - These references **cannot be modified** after the PV is created
> - You **MUST keep** these secrets in the original namespace
> - **DO NOT** delete the old namespace or these secrets
> - If you delete these secrets, existing volumes will fail to mount with errors like:
>   ```text
>   failed to find the secret csi-rbd-secret in the namespace ceph-csi
>   ```
>
> **For New PersistentVolumes:**
> - New PVCs can use secrets in any namespace
> - The namespace is specified in your StorageClass parameters

### Migration Strategy Options

#### Option 1: Keep Everything in the Old Namespace (Simplest)

Continue using your existing StorageClasses and secrets:
- Keep secrets in the old namespace (e.g., `ceph-csi`)
- Keep using existing StorageClasses
- Both old and new PVs will use secrets from the old namespace

#### Option 2: Create New StorageClasses for New Workloads

For new applications, create new StorageClasses that reference secrets in a different namespace:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-rbd-operator  # New StorageClass name
provisioner: rbd.csi.ceph.com
parameters:
  clusterID: my-ceph-cluster
  pool: replicapool
  imageFeatures: layering
  # New secrets in a different namespace
  csi.storage.k8s.io/provisioner-secret-name: csi-rbd-secret
  csi.storage.k8s.io/provisioner-secret-namespace: ceph-csi-operator-system
  csi.storage.k8s.io/node-stage-secret-name: csi-rbd-secret
  csi.storage.k8s.io/node-stage-secret-namespace: ceph-csi-operator-system
  # ... other secret parameters
```

With this approach:
- Old PVCs/PVs continue using secrets from the old namespace
- New PVCs can use the new StorageClass with secrets in the new namespace
- You still cannot delete the old namespace until all old PVs are deleted

### Cleanup Limitations

> [!WARNING]
> **You Cannot Fully Clean Up the Old Namespace**
>
> As long as you have existing PersistentVolumes created by the old ceph-csi deployment:
> - You **cannot** delete the old namespace (e.g., `ceph-csi`)
> - You **cannot** delete the CSI secrets in that namespace
> - These resources must remain until:
>   1. All pods using old PVs are deleted
>   2. All old PVCs are deleted
>   3. All old PVs are deleted
>
> This is a limitation of Kubernetes CSI, not the ceph-csi-operator.

## 7. Verify the Migration

Once all the operator pods are up and running, verify the migration:

```bash
# Check operator and driver pods
kubectl get pods -n ceph-csi-operator-system

# Verify existing PVCs remain bound
kubectl get pvc --all-namespaces

# Test new provisioning with existing StorageClass
kubectl create -f <test-pvc.yaml>
```

The migration is complete when:
- All CSI driver pods are running
- Existing PVCs remain in `Bound` status
- New PVCs can be provisioned successfully

## 8. Clean Up

Once all the pods are up and running and you've verified that storage provisioning works correctly, the migration is complete. You can now delete the backup files:

```bash
rm -rf backup/
```
