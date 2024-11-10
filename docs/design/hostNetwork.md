# Enabling host networking for controller plugin pods

By default, the Ceph-CSI controller plugins operate on the pod network
but under some circumstances, like setups with a dedicated storage network, 
where the pod network cannot connect to the ceph cluster,
it is necessary to run the Ceph-CSI controller plugin pods on the host network.

This document describes how the ceph-csi-operator can be configured to enforce the use of host network
in the Ceph-CSI controller plugin pods

The use of host networking can be enabled for a driver's controller plugin by setting `hostNetwork` to `true`in the `ControllerPlugin` section of the corresponding Driver CR.

The `hostNetwork` setting is also available in the `driverSpecDefaults.controllerPlugin` section
of the `OperatorConfig` CR. As this is a default for all Ceph-CSI controller plugins created by the operator, the setting
in concrete Driver CRs will take precedence.

There is currently no means of enforcing the use of host networking on all controller plugins against `Driver` CR settings.

Example:

## OperatorConfig CR

```yaml
kind: OperatorConfig 
apiVersion: csi.ceph.io/v1alpha1
metadata:
  name: ceph-csi-operator-config
  namespace: <operator-namespace>
spec:
   driverSpecDefaults:
     controllerPlugin:
       hostNetwork: true
```
## Driver CR

```yaml
apiVersion: csi.ceph.io/v1alpha1
kind: Driver
metadata:
  name: rbd.csi.ceph.com
  namespace: <operator-namespace> 
spec:
   controllerPlugin:
     hostNetwork: false
```
