# v1.1.0 Pending Release Notes

## Breaking Changes

## Features

- Added NetworkPolicies for the operator pod and CSI driver pods (controller-plugin, csi-addons nodeplugin). Included in all generated manifests by default. Driver pod NPs are created by the operator for every reconciled driver. Node-plugin pods are exempt (`hostNetwork: true`).
- Added `ClientProfileReplication` CR to enable replication destination mapping for disaster recovery scenarios. This allows the operator to configure destination cluster and pool mapping information in the ceph-csi-config ConfigMap's `replicationDestination` field. The ClientProfileReplication controller validates CRs and ensures only one Ready CR exists per ClientProfile (oldest wins). The ClientProfile controller consumes Ready ClientProfileReplication CRs to populate the replication destination mapping, which ceph-csi uses for the `GetReplicationDestinationInfo` RPC to discover correct destination volume IDs when pools have different IDs across mirrored clusters. Supports both `ClientProfileMapping` and `ClientProfileReplication` during migration, with deletion protection preventing removal of ClientProfile CRs that have referencing ClientProfileReplication CRs.
- Added PodMonitor support to automate Prometheus monitoring of the CSI metrics. When `spec.podMonitor.enabled` is set to `true` on a Driver CR (or via `spec.driverSpecDefaults.podMonitor` in the OperatorConfig CR), the operator creates and maintains a `monitoring.coreos.com/v1` PodMonitor resource that lets the Prometheus Operator discover and scrape the CSI metrics served by the driver's `liveness-prometheus` sidecar (requires `spec.liveness.metricsPort` to be configured and the Prometheus Operator to be installed). The feature is disabled by default; see `docs/design/podmonitor.md` for details, including how to avoid `metricsPort` conflicts between node plugin pods that run with `hostNetwork: true`.
## NOTE
