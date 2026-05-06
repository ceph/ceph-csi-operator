# Ceph-CSI-Operator Release Installation Guide

---
- [Ceph-CSI-Operator Release Installation Guide](#ceph-csi-operator-release-installation-guide)
  - [1. Prerequisites](#1-prerequisites)
    - [Clone the Repository](#clone-the-repository)
  - [2. Kubernetes Installation](#2-kubernetes-installation)
    - [2.1 All-in-One Installation](#21-all-in-one-installation)
      - [Install the Operator](#install-the-operator)
      - [Verify Installation](#verify-installation)
    - [2.2 Multi-File Installation](#22-multi-file-installation)
      - [Step 1: Install CRDs](#step-1-install-crds)
      - [Step 2: Create RBAC Resources](#step-2-create-rbac-resources)
      - [Step 3: Install the Operator](#step-3-install-the-operator)
      - [Step 4: Verify Installation](#step-4-verify-installation)
  - [3. OpenShift Installation](#3-openshift-installation)
    - [Prerequisites](#prerequisites)
    - [3.1 All-in-One Installation](#31-all-in-one-installation)
      - [Install the Operator with SCC](#install-the-operator-with-scc)
      - [Verify Installation](#verify-installation-1)
      - [Verify SCC Resources](#verify-scc-resources)
    - [3.2 Multi-File Installation](#32-multi-file-installation)
      - [Step 1: Install CRDs](#step-1-install-crds-1)
      - [Step 2: Create RBAC Resources](#step-2-create-rbac-resources-1)
      - [Step 3: Create OpenShift SCC](#step-3-create-openshift-scc)
      - [Step 4: Install the Operator](#step-4-install-the-operator)
      - [Step 5: Verify Installation](#step-5-verify-installation)
  - [4. Deploy Ceph-CSI Drivers](#4-deploy-ceph-csi-drivers)
    - [4.1 Deploy the RBD Driver](#41-deploy-the-rbd-driver)
    - [4.2 Deploy the CephFS Driver](#42-deploy-the-cephfs-driver)
    - [4.3 Deploy the Ceph-NFS Driver](#43-deploy-the-ceph-nfs-driver)
  - [5. Verify Installation](#5-verify-installation)
  - [6. Create CephConnection](#6-create-cephconnection)
  - [7. Create ClientProfile](#7-create-clientprofile)
  - [8. Create Ceph Secrets](#8-create-ceph-secrets)
  - [9. Create StorageClasses](#9-create-storageclasses)
  - [10. Create VolumeSnapshotClasses (Optional)](#10-create-volumesnapshotclasses-optional)
  - [11. Verify Storage Provisioning](#11-verify-storage-provisioning)
  - [12. Upgrade Ceph-CSI Operator and Drivers](#12-upgrade-ceph-csi-operator-and-drivers)
    - [Step 1: Fetch and Checkout the Latest Tag](#step-1-fetch-and-checkout-the-latest-tag)
    - [Step 2: Apply Updated Manifests](#step-2-apply-updated-manifests)
      - [For Kubernetes (All-in-One)](#for-kubernetes-all-in-one)
      - [For OpenShift (All-in-One)](#for-openshift-all-in-one)
      - [For Multi-File Installation](#for-multi-file-installation)
    - [Step 3: Verify the Upgrade](#step-3-verify-the-upgrade)
  - [13. Clean Up Resources](#13-clean-up-resources)
    - [Step 1: Delete Custom Resources](#step-1-delete-custom-resources)
    - [Step 2: Uninstall the Operator](#step-2-uninstall-the-operator)
      - [For Kubernetes](#for-kubernetes)
      - [For OpenShift](#for-openshift)
      - [For Multi-File Installation](#for-multi-file-installation-1)
    - [Step 3: Verify Deletion](#step-3-verify-deletion)

## 1. Prerequisites

Before proceeding with the installation of the Ceph-CSI Operator, ensure the following requirements are met:

- **Kubernetes or OpenShift Cluster:**
  - Kubernetes: A running cluster with a supported version ([Kubernetes Release Versions](https://kubernetes.io/releases/))
  - OpenShift: Version 4.19 or later with cluster administrator privileges
- **Ceph Cluster:** A Ceph cluster with a supported version ([Ceph Releases](https://docs.ceph.com/en/latest/releases/))
- **CLI Tools:**
  - `kubectl` for Kubernetes clusters
  - `oc` or `kubectl` for OpenShift clusters

### Clone the Repository

For all installation methods, clone the Ceph-CSI-Operator repository and checkout the desired release tag:

```console
git clone https://github.com/ceph/ceph-csi-operator.git
cd ceph-csi-operator
git checkout v0.3.1
```

**Note:** Check out the latest tag from [Releases](https://github.com/ceph/ceph-csi-operator/releases).

---

## 2. Kubernetes Installation

Choose either the All-in-One or Multi-File installation method based on your requirements.

### 2.1 All-in-One Installation

The All-in-One installation deploys all components (CRDs, RBAC, operator) in a single command.

#### Install the Operator

```console
kubectl create -f deploy/all-in-one/install.yaml
```

This creates:
- Custom Resource Definitions (CRDs)
- RBAC resources (Role-Based Access Control)
- Ceph-CSI Operator deployment

#### Verify Installation

```bash
kubectl get pods -n ceph-csi-operator-system
```

Expected output:

```text
NAME                                                    READY   STATUS    RESTARTS   AGE
ceph-csi-operator-controller-manager-67d45fd9ff-zgst7   2/2     Running   0          40s
```

### 2.2 Multi-File Installation

The Multi-File installation provides finer control by deploying components separately.

#### Step 1: Install CRDs

```console
kubectl create -f deploy/multifile/crd.yaml
```

This creates the Custom Resource Definitions: `CephConnection`, `ClientProfile`, `ClientProfileMapping`, and `Driver`.

#### Step 2: Create RBAC Resources

Create RBAC resources in the namespace where you plan to install the Ceph-CSI drivers:

```console
kubectl create -f deploy/multifile/csi-rbac.yaml -n ceph-csi-operator-system
```

#### Step 3: Install the Operator

```console
kubectl create -f deploy/multifile/operator.yaml -n ceph-csi-operator-system
```

#### Step 4: Verify Installation

```bash
kubectl get pods -n ceph-csi-operator-system
```

Expected output:

```text
NAME                                                    READY   STATUS    RESTARTS   AGE
ceph-csi-operator-controller-manager-67d45fd9ff-zgst7   2/2     Running   0          40s
```

---

## 3. OpenShift Installation

When deploying on OpenShift, additional SecurityContextConstraints (SCC) resources are required to grant necessary permissions for CSI operations.

### Prerequisites

- OpenShift 4.x cluster
- Cluster administrator privileges to create SecurityContextConstraints

### 3.1 All-in-One Installation

The recommended method for OpenShift is the all-in-one installer that includes SCC resources.

#### Install the Operator with SCC

```console
kubectl create -f deploy/all-in-one/install-openshift.yaml
```

This creates:
- All operator components (CRDs, RBAC, deployment)
- SecurityContextConstraint (`ceph-csi-scc`) with necessary host-level permissions
- ClusterRole to use the SCC
- ClusterRoleBindings for all CSI service accounts (RBD, CephFS, NFS, NVMe-oF)

#### Verify Installation

```bash
kubectl get pods -n ceph-csi-operator-system
```

Expected output:

```text
NAME                                                    READY   STATUS    RESTARTS   AGE
ceph-csi-operator-controller-manager-67d45fd9ff-zgst7   2/2     Running   0          40s
```

#### Verify SCC Resources

```bash
oc get scc | grep ceph-csi
```

Expected output:

```text
ceph-csi-scc   false   []        RunAsAny   RunAsAny   RunAsAny   RunAsAny   <none>     false
```

### 3.2 Multi-File Installation

For more granular control, you can install components separately.

#### Step 1: Install CRDs

```console
kubectl create -f deploy/multifile/crd.yaml
```

#### Step 2: Create RBAC Resources

```console
kubectl create -f deploy/multifile/csi-rbac.yaml -n ceph-csi-operator-system
```

#### Step 3: Create OpenShift SCC

```console
kubectl create -f deploy/multifile/openshift-scc.yaml
```

This creates the SecurityContextConstraints and necessary RBAC for CSI service accounts.

#### Step 4: Install the Operator

```console
kubectl create -f deploy/multifile/operator.yaml -n ceph-csi-operator-system
```

#### Step 5: Verify Installation

```bash
kubectl get pods -n ceph-csi-operator-system
oc get scc | grep ceph-csi
```

---

## 4. Deploy Ceph-CSI Drivers

Once the operator is installed (on either Kubernetes or OpenShift), deploy the required CSI drivers.

### 4.1 Deploy the RBD Driver

```console
echo '
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: rbd.csi.ceph.com
  namespace: ceph-csi-operator-system
' | kubectl create -f -
```

### 4.2 Deploy the CephFS Driver

```console
echo '
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: cephfs.csi.ceph.com
  namespace: ceph-csi-operator-system
' | kubectl create -f -
```

### 4.3 Deploy the Ceph-NFS Driver

```console
echo '
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: nfs.csi.ceph.com
  namespace: ceph-csi-operator-system
' | kubectl create -f -
```

---

## 5. Verify Installation

Verify that all CSI driver components are running:

```bash
kubectl get pod -n ceph-csi-operator-system
```

Expected output:

```text
NAME                                                    READY   STATUS    RESTARTS   AGE
ceph-csi-operator-controller-manager-744dc99cb5-scxxh   2/2     Running   0          45s
cephfs.csi.ceph.com-ctrlplugin-5847c998b5-xf85m         5/5     Running   0          27s
cephfs.csi.ceph.com-nodeplugin-r6pkt                    2/2     Running   0          27s
nfs.csi.ceph.com-ctrlplugin-76fd4f5b4c-smk2g            5/5     Running   0          27s
nfs.csi.ceph.com-nodeplugin-kbzms                       2/2     Running   0          27s
rbd.csi.ceph.com-ctrlplugin-6965dcfdb8-w88kn            5/5     Running   0          4m35s
rbd.csi.ceph.com-nodeplugin-lnm4n                       2/2     Running   0          4m35s
```

---

## 6. Create CephConnection

Create a CephConnection CR to connect to the Ceph cluster:

```console
echo '
apiVersion: csi.ceph.io/v1
kind: CephConnection
metadata:
  name: ceph-connection
  namespace: ceph-csi-operator-system
spec:
  monitors:
  - 10.98.44.171:6789
' | kubectl create -f -
```

Replace the monitor IP address with your Ceph cluster's monitor addresses.

---

## 7. Create ClientProfile

Create a ClientProfile CR to define the client configuration:

```console
echo '
apiVersion: csi.ceph.io/v1
kind: ClientProfile
metadata:
  name: storage
  namespace: ceph-csi-operator-system
spec:
  cephConnectionRef:
    name: ceph-connection
  cephFs:
    subVolumeGroup: csi
' | kubectl create -f -
```

> [!IMPORTANT]
> The ClientProfile name (`storage` in this example) will be used as the `clusterID` parameter in your StorageClass and VolumeSnapshotClass resources.

---

## 8. Create Ceph Secrets

Before you can provision storage, create Kubernetes Secrets containing Ceph credentials for CSI operations.

For detailed instructions on creating Ceph users and Kubernetes Secrets, refer to the upstream Ceph-CSI documentation:

- **Secret Examples**:
  - [RBD Secret Example](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/secret.yaml)
  - [CephFS Secret Example](https://github.com/ceph/ceph-csi/blob/devel/examples/cephfs/secret.yaml) (also used for NFS)
- **Ceph Capabilities**: [Required Ceph Capabilities](https://github.com/ceph/ceph-csi/blob/devel/docs/capabilities.md)

> [!NOTE]
> - Create secrets in the namespace where your applications will create PVCs
> - NFS volumes use the same CephFS secret format since NFS is built on CephFS

## 9. Create StorageClasses

Create StorageClasses using the upstream Ceph-CSI examples:

- [RBD StorageClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/storageclass.yaml)
- [CephFS StorageClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/cephfs/storageclass.yaml)
- [NFS StorageClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/nfs/storageclass.yaml)

> [!IMPORTANT]
> **ClusterID and ClientProfile Mapping**
>
> The `clusterID` parameter **must match** your ClientProfile CR name:
>
> ```yaml
> # In your StorageClass
> parameters:
>   clusterID: storage  # Must match the ClientProfile name from step 6
> ```
>
> This is the key difference from legacy Ceph-CSI deployments where `clusterID` was arbitrary.

## 10. Create VolumeSnapshotClasses (Optional)

For snapshot support, use the upstream Ceph-CSI VolumeSnapshotClass examples:

- [RBD VolumeSnapshotClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/snapshotclass.yaml)
- [CephFS VolumeSnapshotClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/cephfs/snapshotclass.yaml)
- [NFS VolumeSnapshotClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/nfs/snapshotclass.yaml)

Ensure the `clusterID` parameter matches your ClientProfile name:

```yaml
parameters:
  clusterID: storage  # Must match your ClientProfile name
```

## 11. Verify Storage Provisioning

Test your setup using the [Ceph-CSI PVC examples](https://github.com/ceph/ceph-csi/tree/devel/examples):

- [RBD PVC Example](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/pvc.yaml)
- [CephFS PVC Example](https://github.com/ceph/ceph-csi/blob/devel/examples/cephfs/pvc.yaml)
- [NFS PVC Example](https://github.com/ceph/ceph-csi/blob/devel/examples/nfs/pvc.yaml)

The PVC should reach `Bound` status, indicating successful provisioning.

---

## 12. Upgrade Ceph-CSI Operator and Drivers

To upgrade to a newer version:

### Step 1: Fetch and Checkout the Latest Tag

```bash
git fetch --tags
git tag -l                    # List available tags
git checkout v1.0.0           # Replace with desired version
```

### Step 2: Apply Updated Manifests

#### For Kubernetes (All-in-One)
```console
kubectl apply -f deploy/all-in-one/install.yaml
```

#### For OpenShift (All-in-One)
```console
kubectl apply -f deploy/all-in-one/install-openshift.yaml
```

#### For Multi-File Installation
```console
kubectl apply -f deploy/multifile/crd.yaml
kubectl apply -f deploy/multifile/operator.yaml -n ceph-csi-operator-system
```

### Step 3: Verify the Upgrade

```console
kubectl get pods -n ceph-csi-operator-system
```

Ensure all pods are running and using the upgraded version.

---

## 13. Clean Up Resources

### Step 1: Delete Custom Resources

```console
kubectl delete cephconnection ceph-connection -n ceph-csi-operator-system
kubectl delete clientprofile storage -n ceph-csi-operator-system
kubectl delete driver rbd.csi.ceph.com -n ceph-csi-operator-system
kubectl delete driver cephfs.csi.ceph.com -n ceph-csi-operator-system
kubectl delete driver nfs.csi.ceph.com -n ceph-csi-operator-system
```

### Step 2: Uninstall the Operator

#### For Kubernetes
```console
kubectl delete -f deploy/all-in-one/install.yaml
```

#### For OpenShift
```console
kubectl delete -f deploy/all-in-one/install-openshift.yaml
```

#### For Multi-File Installation
```console
kubectl delete -f deploy/multifile/operator.yaml -n ceph-csi-operator-system
kubectl delete -f deploy/multifile/csi-rbac.yaml -n ceph-csi-operator-system
kubectl delete -f deploy/multifile/openshift-scc.yaml  # OpenShift only
kubectl delete -f deploy/multifile/crd.yaml
```

### Step 3: Verify Deletion

```bash
kubectl get pods -n ceph-csi-operator-system
```

Expected output:

```text
No resources found in ceph-csi-operator-system namespace.
```
