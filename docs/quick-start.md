- [Quick Start Guide for Ceph-CSI-Operator](#quick-start-guide-for-ceph-csi-operator)
  - [1. Prerequisites](#1-prerequisites)
  - [2. Install the Ceph-CSI Operator](#2-install-the-ceph-csi-operator)
    - [3. Deploy Ceph-CSI Drivers](#3-deploy-ceph-csi-drivers)
      - [3.1 Deploy the RBD Driver](#31-deploy-the-rbd-driver)
      - [3.2 Deploy the CephFS Driver](#32-deploy-the-cephfs-driver)
    - [3.3 Deploy the Ceph-NFS Driver](#33-deploy-the-ceph-nfs-driver)
  - [4. Verify Installation](#4-verify-installation)
  - [5. Create CephConnection](#5-create-cephconnection)
  - [6. Create ClientProfile](#6-create-clientprofile)
  - [7. Clean Up Resources](#7-clean-up-resources)

# Quick Start Guide for Ceph-CSI-Operator

## 1. Prerequisites

Before deploying the Ceph-CSI-Operator, ensure the following requirements are met:

- A Kubernetes cluster ([supported version](https://kubernetes.io/releases/) recommended)
- Ceph cluster ([supported version](https://docs.ceph.com/en/latest/releases/) recommended)
- `kubectl` CLI installed

**Note:** In this guide, we will use minimal configurations to deploy the Ceph-CSI-Operator and drivers. You can customize the configurations as per your requirements.

## 2. Install the Ceph-CSI Operator

```console
kubectl create -f deploy/all-in-one/install.yaml
```

verify the installation:

```bash
kubectl get pods -n ceph-csi-operator-system
NAME                                                    READY   STATUS    RESTARTS   AGE
ceph-csi-operator-controller-manager-67d45fd9ff-zgst7   2/2     Running   0          40s
```

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

### 3.3 Deploy the Ceph-NFS Driver

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

Use the ClientProfile Name as the ClusterID in the required classes (StrorageClass,VolumeSnapshotClass etc).


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
