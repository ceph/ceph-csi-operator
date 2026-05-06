# Ceph-CSI-Operator Release Installation Guide

---
- [Ceph-CSI-Operator Release Installation Guide](#ceph-csi-operator-release-installation-guide)
  - [1. Prerequisites](#1-prerequisites)
  - [2. Install the Ceph-CSI Operator](#2-install-the-ceph-csi-operator)
    - [2.1 All-in-One Installation](#21-all-in-one-installation)
      - [Step 1: Install from the Released Version](#step-1-install-from-the-released-version)
      - [Step 2: Verify Installation](#step-2-verify-installation)
    - [2.2 Multi-File Installation](#22-multi-file-installation)
      - [Step 1: Install CRDs](#step-1-install-crds)
      - [Step 2: Create RBAC Resources in the Desired Namespace](#step-2-create-rbac-resources-in-the-desired-namespace)
      - [Step 3: Install the Ceph-CSI Operator](#step-3-install-the-ceph-csi-operator)
      - [Step 4: Verify the Installation](#step-4-verify-the-installation)
    - [3. Deploy Ceph-CSI Drivers](#3-deploy-ceph-csi-drivers)
      - [3.1 Deploy the RBD Driver](#31-deploy-the-rbd-driver)
      - [3.2 Deploy the CephFS Driver](#32-deploy-the-cephfs-driver)
      - [3.3 Deploy the Ceph-NFS Driver](#33-deploy-the-ceph-nfs-driver)
  - [4. Verify Installation](#4-verify-installation)
  - [5. Create CephConnection](#5-create-cephconnection)
  - [6. Create ClientProfile](#6-create-clientprofile)
  - [7. Create Ceph Secrets](#7-create-ceph-secrets)
  - [8. Create StorageClasses](#8-create-storageclasses)
  - [9. Create VolumeSnapshotClasses (Optional)](#9-create-volumesnapshotclasses-optional)
  - [10. Verify Storage Provisioning](#10-verify-storage-provisioning)
  - [11. Upgrade Ceph-CSI Operator and Drivers](#11-upgrade-ceph-csi-operator-and-drivers)
    - [Step 1: Checkout the Latest Tag](#step-1-checkout-the-latest-tag)
      - [Step 2: Apply the updated yaml's](#step-2-apply-the-updated-yamls)
      - [Step 3: Verify the Upgrade](#step-3-verify-the-upgrade)
  - [7. Clean Up Resources](#7-clean-up-resources)

## 1. Prerequisites

Before proceeding with the installation of the Ceph-CSI Operator, ensure the following requirements are met:

- **Kubernetes Cluster:** A running Kubernetes cluster with a supported version ([Kubernetes Release Versions](https://kubernetes.io/releases/)).
- **Ceph Cluster:** A Ceph cluster with a supported version ([Ceph Releases](https://docs.ceph.com/en/latest/releases/)).
- **kubectl CLI:** The `kubectl` command-line tool must be installed and configured to interact with your Kubernetes cluster.

**Note:** This guide assumes minimal configuration to deploy the Ceph-CSI Operator and drivers. You may customize the configurations as per your environment and requirements.



## 2. Install the Ceph-CSI Operator

Ceph-CSI Operator installation can be done using two methods: **All-in-One Installation** or **Multi-File Installation**. Choose the method that best suits your requirements.

Step 1: Clone the Ceph-CSI Repository and Checkout the Release Tag

For both installation methods (All-in-One and Multi-File), it's necessary to checkout the desired release tag from the Ceph-CSI repository.

```concole
git clone https://github.com/ceph/ceph-csi-operator.git
cd ceph-csi-operator
git checkout v0.3.1
```

**Note:** checkout the latest tag, refer to [Releases](https://github.com/ceph/ceph-csi-operator/releases) for latest.

### 2.1 All-in-One Installation

The **All-in-One Installation** method allows for a quick and easy deployment of the Ceph-CSI Operator and all its components (CRDs, RBAC, operator) in a single step.

#### Step 1: Install from the Released Version

To install the Ceph-CSI Operator, use the following command:

```console
kubectl create -f deploy/all-in-one/install.yaml
```

This YAML file will install:

- The Ceph-CSI Operator
- Custom Resource Definitions (CRDs)
- RBAC resources (Role-Based Access Control)

#### Step 2: Verify Installation


```bash
kubectl get pods -n ceph-csi-operator-system
NAME                                                    READY   STATUS    RESTARTS   AGE
ceph-csi-operator-controller-manager-67d45fd9ff-zgst7   2/2     Running   0          40s
```


### 2.2 Multi-File Installation

The **Multi-File Installation** method is more flexible and allows you to deploy each component (CRDs, RBAC, Operator) separately, providing finer control over the installation process.

Install CRDs, Operator, and RBAC Resources in your specific namespace

The RBAC (Role-Based Access Control) resources should be applied in the same namespace where you plan to install the Ceph-CSI drivers.

#### Step 1: Install CRDs

First, create the CRDs for the Ceph-CSI components:

```concole
kubectl create -f deploy/multifile/crd.yaml
```

This will create the necessary Custom Resource Definitions (CRDs) like CephConnection, ClientProfile, ClientProfileMapping and Driver.

#### Step 2: Create RBAC Resources in the Desired Namespace

You need to create the RBAC resources in the namespace where you plan to install the Ceph-CSI drivers. For example, if you're using the ceph-csi-operator-system namespace:

kubectl create -f deploy/multifile/csi-rbac.yaml -n ceph-csi-operator-system

This will ensure the correct service accounts, roles, and role bindings are created in the target namespace.

#### Step 3: Install the Ceph-CSI Operator

After applying the CRDs and RBAC resources, install the Ceph-CSI Operator itself. Make sure you have the correct namespace set for the installation.

```concole
kubectl create -f deploy/multifile/operator.yaml -n ceph-csi-operator-system
```

#### Step 4: Verify the Installation

After installing the operator, verify the pods in your specified namespace:

```bash
kubectl get pods -n ceph-csi-operator-system
NAME                                                    READY   STATUS    RESTARTS   AGE
ceph-csi-operator-controller-manager-67d45fd9ff-zgst7   2/2     Running   0          40s
```

You should see the Ceph-CSI operator controller running.

### 3. Deploy Ceph-CSI Drivers

Once the operator is installed, deploy the Ceph-CSI drivers:

#### 3.1 Deploy the RBD Driver

```console
echo '
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: rbd.csi.ceph.com
  namespace: ceph-csi-operator-system
' | kubectl create -f -
```

#### 3.2 Deploy the CephFS Driver

```console
echo '
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: cephfs.csi.ceph.com
  namespace: ceph-csi-operator-system
' | kubectl create -f -
```

#### 3.3 Deploy the Ceph-NFS Driver

```console
echo '
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: nfs.csi.ceph.com
  namespace: ceph-csi-operator-system
' | kubectl create -f -
```

## 4. Verify Installation

To verify the installation, check the status of the Ceph-CSI components:

```bash
kubectl get pod -nceph-csi-operator-system
NAME                                                    READY   STATUS    RESTARTS   AGE
ceph-csi-operator-controller-manager-744dc99cb5-scxxh   2/2     Running   0          45s
cephfs.csi.ceph.com-ctrlplugin-5847c998b5-xf85m         5/5     Running   0          27s
cephfs.csi.ceph.com-nodeplugin-r6pkt                    2/2     Running   0          27s
nfs.csi.ceph.com-ctrlplugin-76fd4f5b4c-smk2g            5/5     Running   0          27s
nfs.csi.ceph.com-nodeplugin-kbzms                       2/2     Running   0          27s
rbd.csi.ceph.com-ctrlplugin-6965dcfdb8-w88kn            5/5     Running   0          4m35s
rbd.csi.ceph.com-nodeplugin-lnm4n                       2/2     Running   0          4m35s
```

## 5. Create CephConnection

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

## 6. Create ClientProfile

Create a ClientProfile CR to define the client configuration which points to
the CephConnection CR and the CephFS and RBD configurations:

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

## 7. Create Ceph Secrets

Before you can provision storage, create Kubernetes Secrets containing Ceph credentials for CSI operations.

For detailed instructions on creating Ceph users and Kubernetes Secrets, refer to the upstream Ceph-CSI documentation:

- **Secret Examples**:
  - [RBD Secret Example](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/secret.yaml)
  - [CephFS Secret Example](https://github.com/ceph/ceph-csi/blob/devel/examples/cephfs/secret.yaml) (also used for NFS)
- **Ceph Capabilities**: [Required Ceph Capabilities](https://github.com/ceph/ceph-csi/blob/devel/docs/capabilities.md)

> [!NOTE]
> - Create secrets in the namespace where your applications will create PVCs
> - NFS volumes use the same CephFS secret format since NFS is built on CephFS

## 8. Create StorageClasses

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

## 9. Create VolumeSnapshotClasses (Optional)

For snapshot support, use the upstream Ceph-CSI VolumeSnapshotClass examples:

- [RBD VolumeSnapshotClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/snapshotclass.yaml)
- [CephFS VolumeSnapshotClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/cephfs/snapshotclass.yaml)
- [NFS VolumeSnapshotClass Example](https://github.com/ceph/ceph-csi/blob/devel/examples/nfs/snapshotclass.yaml)

Ensure the `clusterID` parameter matches your ClientProfile name:

```yaml
parameters:
  clusterID: storage  # Must match your ClientProfile name
```

## 10. Verify Storage Provisioning

Test your setup using the [Ceph-CSI PVC examples](https://github.com/ceph/ceph-csi/tree/devel/examples):

- [RBD PVC Example](https://github.com/ceph/ceph-csi/blob/devel/examples/rbd/pvc.yaml)
- [CephFS PVC Example](https://github.com/ceph/ceph-csi/blob/devel/examples/cephfs/pvc.yaml)
- [NFS PVC Example](https://github.com/ceph/ceph-csi/blob/devel/examples/nfs/pvc.yaml)

The PVC should reach `Bound` status, indicating successful provisioning.

## 11. Upgrade Ceph-CSI Operator and Drivers

Upgrading your Ceph-CSI installation involves updating the Ceph-CSI Operator and drivers to a newer version. Follow the steps below to perform the upgrade:

### Step 1: Checkout the Latest Tag

First, make sure you have the latest version of the Ceph-CSI repository by checking out the latest release tag:

1. **Fetch all the tags from the remote repository:**

   ```bash
   git fetch --tags

2. **Checkout the latest tag:**

For example, to checkout the latest release tag (v1.0.0):

```concole
git checkout v1.0.0
```

You can list all available tags with:

```concle
git tag -l
```

Use the `git describe --tags` command to verify the current tag you are on.

#### Step 2: Apply the updated yaml's

based on the installation steps above you can use similar steps and apply the yaml files from the newly checked out branch.

#### Step 3: Verify the Upgrade

After the upgrade is complete, verify that all pods are running the latest version of the Ceph-CSI components:

```concole
kubectl get pods -n ceph-csi-operator-system
```

Ensure that the image/CRD versions match the upgraded version.

## 7. Clean Up Resources

To clean up the resources, delete the cepconnection, clientprofile and drivers:

```console
kubectl delete cephconnection ceph-connection -n ceph-csi-operator-system
kubectl delete clientprofile storage -n ceph-csi-operator-system
kubectl delete driver rbd.csi.ceph.com -n ceph-csi-operator-system
kubectl delete driver cephfs.csi.ceph.com -n ceph-csi-operator-system
kubectl delete driver nfs.csi.ceph.com -n ceph-csi-operator-system
```

To uninstall the Ceph-CSI-Operator, delete the operator:

```console
kubectl delete -f deploy/all-in-one/install.yaml
```

verify the deletion:

```bash
kubectl get pods -n ceph-csi-operator-system
No resources found in ceph-csi-operator-system namespace.
```
