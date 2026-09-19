# Monitoring Ceph-CSI controller metrics with PodMonitor

This document describes how to scrape Ceph-CSI controller plugin metrics
with the Prometheus Operator, using the example PodMonitor manifests in
`deploy/examples/podmonitor/`.

The operator does not create PodMonitor resources. Apply and maintain them
yourself, or deploy them through the Helm chart's arbitrary-manifest support
(`extraDeploy`). No operator API fields are required.

## What is collected

The upstream Kubernetes CSI sidecars running in the controller plugin
Deployment (`csi-provisioner`, `csi-attacher`, `csi-resizer`,
`csi-snapshotter`) can expose CSI operation metrics on an HTTP endpoint
(`--http-endpoint`, path `/metrics` by default).

These are controller operation metrics, not a replacement for the old
`csi_liveness` gauge. The `liveness-prometheus` sidecar is deprecated and
is not used here.

Node plugin monitoring is out of scope: the node driver registrar HTTP
endpoint serves only `/healthz`, not equivalent metrics.

## Prerequisites

- Prometheus Operator installed, so the `monitoring.coreos.com/v1`
  `PodMonitor` API is served.
- A `Prometheus` or `PrometheusAgent` object that selects the PodMonitor
  (commonly via `podMonitorSelector` matching the `monitoring: ceph-csi`
  label used in the examples).
- Metrics endpoints enabled on the sidecars via the Driver CR, for example:

```yaml
spec:
  controllerPlugin:
    containerExtraArgs:
      csi-provisioner: ["--http-endpoint=:8080"]
      csi-attacher: ["--http-endpoint=:8081"]
      csi-resizer: ["--http-endpoint=:8082"]
      csi-snapshotter: ["--http-endpoint=:8083"]
```

The ports in the PodMonitor must match these `--http-endpoint` values.
The examples use `targetPort` so no named container ports are required.

## Apply the examples

```bash
kubectl apply -f deploy/examples/podmonitor/rbd-controller-podmonitor.yaml
kubectl apply -f deploy/examples/podmonitor/cephfs-controller-podmonitor.yaml
```

Adjust `metadata.namespace` and `spec.selector.matchLabels.app`
(`<driver-name>-ctrlplugin`, e.g. `rbd.csi.ceph.com-ctrlplugin`) if your
Driver name or namespace differ.

## Helm (arbitrary manifests)

The `ceph-csi-drivers` chart renders anything listed in `extraDeploy`
through `tpl`, so the same PodMonitor can be shipped with the release
without operator API support:

```yaml
extraDeploy:
  - apiVersion: monitoring.coreos.com/v1
    kind: PodMonitor
    metadata:
      name: rbd-csi-controller
      namespace: ceph-csi-driver
      labels:
        monitoring: ceph-csi
    spec:
      selector:
        matchLabels:
          app: rbd.csi.ceph.com-ctrlplugin
      podMetricsEndpoints:
        - targetPort: 8080
          path: /metrics
          interval: 30s
```

See `deploy/examples/podmonitor/` for full manifests.

## Host network port conflicts

If the controller plugin runs with `hostNetwork: true`, the
`--http-endpoint` ports bind on the node itself. Two drivers using the
same port on the same node conflict: only one pod can bind the port.

Give each driver distinct ports (the RBD example uses 8080-8083, the
CephFS example uses 8090-8093). Do the same for NFS and NVMe-oF if you
monitor them.
