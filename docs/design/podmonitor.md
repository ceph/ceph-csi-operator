# Ceph CSI PodMonitor Design Document

The Ceph-CSI driver pods expose Prometheus metrics on an HTTP endpoint. The
[liveness sidecar container](https://github.com/ceph/ceph-csi/blob/devel/docs/metrics.md)
collects the metrics from the CSI driver and serves them on the configured
`metricsPort` of every controller plugin and node plugin pod. To collect these
metrics, [Prometheus](https://prometheus.io/) needs to know how to discover and
scrape the endpoint of each pod.

When the [Prometheus Operator](https://prometheus-operator.dev/) is used to run
Prometheus, discovery is configured through the `PodMonitor` resource of the
`monitoring.coreos.com/v1` API. A `PodMonitor` describes a set of pods
(via a label selector) and the ports on those pods that serve metrics.

This feature automates the creation of such a `PodMonitor` resource: whenever a
`Driver` is reconciled, the operator creates and maintains a `PodMonitor` for
the driver's controller plugin and node plugin pods. There is no need anymore
for the administrator to hand-craft and maintain `PodMonitor` (or
`ServiceMonitor`) resources for Ceph-CSI.

## Enabling the PodMonitor

The feature is disabled by default, and can be enabled per driver through the
`spec.podMonitor` section of the `Driver` CRD. Scraping requires the liveness
sidecar, so `spec.liveness` must be configured as well:

```yaml
kind: Driver
apiVersion: csi.ceph.io/v1
metadata:
    name: "<prefix>.<driver_type>.csi.ceph.com"
    namespace: <operator-namespace>
spec:
    liveness:
        # Port on which the liveness sidecar serves the CSI metrics
        metricsPort: 9090
    podMonitor:
        # create and maintain a PodMonitor resource for this driver
        enabled: true
        # optional: labels to add to the PodMonitor, these can be used by a
        # Prometheus or PrometheusAgent resource to select this PodMonitor
        labels:
            monitoring: ceph-csi
        # optional: annotations to add to the PodMonitor
        annotations:
            owner: ceph-csi-operator
        # optional: Prometheus scrape interval, defaults to the global scrape
        # interval configured on the Prometheus resource
        interval: 30s
```

With the above configuration, the operator creates a `PodMonitor` resource in
the same namespace as the `Driver`:

```yaml
kind: PodMonitor
apiVersion: monitoring.coreos.com/v1
metadata:
    name: "<prefix>.<driver_type>.csi.ceph.com-podmonitor"
    namespace: <operator-namespace>
    labels:
        monitoring: ceph-csi
    ownerReferences:
        # owned by the Driver, garbage collected with it
        - kind: Driver
          name: "<prefix>.<driver_type>.csi.ceph.com"
          ...
spec:
    podMetricsEndpoints:
        - port: metrics
          path: /metrics
          interval: 30s
    selector:
        matchExpressions:
            - key: app
              operator: In
              values:
                  - "<prefix>.<driver_type>.csi.ceph.com-ctrlplugin"
                  - "<prefix>.<driver_type>.csi.ceph.com-nodeplugin"
```

The `PodMonitor` selects the controller plugin `Deployment` and the node plugin
`DaemonSet` pods of this driver only, by their `app` label. The `metrics` port
name refers to the container port that the operator declares on the
`liveness-prometheus` sidecar of both workloads (same port as configured with
`spec.liveness.metricsPort`).

Just like other resources created for the driver, the `PodMonitor` is owned by
the `Driver` resource and garbage collected together with it.

## Defaults for all drivers

Drivers that do not configure `spec.podMonitor` themselves inherit the value
from `spec.driverSpecDefaults.podMonitor` of the `OperatorConfig` resource:

```yaml
kind: OperatorConfig
apiVersion: csi.ceph.io/v1
metadata:
    name: ceph-csi-operator-config
    namespace: <operator-namespace>
spec:
    driverSpecDefaults:
        liveness:
            metricsPort: 9090
        podMonitor:
            enabled: true
```

## Disabling the PodMonitor

Removing `spec.podMonitor` from the `Driver` (or setting
`spec.podMonitor.enabled` to `false`, or removing `spec.liveness`) makes the
operator delete the `PodMonitor` resource during the next reconciliation of the
driver.

When the Prometheus Operator is not installed in the cluster, no
`monitoring.coreos.com/v1` API is served, and the operator simply skips the
`PodMonitor` cleanup; reconciling the driver does not fail in that case.

## Prerequisites

- The [Prometheus Operator](https://prometheus-operator.dev/docs/getting-started/introduction/)
  must be running in the cluster, so that the `monitoring.coreos.com/v1` API
  (with the `PodMonitor` kind) is served, and a `Prometheus` object exists that
  selects the created `PodMonitor` (commonly via
  `podMonitorSelector: {}` or a matching label).
- `spec.liveness.metricsPort` must be set on the driver, as the liveness
  sidecar serves the CSI metrics endpoint.

## Considerations for hostNetwork pods

The node plugin pods run with `hostNetwork: true`. Prometheus discovers these
targets through their pod IP, which equals the IP of the Kubernetes node. The
`metricsPort` is bound on the node itself, and two drivers with the same
`spec.liveness.metricsPort` running on the same node conflict with each other:
only one of the pods can bind the port, and its metrics are scraped while the
other pod fails to start its metrics endpoint.

When more than one Ceph-CSI driver is deployed in the cluster (for example an
RBD and a CephFS driver), configure a distinct `spec.liveness.metricsPort` for
each driver type. The default manifests of Ceph-CSI historically use different
ports per driver type for the same reason (e.g. 8080 for RBD, 8081 for CephFS).

## Helm charts

Both the `ceph-csi-drivers` chart and the `ceph-csi-operator` chart
(`operatorConfig.driverSpecDefaults`) expose the `liveness` and `podMonitor`
settings, see the [drivers chart values](../helm-charts/drivers-chart.md) for
the available options:

```yaml
drivers:
  rbd:
    liveness:
      metricsPort: 8080
    podMonitor:
      enabled: true
      labels:
        monitoring: ceph-csi
      interval: 30s
```
