---
title: Ceph-CSI Driver Helm Chart
---
{{ template "generatedDocsWarning" . }}

Creates ceph-csi-operator resources to configure a [ceph-csi](https://github.com/ceph/ceph-csi) drivers using the [Helm](https://helm.sh) package manager.
This chart is a simple packaging of templates that will optionally create ceph-csi-operator resources such as:

* Driver CRs (RBD,cephFS,NFS)
* CephConnection that contains the ceph details
* ClientProfile for the RBD/CephFS/NFS clusterID and corresponding configurations
* ClientProfileMapping for disaster recovery


## Prerequisites

* Kubernetes 1.32+
* Helm 3.x

See the [Helm support matrix](https://helm.sh/docs/topics/version_skew/) for more details.

## Installing

The Ceph-CSI Drivers helm chart will install the basic components necessary to install [ceph-csi](https://github.com/ceph/ceph-csi) on Kubernetes cluster.

1. Install the Helm chart

The `helm install` command deploys ceph-csi-drivers on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

ceph-csi-drivers currently publishes artifacts of the ceph-csi drivers to tagged versions.

### **Released version**


```console
helm repo add ceph-csi-operator https://ceph.github.io/ceph-csi-operator-charts
helm install ceph-csi-drivers --create-namespace --namespace ceph-csi-driver ceph-csi-operator/ceph-csi-drivers
```

For example settings, see the next section or [values.yaml](https://github.com/ceph/ceph-csi-operator/tree/main/deploy/charts/ceph-csi-drivers/values.yaml)

## Configuration

The following table lists the configurable parameters of the ceph-csi-drivers chart and their default values.

{{ template "chart.valuesTable" . }}

### **Development Build**

To deploy from a local build from your development environment:

1. Install the helm chart:

```console
cd deploy/charts/ceph-csi-drivers
helm install ceph-csi-drivers --create-namespace --namespace ceph-csi-driver .
```

## Uninstalling the Chart

To see the currently installed ceph-csi-drivers chart:

```console
helm ls --namespace ceph-csi-driver
```

To uninstall/delete the `ceph-csi-drivers` deployment:

```console
helm delete --namespace ceph-csi-driver ceph-csi-drivers
```

The command removes all the Kubernetes components associated with the chart and deletes the release.
