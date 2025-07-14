---
title: Helm Charts Overview
---

Ceph-csi-operator has published the following Helm charts for the ceph-csi-operators:

* [ceph-csi Operator](operator-chart.md): Starts the ceph-csi Operator, which will watch for Ceph CRs (custom resources)
* [ceph-csi Drivers](drivers-chart.md): Creates `Drivers`,`Cephconnection`,`ClientProfile` and `ClientProfileMapping` CRs that the operator will use to configure the ceph-csi drivers

The Helm charts are intended to simplify deployment and upgrades.
