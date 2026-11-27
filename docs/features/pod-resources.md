# Pod-level resources

The driver CRs expose per-container resource requests and limits under
`nodePlugin.resources` and `controllerPlugin.resources`. Kubernetes also
supports setting requests and limits for a pod as a whole. The operator passes
that configuration through with the `podResources` field.

Pod-level resources are useful for the controller plugin, which runs the
plugin together with several sidecars (provisioner, attacher, resizer,
snapshotter, and optionally more). Setting a single pod-level request avoids
reserving a separate request for every sidecar while still bounding the total
usage of the pod.

## Requirements

Pod-level resources require the Kubernetes `PodLevelResources` feature gate:

- Kubernetes 1.32 and 1.33: alpha, disabled by default.
- Kubernetes 1.34 and later: beta, enabled by default.

Only CPU, memory, and hugepages resources can be set at pod level. Pod-level
resources are not supported for Windows pods.

## Configuration

Set `podResources` next to the existing container resources in the `Driver`
CR:

```yaml
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: rbd.csi.ceph.com
  namespace: ceph-csi-operator-system
spec:
  controllerPlugin:
    # Applies to the controller plugin Deployment as a whole.
    podResources:
      requests:
        cpu: 500m
        memory: 512Mi
      limits:
        cpu: "2"
        memory: 1Gi
  nodePlugin:
    # Applies to the node plugin and CSI-Addons DaemonSets.
    podResources:
      requests:
        cpu: 250m
        memory: 256Mi
      limits:
        cpu: "1"
        memory: 512Mi
```

Container resources can still be set. Both configurations may be used at the
same time, and Kubernetes gives the pod-level values precedence when it
calculates the pod's effective resources and QoS class:

```yaml
spec:
  controllerPlugin:
    podResources:
      requests:
        cpu: 500m
        memory: 512Mi
    resources:
      provisioner:
        requests:
          cpu: 100m
          memory: 128Mi
```

The operator does not aggregate or rewrite container resources. If you set
both, the following Kubernetes constraints apply. The pod is rejected if they
are violated:

- The pod-level request for a resource must be greater than or equal to the
  sum of the container requests for that resource.
- Each container limit must be less than or equal to the pod-level limit.
- For hugepages, the pod-level limit must be greater than or equal to the sum
  of the container hugepage limits.
- `claims` cannot be set at pod level. The CRD rejects it.

To reduce the resources reserved for a pod, set `podResources` and either omit
the per-container requests or keep their sum at or below the pod-level
request.

## OperatorConfig defaults

The `podResources` field can also be set for every driver through
`OperatorConfig.spec.driverSpecDefaults`:

```yaml
apiVersion: csi.ceph.io/v1
kind: OperatorConfig
metadata:
  name: ceph-csi-operator-config
  namespace: ceph-csi-operator-system
spec:
  driverSpecDefaults:
    controllerPlugin:
      podResources:
        requests:
          cpu: 500m
          memory: 512Mi
    nodePlugin:
      podResources:
        requests:
          cpu: 250m
          memory: 256Mi
```

A value set on a `Driver` takes precedence over the `OperatorConfig` default.

## Helm chart

With the `ceph-csi-drivers` chart, set the values for all drivers or for an
individual driver:

```yaml
operatorConfig:
  driverSpecDefaults:
    controllerPlugin:
      podResources:
        requests:
          cpu: 500m
          memory: 512Mi

drivers:
  rbd:
    controllerPlugin:
      podResources:
        requests:
          cpu: 500m
          memory: 512Mi
    nodePlugin:
      podResources:
        requests:
          cpu: 250m
          memory: 256Mi
```

The values render only when set. The default is an empty object, which leaves
the generated pod spec unchanged.
