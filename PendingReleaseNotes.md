# v1.1.0 Pending Release Notes

## Breaking Changes

## Features

- Added NetworkPolicies for the operator pod and CSI driver pods (controller-plugin, csi-addons nodeplugin). Included in all generated manifests by default. Driver pod NPs are created by the operator for every reconciled driver. Node-plugin pods are exempt (`hostNetwork: true`).
- Added `ClientProfileReplication` CR to enable replication destination mapping for disaster recovery scenarios. This allows the operator to configure destination cluster and pool mapping information in the ceph-csi-config ConfigMap's `replicationDestination` field. The ClientProfileReplication controller validates CRs and ensures only one Ready CR exists per ClientProfile (oldest wins). The ClientProfile controller consumes Ready ClientProfileReplication CRs to populate the replication destination mapping, which ceph-csi uses for the `GetReplicationDestinationInfo` RPC to discover correct destination volume IDs when pools have different IDs across mirrored clusters. Supports both `ClientProfileMapping` and `ClientProfileReplication` during migration, with deletion protection preventing removal of ClientProfile CRs that have referencing ClientProfileReplication CRs.
- The operator now runs the controller plugin containers privileged whenever driver log rotation is enabled (which is the default in the Helm chart), so that writing the rotated log files to the hostPath logs directory (default `/var/lib/cephcsi`) works on hosts with SELinux in enforcing mode. This matches the node plugin containers, which already run privileged for the same reason. Consequently, the `controllerPlugin.privileged` field of the Driver CR is deprecated and its value is ignored; disabling log rotation (`log.rotation`) restores fully unprivileged controller plugin pods.
## NOTE
