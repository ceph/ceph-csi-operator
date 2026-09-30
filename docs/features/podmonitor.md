# Monitoring Ceph-CSI metrics with Prometheus Operator

The CSI sidecar containers (`csi-provisioner`, `csi-attacher`, `csi-resizer`
and `csi-snapshotter`) deployed with the Ceph-CSI controller plugin pods can
expose Prometheus metrics through an HTTP endpoint. A `PodMonitor` resource
tells the [Prometheus Operator](https://prometheus-operator.dev/) how to
discover and scrape those metrics.

> **Note:** the `liveness-prometheus` sidecar is deprecated and is not used
> by the approach documented here. Node plugin pods expose no CSI metrics
> endpoints, so node plugin monitoring is out of scope.

## Prerequisites

- The Prometheus Operator is installed in the cluster, so that the
  `monitoring.coreos.com/v1` API (with the `PodMonitor` kind) is served.
- A `Prometheus` (or `PrometheusAgent`) resource that selects the
  `PodMonitor` objects, e.g.:

  ```yaml
  spec:
    podMonitorSelector:
      matchLabels:
        monitoring: ceph-csi
  ```

  Adjust the `PodMonitor` labels in the example to match the selector of
  your Prometheus deployment.

## Admin Responsibilities

1. Enable the metrics endpoint of the CSI sidecars on the `Driver` CR,
   through `controllerPlugin.containerExtraArgs` (one port per sidecar;
   the `--http-endpoint` argument must carry a value, otherwise no metrics
   server is started):

   ```yaml
   spec:
     controllerPlugin:
       containerExtraArgs:
         csi-provisioner: ["--http-endpoint=:8080"]
         csi-attacher: ["--http-endpoint=:8081"]
         csi-resizer: ["--http-endpoint=:8082"]
         csi-snapshotter: ["--http-endpoint=:8083"]
   ```

2. Apply the example `PodMonitor`:

   ```console
   kubectl apply -f deploy/examples/podmonitor/controller-podmonitor.yaml
   ```

   The [example](https://github.com/ceph/ceph-csi-operator/blob/main/deploy/examples/podmonitor/controller-podmonitor.yaml)
   scrapes the controller plugin pods of the `rbd.csi.ceph.com` driver in the
   `ceph-csi-driver` namespace. Adjust `metadata.namespace`, the `app` label
   in `spec.selector` and the port numbers to your deployment.

3. Deploy as many `PodMonitor` resources as you have drivers. The manifest is
   the same for every driver, so create one more copy of the example for each
   additional driver and adjust only these driver-specific fields:

   | Field | RBD (the example) | CephFS | NFS | NVMe-oF |
   | --- | --- | --- | --- | --- |
   | `metadata.name` | `rbd-controller-podmonitor` | `cephfs-controller-podmonitor` | `nfs-controller-podmonitor` | `nvmeof-controller-podmonitor` |
   | `spec.selector.matchLabels.app` | `rbd.csi.ceph.com-ctrlplugin` | `cephfs.csi.ceph.com-ctrlplugin` | `nfs.csi.ceph.com-ctrlplugin` | `nvmeof.csi.ceph.com-ctrlplugin` |
   | `podMetricsEndpoints[].targetPort` | `8080`-`8083` | `8090`-`8093` | `8100`-`8103` | `8110`-`8113` |

   Each driver needs its own set of ports: with `hostNetwork: true`, the
   ports bind on the node itself, so two controller plugin pods scheduled on
   the same node cannot bind the same port. Enable the matching
   `--http-endpoint` ports in that driver's `containerExtraArgs`.

4. Alternatively, ship the `PodMonitor` resources with your Helm release
   through the `ceph-csi-drivers` chart `extraDeploy` value, which renders
   arbitrary resources through `tpl`:

   ```yaml
   extraDeploy:
     - apiVersion: monitoring.coreos.com/v1
       kind: PodMonitor
       metadata:
         name: rbd-controller-podmonitor
         namespace: {{ .Release.Namespace }}
         labels:
           monitoring: ceph-csi
       spec:
         selector:
           matchLabels:
             app: rbd.csi.ceph.com-ctrlplugin
         namespaceSelector:
           matchNames:
             - {{ .Release.Namespace }}
         podMetricsEndpoints:
           - targetPort: 8080
             path: /metrics
             interval: 30s
   ```

## Ceph-CSI Operator Responsibilities

The operator does not create or manage `PodMonitor` resources. It renders the
`controllerPlugin.containerExtraArgs` configured on the `Driver` CR into the
sidecar container arguments, which is what makes the metrics endpoints (the
targets of the `PodMonitor` above) available.
