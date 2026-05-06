---
title: Ceph-CSI Operator Helm Chart
---
{{ template "generatedDocsWarning" . }}

Installs [ceph-csi-operator](https://github.com/ceph/ceph-csi-operator) to automates the deployment, configuration, and management of [ceph-csi](https://github.com/ceph/ceph-csi) drivers using new Kubernetes APIs defined as a set of Custom Resource Definitions (CRDs).

## Introduction

This chart bootstraps a [ceph-csi-operator](https://github.com/ceph/ceph-csi-operator) deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Prerequisites

* Kubernetes 1.32+
* Helm 3.x

See the [Helm support matrix](https://helm.sh/docs/topics/version_skew/) for more details.

## Installing

The Ceph-CSI Operator helm chart will install the basic components necessary to install [ceph-csi](https://github.com/ceph/ceph-csi) on Kubernetes cluster.

1. Install the Helm chart

The `helm install` command deploys ceph-csi-operator on the Kubernetes cluster in the default configuration. The [configuration](#configuration) section lists the parameters that can be configured during installation.

ceph-csi-operator currently publishes builds of the ceph-csi operator to tagged versions.

### **Released version**


```console
helm repo add ceph-csi-operator https://ceph.github.io/ceph-csi-operator/
helm install ceph-csi-operator --create-namespace --namespace ceph-csi-operator-system ceph-csi-operator/ceph-csi-operator
```

For example settings, see the next section or [values.yaml](https://github.com/ceph/ceph-csi-operator/tree/main/deploy/charts/ceph-csi-operator/values.yaml)

### **OpenShift Installation**

For OpenShift clusters, enable the OpenShift-specific SecurityContextConstraints (SCC) by setting `openshift.enabled=true`:

```console
helm repo add ceph-csi-operator https://ceph.github.io/ceph-csi-operator/
helm install ceph-csi-operator --create-namespace --namespace ceph-csi-operator-system \
  --set openshift.enabled=true \
  ceph-csi-operator/ceph-csi-operator
```

This will create:
* A SecurityContextConstraint (`ceph-csi-operator-scc`) that grants the necessary permissions for CSI operations
* A ClusterRole (`ceph-csi-operator-scc-user`) that allows using the SCC
* ClusterRoleBindings that bind all CSI service accounts to the SCC ClusterRole

**Note:** When deploying drivers on OpenShift, you must also enable OpenShift support in the drivers chart. See the [drivers chart documentation](./drivers-chart.md#openshift-installation) for details.

## Configuration

The following table lists the configurable parameters of the ceph-csi-operator chart and their default values.

{{ template "chart.valuesTable" . }}

### **Development Build**

To deploy from a local build from your development environment:

1. Build the cephcsi-operator container image: `make docker-build`
1. Copy the image to your K8s cluster, such as with the `docker save` then the `docker load` commands
1. Install the helm chart:

```console
cd deploy/charts/ceph-csi-operator
helm install --create-namespace --namespace ceph-csi-operator-system ceph-csi-operator .
```

## Uninstalling the Chart

To see the currently installed ceph-csi-operator chart:

```console
helm ls --namespace ceph-csi-operator-system
```

To uninstall/delete the `ceph-csi-operator` deployment:

```console
helm delete --namespace ceph-csi-operator-system ceph-csi-operator
```

The command removes all the Kubernetes components associated with the chart and deletes the release.
