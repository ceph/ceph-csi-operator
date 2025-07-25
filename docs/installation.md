# Ceph-CSI Installation Methods

- [Ceph-CSI Installation Methods](#ceph-csi-installation-methods)
  - [2. Choose Your Installation Method](#2-choose-your-installation-method)
    - [⚠️ Method 1: Helm Chart Installation (Experimental)](#️-method-1-helm-chart-installation-experimental)
    - [Method 2: Kubernetes YAML Installation](#method-2-kubernetes-yaml-installation)
  - [3. Summary](#3-summary)


This document provides an overview of the two main installation methods for **Ceph-CSI**:

- **Helm Chart Installation**: A simpler, more automated way to install Ceph-CSI using Helm charts.
- **Kubernetes YAML Installation**: A manual method for installing Ceph-CSI using Kubernetes YAML files.

Both methods achieve the same goal deploying Ceph-CSI drivers and operators in your Kubernetes cluster but they offer different levels of control and automation.

## 2. Choose Your Installation Method

### ⚠️ Method 1: Helm Chart Installation (Experimental)

> **🚧 Experimental Feature**
> The Helm-based installation of the Ceph-CSI Operator and drivers is currently **experimental**.
> It simplifies deployment and supports automation, but may not support all production use cases yet.
> Use it with caution and test thoroughly before adopting in critical environments.

Using Helm for installation allows you to quickly deploy the Ceph-CSI Operator and drivers with minimal configuration. This method is ideal if you want to automate the deployment and benefit from Helm's management features.

**Steps for Helm Installation:**

1. Follow the instructions in [operator](./helm-charts/operator-chart.md) and [drivers](./helm-charts/drivers-chart.md).
2. The Helm chart will install the Ceph-CSI Operator, CRDs, RBAC, and drivers.

### Method 2: Kubernetes YAML Installation

The Kubernetes YAML method involves manually applying YAML files to your cluster. This gives you more control over the configuration and is useful if you prefer a more hands-on approach.

**Steps for YAML Installation:**

1. Follow the instructions in [YAML Installation Guide](./kubernetes-installation.md).
2. Apply the YAML files for CRDs, RBAC, and drivers to install the Ceph-CSI Operator and drivers.

## 3. Summary

- **Use Helm Chart Installation** if you prefer a streamlined, automated process for installation and updates.
- **Use Kubernetes YAML Installation** if you want greater control over the configuration or if you are working in environments where Helm cannot be used.

For more detailed steps, please refer to the respective guides for Helm or Kubernetes YAML installation.
